package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PublicationTarget string

const (
	TargetStandardSite PublicationTarget = "standard-site"
	TargetBluesky      PublicationTarget = "bluesky"
)

type DocumentStatus string

const (
	StatusDiscovered  DocumentStatus = "discovered"
	StatusModified    DocumentStatus = "modified"
	StatusUnchanged   DocumentStatus = "unchanged"
	StatusMissing     DocumentStatus = "missing"
	StatusUnpublished DocumentStatus = "unpublished"
	StatusOutOfScope  DocumentStatus = "out_of_scope"
)

type DocumentMetadata struct {
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	TextContent string     `json:"text_content,omitempty"`
}

type TargetState struct {
	Selected             bool       `json:"selected"`
	URI                  string     `json:"uri,omitempty"`
	CID                  string     `json:"cid,omitempty"`
	PublishedFingerprint string     `json:"published_fingerprint,omitempty"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
}
type TargetChange struct {
	Target PublicationTarget `json:"target"`
	Added  bool              `json:"added"`
}

type DocumentState struct {
	Path          string                            `json:"path"`
	Source        string                            `json:"source"`
	CanonicalURL  string                            `json:"canonical_url"`
	Status        DocumentStatus                    `json:"status"`
	Fingerprint   string                            `json:"fingerprint"`
	Metadata      DocumentMetadata                  `json:"metadata"`
	Targets       map[PublicationTarget]TargetState `json:"targets"`
	TargetChanges []TargetChange                    `json:"target_changes,omitempty"`
}

type ProjectState struct {
	Version      int                         `json:"version"`
	Documents    map[string]*DocumentState   `json:"documents"`
	Integrations map[string]IntegrationState `json:"integrations,omitempty"`
}

type IntegrationState struct {
	Publication *PublicationState `json:"publication,omitempty"`
}

type PublicationState struct {
	URI string `json:"uri"`
}

func New() ProjectState {
	return ProjectState{Version: 1, Documents: map[string]*DocumentState{}, Integrations: map[string]IntegrationState{}}
}

func Load(path string) (ProjectState, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return ProjectState{}, fmt.Errorf("read project state: %w", err)
	}
	var result ProjectState
	if err := json.Unmarshal(data, &result); err != nil {
		return ProjectState{}, fmt.Errorf("decode project state: %w", err)
	}
	if result.Version != 1 {
		return ProjectState{}, fmt.Errorf("unsupported project state version %d", result.Version)
	}
	if result.Documents == nil {
		result.Documents = map[string]*DocumentState{}
	}
	if result.Integrations == nil {
		result.Integrations = map[string]IntegrationState{}
	}
	return result, nil
}

func Write(path string, project ProjectState) error {
	if project.Version == 0 {
		project.Version = 1
	}
	if project.Documents == nil {
		project.Documents = map[string]*DocumentState{}
	}
	if project.Integrations == nil {
		project.Integrations = map[string]IntegrationState{}
	}
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project state: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create project state directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return fmt.Errorf("create project state temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write project state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync project state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close project state: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace project state: %w", err)
	}
	return nil
}

func Fingerprint(metadata DocumentMetadata, path, canonical string) string {
	value := struct {
		Path, Canonical, Title, Description, Text string
		Published, Updated                        *time.Time
		Tags                                      []string
	}{path, canonical, metadata.Title, metadata.Description, metadata.TextContent, metadata.PublishedAt, metadata.UpdatedAt, metadata.Tags}
	data, _ := json.Marshal(value)
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func NormalizePath(value string) string {
	value = "/" + strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
	if value == "/" {
		return value
	}
	return value
}
