package scan

import (
	"context"
	"fmt"
	"io/fs"
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
		scope, applicable, err := selectScope(filePath, options.Config.Scan.Scopes)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, err))
			return nil
		}
		if !applicable {
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
		websitePath, err := websitePath(filePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, err))
			return nil
		}
		if seen[websitePath] {
			result.Errors = append(result.Errors, fmt.Errorf("path collision for %s", websitePath))
			return nil
		}
		seen[websitePath] = true
		targets := scopeTargets(scope)
		if scope.Mode == "explicit" && !doc.TargetsExplicit {
			return nil
		}
		if doc.TargetsExplicit {
			targets = doc.Targets
		}
		if err := validateEnabledTargets(targets, options.Config); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", websitePath, err))
			return nil
		}
		metadata := state.DocumentMetadata{Title: doc.Title, Description: doc.Description, TextContent: doc.TextContent, Tags: doc.Tags}
		if !doc.PublishedAt.IsZero() {
			value := doc.PublishedAt
			metadata.PublishedAt = &value
		}
		metadata.UpdatedAt = doc.UpdatedAt
		fingerprint := state.Fingerprint(metadata, websitePath, doc.CanonicalURL)
		prior := options.Previous.Documents[websitePath]
		status := state.StatusDiscovered
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
				targetMap[target] = value
			}
		}
		for _, target := range targets {
			targetMap[target] = state.TargetState{Selected: true}
		}
		for target, value := range targetMap {
			if !containsTarget(targets, target) {
				value.Selected = false
				targetMap[target] = value
			}
		}
		if len(targets) == 0 {
			status = state.StatusUnpublished
		}
		documentState := &state.DocumentState{Path: websitePath, Source: filePath, CanonicalURL: doc.CanonicalURL, Status: status, Fingerprint: fingerprint, Metadata: metadata, Targets: targetMap}
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
			current.Documents[path] = prior
			continue
		}
		copy := *prior
		copy.Status = state.StatusMissing
		current.Documents[path] = &copy
	}
	sort.Slice(result.Documents, func(i, j int) bool { return result.Documents[i].Path < result.Documents[j].Path })
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("scan found %d error(s)", len(result.Errors))
	}
	return result, nil
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

func selectScope(filePath string, scopes []config.ScanScope) (config.ScanScope, bool, error) {
	clean := path.Clean(strings.TrimPrefix(filePath, "./"))
	var selected config.ScanScope
	best := -1
	for _, scope := range scopes {
		prefix := path.Clean(strings.Trim(scope.Path, "/"))
		if prefix == "." {
			prefix = ""
		}
		if prefix != "" && clean != prefix && !strings.HasPrefix(clean, prefix+"/") {
			continue
		}
		if len(prefix) > best {
			selected, best = scope, len(prefix)
		}
	}
	if best < 0 {
		if len(scopes) == 0 {
			return config.ScanScope{Mode: "all"}, true, nil
		}
		return config.ScanScope{}, false, nil
	}
	if selected.Mode == "" {
		selected.Mode = "all"
	}
	if selected.Mode != "all" && selected.Mode != "explicit" {
		return config.ScanScope{}, false, fmt.Errorf("invalid scan scope mode %q", selected.Mode)
	}
	return selected, true, nil
}

func scopeTargets(scope config.ScanScope) []state.PublicationTarget {
	result := []state.PublicationTarget{}
	for _, target := range scope.Targets {
		result = append(result, state.PublicationTarget(target))
	}
	return result
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
			if !cfg.ATProto.StandardSite.Enabled {
				return fmt.Errorf("target %q is not enabled", target)
			}
		case state.TargetBluesky:
			if !cfg.ATProto.Bluesky.Enabled {
				return fmt.Errorf("target %q is not enabled", target)
			}
		default:
			return fmt.Errorf("unknown publication target %q", target)
		}
	}
	return nil
}
