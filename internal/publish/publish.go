package publish

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/state"
)

const standardSiteDocumentCollection = "site.standard.document"

type Action string

const (
	ActionCreate Action = "created"
	ActionUpdate Action = "updated"
	ActionNoop   Action = "unchanged"
	ActionSkip   Action = "skipped"
)

type RecordClient interface {
	CreateRecord(context.Context, atproto.Session, string, any) (string, error)
	UpdateRecord(context.Context, atproto.Session, string, string, any) (string, error)
}

type Result struct {
	Path   string
	Action Action
}

type Summary struct {
	Results []Result
	Errors  []error
}

// StandardSite publishes selected documents from the persisted project state.
func StandardSite(ctx context.Context, project *state.ProjectState, session atproto.Session, client RecordClient, persist func(state.ProjectState) error) Summary {
	var summary Summary
	publication, ok := project.Integrations["standard-site"]
	if !ok || publication.Publication == nil || publication.Publication.URI == "" {
		summary.Errors = append(summary.Errors, errors.New("Standard.site publication state is missing from .pdg/state.json"))
		return summary
	}
	if err := validateRecordURI(publication.Publication.URI, session.Identity.DID, "site.standard.publication"); err != nil {
		summary.Errors = append(summary.Errors, fmt.Errorf("validate Standard.site publication: %w", err))
		return summary
	}
	for path, document := range project.Documents {
		if document == nil || document.Status == state.StatusMissing {
			continue
		}
		target, selected := document.Targets[state.TargetStandardSite]
		if !selected || !target.Selected {
			continue
		}
		if target.URI == "" && target.PublishedFingerprint != "" {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: Standard.site target has a fingerprint but no URI", path))
			continue
		}
		if target.URI != "" {
			if err := validateRecordURI(target.URI, session.Identity.DID, standardSiteDocumentCollection); err != nil {
				summary.Errors = append(summary.Errors, fmt.Errorf("%s: %w", path, err))
				continue
			}
			if target.PublishedFingerprint == document.Fingerprint {
				summary.Results = append(summary.Results, Result{Path: path, Action: ActionNoop})
				continue
			}
			if err := validateDocument(document); err != nil {
				summary.Errors = append(summary.Errors, fmt.Errorf("%s: %w", path, err))
				continue
			}
			_, rkey, _ := splitRecordURI(target.URI)
			cid, err := client.UpdateRecord(ctx, session, standardSiteDocumentCollection, rkey, documentRecord(document, publication.Publication.URI))
			if err != nil {
				summary.Errors = append(summary.Errors, fmt.Errorf("%s: update Standard.site document: %w", path, err))
				continue
			}
			target.CID = cid
			target.PublishedFingerprint = document.Fingerprint
			document.Targets[state.TargetStandardSite] = target
			if err := persist(*project); err != nil {
				summary.Errors = append(summary.Errors, fmt.Errorf("%s: persist updated publication state: %w", path, err))
				return summary
			}
			summary.Results = append(summary.Results, Result{Path: path, Action: ActionUpdate})
			continue
		}
		if err := validateDocument(document); err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}
		uri, err := client.CreateRecord(ctx, session, standardSiteDocumentCollection, documentRecord(document, publication.Publication.URI))
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: create Standard.site document: %w", path, err))
			continue
		}
		if err := validateRecordURI(uri, session.Identity.DID, standardSiteDocumentCollection); err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: validate created document: %w", path, err))
			continue
		}
		target.URI = uri
		target.PublishedFingerprint = document.Fingerprint
		document.Targets[state.TargetStandardSite] = target
		if err := persist(*project); err != nil {
			summary.Errors = append(summary.Errors, fmt.Errorf("%s: persist created publication state: %w", path, err))
			return summary
		}
		summary.Results = append(summary.Results, Result{Path: path, Action: ActionCreate})
	}
	return summary
}

func documentRecord(document *state.DocumentState, publication string) map[string]any {
	record := map[string]any{
		"$type": "site.standard.document", "site": publication, "path": document.Path,
		"title": document.Metadata.Title, "publishedAt": document.Metadata.PublishedAt.Format(time.RFC3339),
	}
	if document.Metadata.Description != "" {
		record["description"] = document.Metadata.Description
	}
	if document.Metadata.TextContent != "" {
		record["textContent"] = document.Metadata.TextContent
	}
	if len(document.Metadata.Tags) > 0 {
		record["tags"] = document.Metadata.Tags
	}
	if document.Metadata.UpdatedAt != nil {
		record["updatedAt"] = document.Metadata.UpdatedAt.Format(time.RFC3339)
	}
	return record
}

func validateDocument(document *state.DocumentState) error {
	if document.Metadata.PublishedAt == nil {
		return errors.New("cannot publish to Standard.site: missing published_at")
	}
	return nil
}

func validateRecordURI(raw, expectedDID, collection string) error {
	parts, _, err := splitRecordURI(raw)
	if err != nil || parts != expectedDID {
		return fmt.Errorf("record URI is not for DID %s", expectedDID)
	}
	uriParts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(raw, "at://"), "/"), "/")
	if len(uriParts) != 3 || uriParts[1] != collection {
		return fmt.Errorf("record URI must use collection %s", collection)
	}
	return nil
}

func splitRecordURI(raw string) (string, string, error) {
	if !strings.HasPrefix(raw, "at://") {
		return "", "", errors.New("invalid AT URI")
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(raw, "at://"), "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" || strings.ContainsAny(raw, "?#") {
		return "", "", errors.New("invalid AT record URI")
	}
	return parts[0], parts[2], nil
}
