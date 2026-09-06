package scan

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/state"
)

type StatefulOptions struct {
	Config   config.Config
	Previous state.ProjectState
}

type StatefulResult struct {
	State     state.ProjectState
	Documents []*state.DocumentState
	Errors    []error
}

func (s *Scanner) ScanStateful(ctx context.Context, filesystem fs.FS, options StatefulOptions) (StatefulResult, error) {
	current := state.New()
	if options.Previous.Documents != nil {
		current = options.Previous
	}
	result := StatefulResult{State: current}
	seen := map[string]bool{}
	err := fs.WalkDir(filesystem, ".", func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || !isHTML(filePath) {
			return nil
		}
		websitePath, err := websitePath(filePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, err))
			return nil
		}
		prior := options.Previous.Documents[websitePath]
		potential := hasPotentialTarget(websitePath, options.Config)
		if seen[websitePath] {
			result.Errors = append(result.Errors, fmt.Errorf("path collision for %s (including %s)", websitePath, filePath))
			return nil
		}
		seen[websitePath] = true
		if !potential {
			if prior != nil {
				copy := sanitizedDocument(prior)
				copy.Status = state.StatusOutOfScope
				current.Documents[websitePath] = copy
				result.Documents = append(result.Documents, copy)
			}
			return nil
		}
		file, err := filesystem.Open(filePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: open: %w", filePath, err))
			return nil
		}
		doc, parseErr := ParseHTML(file)
		_ = file.Close()
		if parseErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, parseErr))
			return nil
		}
		targets := effectiveTargets(websitePath, doc, options.Config)
		if err := validateEnabledTargets(targets, options.Config); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", websitePath, err))
			return nil
		}
		metadata := state.DocumentMetadata{Title: doc.Title, Description: doc.Description, TextContent: doc.TextContent, Tags: doc.Tags, Bluesky: doc.Bluesky}
		canonical := doc.CanonicalURL
		if canonical == "" {
			base, parseErr := url.Parse(options.Config.Site.URL)
			if parseErr != nil || base.Scheme == "" || base.Host == "" {
				result.Errors = append(result.Errors, fmt.Errorf("%s: invalid site URL", websitePath))
				return nil
			}
			canonical = strings.TrimRight(base.String(), "/") + websitePath
		}
		if !doc.PublishedAt.IsZero() {
			value := doc.PublishedAt
			metadata.PublishedAt = &value
		}
		metadata.UpdatedAt = doc.UpdatedAt
		fingerprint := state.Fingerprint(metadata, websitePath, canonical)
		status := state.StatusDiscovered
		if !potential {
			status = state.StatusOutOfScope
		}
		if prior != nil {
			if prior.Fingerprint == fingerprint {
				status = state.StatusUnchanged
			} else {
				status = state.StatusModified
			}
		}
		targetMap := map[state.PublicationTarget]state.TargetState{}
		if prior != nil {
			for target, value := range prior.Targets {
				if target == state.TargetStandardSite {
					targetMap[target] = value
				}
			}
		}
		for _, target := range potentialTargets(websitePath, options.Config) {
			if _, exists := targetMap[target]; !exists {
				targetMap[target] = state.TargetState{}
			}
		}
		for _, target := range targets {
			value := targetMap[target]
			value.Selected = true
			targetMap[target] = value
		}
		for target, value := range targetMap {
			if !containsTarget(targets, target) {
				value.Selected = false
				targetMap[target] = value
			}
		}
		if potential && len(targets) == 0 {
			status = state.StatusUnpublished
		}
		documentState := &state.DocumentState{Path: websitePath, Source: filePath, CanonicalURL: canonical, Status: status, Fingerprint: fingerprint, Metadata: metadata, Targets: targetMap, Posts: priorPosts(prior)}
		current.Documents[websitePath] = documentState
		result.Documents = append(result.Documents, documentState)
		return nil
	})
	if err != nil {
		return StatefulResult{}, err
	}
	for path, prior := range options.Previous.Documents {
		if seen[path] {
			continue
		}
		if prior.Status == state.StatusOutOfScope {
			current.Documents[path] = sanitizedDocument(prior)
			continue
		}
		copy := sanitizedDocument(prior)
		copy.Status = state.StatusMissing
		current.Documents[path] = copy
	}
	sort.Slice(result.Documents, func(i, j int) bool { return result.Documents[i].Path < result.Documents[j].Path })
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("scan found %d error(s)", len(result.Errors))
	}
	return result, nil
}

func sanitizedDocument(document *state.DocumentState) *state.DocumentState {
	if document == nil {
		return nil
	}
	copy := *document
	copy.Targets = map[state.PublicationTarget]state.TargetState{}
	if target, ok := document.Targets[state.TargetStandardSite]; ok {
		copy.Targets[state.TargetStandardSite] = target
	}
	if document.Posts != nil {
		copy.Posts = make(map[string]state.PostState, len(document.Posts))
		for key, value := range document.Posts {
			copy.Posts[key] = value
		}
	}
	return &copy
}

func websitePath(filePath string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(filePath, "./"))
	if path.Ext(clean) == "" {
		return "", fmt.Errorf("not a document path")
	}
	if path.Base(clean) == "index.html" || path.Base(clean) == "index.htm" {
		clean = path.Dir(clean)
	} else {
		clean = strings.TrimSuffix(clean, path.Ext(clean))
	}
	if clean == "." {
		return "/", nil
	}
	return state.NormalizePath(clean), nil
}

func containsTarget(targets []state.PublicationTarget, target state.PublicationTarget) bool {
	for _, value := range targets {
		if value == target {
			return true
		}
	}
	return false
}
func validateEnabledTargets(targets []state.PublicationTarget, cfg config.Config) error {
	for _, target := range targets {
		switch target {
		case state.TargetStandardSite:
			if !cfg.Integrations.StandardSite.Enabled {
				return fmt.Errorf("target %q is not enabled", target)
			}
		default:
			return fmt.Errorf("unknown publication target %q", target)
		}
	}
	return nil
}

func effectiveTargets(websitePath string, doc Document, cfg config.Config) []state.PublicationTarget {
	result := potentialTargets(websitePath, cfg)
	if doc.TargetsNone {
		return result[:0]
	}
	filtered := result[:0]
	for _, target := range result {
		mode := matchingPublicationMode(websitePath, target, cfg)
		if mode != config.PublicationExplicit || containsTarget(doc.Targets, target) {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func potentialTargets(websitePath string, cfg config.Config) []state.PublicationTarget {
	result := []state.PublicationTarget{}
	if matchesPublicationPath(websitePath, cfg.Integrations.StandardSite.Paths) && cfg.Integrations.StandardSite.Enabled {
		result = append(result, state.TargetStandardSite)
	}
	return result
}

func matchesPublicationPath(document string, paths []config.PublicationPath) bool {
	return matchingPath(document, paths) != ""
}

func hasPotentialTarget(document string, cfg config.Config) bool {
	return cfg.Integrations.StandardSite.Enabled && matchesPublicationPath(document, cfg.Integrations.StandardSite.Paths)
}

func matchingPublicationMode(document string, target state.PublicationTarget, cfg config.Config) config.PublicationMode {
	paths := cfg.Integrations.StandardSite.Paths
	bestPath, bestMode := "", config.PublicationMode("")
	for _, item := range paths {
		value := state.NormalizePath(item.Path)
		if value == "/" || document == value || strings.HasPrefix(document, value+"/") {
			if pathDepth(value) > pathDepth(bestPath) {
				bestPath, bestMode = value, item.Publish
			}
		}
	}
	return bestMode
}

func priorPosts(document *state.DocumentState) map[string]state.PostState {
	if document == nil || len(document.Posts) == 0 {
		return nil
	}
	posts := make(map[string]state.PostState, len(document.Posts))
	for name, post := range document.Posts {
		posts[name] = post
	}
	return posts
}

func matchingPath(document string, paths []config.PublicationPath) string {
	best := ""
	for _, item := range paths {
		value := state.NormalizePath(item.Path)
		if value == "/" || document == value || strings.HasPrefix(document, value+"/") {
			if pathDepth(value) > pathDepth(best) {
				best = value
			}
		}
	}
	return best
}
func pathDepth(value string) int {
	if value == "" || value == "/" {
		return 0
	}
	return len(strings.Split(strings.Trim(value, "/"), "/"))
}
