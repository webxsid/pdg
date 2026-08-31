package inject

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/reconcile"
	"github.com/webxsid/pdg/internal/state"
	"golang.org/x/net/html"
)

type Result struct {
	Path   string
	Action string
}

type Summary struct {
	Results []Result
	Errors  []error
}

// StandardSite materializes Standard.site discovery and document artifacts.
func StandardSite(projectRoot, outputRoot string, cfg config.Config, project state.ProjectState) Summary {
	var summary Summary
	standard := cfg.Integrations.StandardSite
	integrationState, ok := project.Integrations["standard-site"]
	if !ok || integrationState.Publication == nil || integrationState.Publication.URI == "" {
		summary.Errors = append(summary.Errors, errors.New("Standard.site publication state is missing from .pdg/state.json"))
		return summary
	}
	publication := integrationState.Publication.URI
	if err := reconcile.ValidatePublicationURI(publication, standard.Identity); err != nil {
		summary.Errors = append(summary.Errors, fmt.Errorf("validate Standard.site publication: %w", err))
		return summary
	}
	if err := materializePublication(projectRoot, standard.PublicDir, publication); err != nil {
		summary.Errors = append(summary.Errors, fmt.Errorf("publication discovery artifact: %w", err))
	} else {
		summary.Results = append(summary.Results, Result{Path: ".well-known/site.standard.publication", Action: "ready"})
	}
	for path, document := range project.Documents {
		if document == nil || document.Status == state.StatusMissing || document.Source == "" {
			continue
		}
		target, exists := document.Targets[state.TargetStandardSite]
		if !exists || target.URI == "" {
			continue
		}
		if err := validateDocumentURI(target.URI, standard.Identity); err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		source, err := safeSource(outputRoot, document.Source)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		action, err := injectLink(source, target.URI)
		if errors.Is(err, os.ErrNotExist) {
			summary.Results = append(summary.Results, Result{Path: path, Action: "skipped: generated HTML not found"})
			continue
		}
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		summary.Results = append(summary.Results, Result{Path: path, Action: action})
	}
	return summary
}

func materializePublication(root, publicDir, publication string) error {
	if publicDir == "" || publicDir == "/" {
		publicDir = "."
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(filepath.Join(root, publicDir))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("public directory %q is outside the project root", publicDir)
	}
	return atomicWrite(filepath.Join(dir, ".well-known", "site.standard.publication"), []byte(publication+"\n"), 0o644)
}

func safeSource(root, source string) (string, error) {
	if filepath.IsAbs(source) {
		return "", errors.New("generated source path is absolute")
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(source)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("generated source path is outside the output directory")
	}
	if strings.ToLower(filepath.Ext(target)) != ".html" {
		return "", errors.New("generated source is not an HTML file")
	}
	return target, nil
}

func validateDocumentURI(raw, did string) error {
	parts := strings.Split(strings.TrimPrefix(raw, "at://"), "/")
	if !strings.HasPrefix(raw, "at://") || len(parts) != 3 || parts[0] != did || parts[1] != "site.standard.document" || parts[2] == "" || strings.ContainsAny(raw, "?#") {
		return errors.New("invalid Standard.site document URI")
	}
	return nil
}

func injectLink(filename, uri string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	var head *html.Node
	var links []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "head" && head == nil {
			head = node
		}
		if node.Type == html.ElementNode && node.Data == "link" && hasRel(node, "site.standard.document") {
			links = append(links, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if head == nil {
		return "", errors.New("generated HTML has no <head>")
	}
	if len(links) == 1 && attr(links[0], "href") == uri {
		return "unchanged", nil
	}
	for _, link := range links {
		link.Parent.RemoveChild(link)
	}
	link := &html.Node{Type: html.ElementNode, Data: "link", Attr: []html.Attribute{{Key: "rel", Val: "site.standard.document"}, {Key: "href", Val: uri}}}
	head.AppendChild(link)
	var output bytes.Buffer
	if err := html.Render(&output, doc); err != nil {
		return "", err
	}
	if err := atomicWrite(filename, output.Bytes(), 0o644); err != nil {
		return "", err
	}
	return "injected", nil
}

func hasRel(node *html.Node, wanted string) bool {
	for _, value := range strings.Fields(attr(node, "rel")) {
		if value == wanted {
			return true
		}
	}
	return false
}
func attr(node *html.Node, name string) string {
	for _, value := range node.Attr {
		if value.Key == name {
			return value.Val
		}
	}
	return ""
}

func atomicWrite(filename string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(filename), ".pdg-inject-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filename)
}
