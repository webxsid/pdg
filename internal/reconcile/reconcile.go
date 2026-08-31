package reconcile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/state"
)

const standardSite = "standard-site"
const publicationCollection = "site.standard.publication"

type Status string

const (
	OK       Status = "ok"
	Drift    Status = "drift"
	Conflict Status = "conflict"
	Failure  Status = "error"
)

type Finding struct {
	Integration string
	Resource    string
	Status      Status
	Message     string
	Repairable  bool
	Expected    string
	Actual      string
}

type Project struct {
	Config config.Config
	State  state.ProjectState
	Root   string
}

func VerifyStandardSite(project Project) []Finding {
	standard := project.Config.Integrations.StandardSite
	if !standard.Enabled {
		return []Finding{{Integration: standardSite, Resource: "configuration", Status: Failure, Message: "Standard.site is not configured for this project."}}
	}
	if standard.Identity == "" {
		return []Finding{{Integration: standardSite, Resource: "identity", Status: Conflict, Message: "Standard.site identity is empty."}}
	}
	integrationState, ok := project.State.Integrations[standardSite]
	if !ok || integrationState.Publication == nil || integrationState.Publication.URI == "" {
		return []Finding{{Integration: standardSite, Resource: "publication state", Status: Conflict, Message: "No provisioned Standard.site publication exists in .pdg/state.json.", Repairable: false}}
	}
	uri := integrationState.Publication.URI
	if err := ValidatePublicationURI(uri, standard.Identity); err != nil {
		return []Finding{{Integration: standardSite, Resource: "publication identity", Status: Conflict, Message: err.Error()}}
	}
	path, err := artifactPath(project.Root, standard.PublicDir)
	if err != nil {
		return []Finding{{Integration: standardSite, Resource: ".well-known/site.standard.publication", Status: Failure, Message: err.Error()}}
	}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Finding{{Integration: standardSite, Resource: ".well-known/site.standard.publication", Status: Drift, Message: "Missing.", Repairable: true, Expected: uri + "\n"}}
	}
	if err != nil {
		return []Finding{{Integration: standardSite, Resource: ".well-known/site.standard.publication", Status: Failure, Message: err.Error()}}
	}
	expected := uri + "\n"
	if string(contents) != expected {
		return []Finding{{Integration: standardSite, Resource: ".well-known/site.standard.publication", Status: Drift, Message: "Contents do not match the provisioned publication.", Repairable: true, Expected: expected, Actual: string(contents)}}
	}
	return []Finding{{Integration: standardSite, Resource: "publication state", Status: OK, Message: "Publication state and identity are valid."}, {Integration: standardSite, Resource: ".well-known/site.standard.publication", Status: OK, Message: "Artifact is correct."}}
}

func RepairStandardSite(project Project, findings []Finding) error {
	standard := project.Config.Integrations.StandardSite
	var expected string
	for _, finding := range findings {
		if finding.Repairable && finding.Resource == ".well-known/site.standard.publication" {
			expected = finding.Expected
			break
		}
	}
	if expected == "" {
		return nil
	}
	path, err := artifactPath(project.Root, standard.PublicDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".well-known-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(expected); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func ValidatePublicationURI(raw, did string) error {
	parts := strings.Split(strings.TrimPrefix(raw, "at://"), "/")
	if !strings.HasPrefix(raw, "at://") || len(parts) != 3 || parts[0] != did || parts[1] != publicationCollection || parts[2] == "" || strings.ContainsAny(parts[2], "?#") {
		return fmt.Errorf("publication URI is not a valid %s record for %s", publicationCollection, did)
	}
	return nil
}

func artifactPath(root, publicDir string) (string, error) {
	if publicDir == "" || publicDir == "/" {
		publicDir = "."
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, publicDir))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("public directory %q is outside the project root", publicDir)
	}
	return filepath.Join(target, ".well-known", "site.standard.publication"), nil
}
