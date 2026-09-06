package post

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/state"
)

const collection = "app.bsky.feed.post"

type RecordClient interface {
	CreateRecordResult(context.Context, atproto.Session, string, any) (atproto.RecordResult, error)
}

type Request struct {
	Path, Content string
	DryRun        bool
}
type Preview struct{ Text, StandardSiteURI, CanonicalURL string }

func BuildPreview(document *state.DocumentState, content string) (Preview, error) {
	if document == nil {
		return Preview{}, errors.New("document was not found")
	}
	if document.CanonicalURL == "" {
		return Preview{}, errors.New("document has no canonical URL")
	}
	target, ok := document.Targets[state.TargetStandardSite]
	if !ok || target.URI == "" {
		return Preview{}, fmt.Errorf("%s has not been published to Standard.site; run `pdg publish standard-site`", document.Path)
	}
	text := content
	if text == "" {
		text = document.Metadata.Bluesky
	}
	if text == "" {
		text = document.Metadata.Title
		if document.Metadata.Description != "" {
			text += "\n" + document.Metadata.Description
		}
	}
	if !strings.Contains(text, document.CanonicalURL) {
		text = strings.TrimSpace(text) + "\n" + document.CanonicalURL
	}
	if utf8.RuneCountInString(text) > 300 || len([]byte(text)) > 3000 {
		return Preview{}, errors.New("Bluesky post exceeds 300 graphemes or 3000 bytes")
	}
	return Preview{Text: text, StandardSiteURI: target.URI, CanonicalURL: document.CanonicalURL}, nil
}

func Create(ctx context.Context, document *state.DocumentState, content string, session atproto.Session, client RecordClient, persist func(state.PostState) error) (Preview, error) {
	preview, err := BuildPreview(document, content)
	if err != nil {
		return Preview{}, err
	}
	if document.Posts != nil {
		if existing, ok := document.Posts["bluesky"]; ok && existing.URI != "" {
			return Preview{}, fmt.Errorf("%s has already been posted to Bluesky: %s", document.Path, existing.URI)
		}
	}
	urlStart := strings.LastIndex(preview.Text, preview.CanonicalURL)
	record := map[string]any{"$type": collection, "text": preview.Text, "createdAt": time.Now().UTC().Format(time.RFC3339Nano), "facets": []any{map[string]any{"index": map[string]any{"byteStart": urlStart, "byteEnd": urlStart + len(preview.CanonicalURL)}, "features": []any{map[string]any{"$type": "app.bsky.richtext.facet#link", "uri": preview.CanonicalURL}}}}}
	result, err := client.CreateRecordResult(ctx, session, collection, record)
	if err != nil {
		return Preview{}, err
	}
	if !strings.HasPrefix(result.URI, "at://"+session.Identity.DID+"/"+collection+"/") {
		return Preview{}, errors.New("Bluesky response returned an invalid post URI")
	}
	if document.Posts == nil {
		document.Posts = map[string]state.PostState{}
	}
	document.Posts["bluesky"] = state.PostState{URI: result.URI, CID: result.CID}
	if err := persist(document.Posts["bluesky"]); err != nil {
		return Preview{}, fmt.Errorf("post created at %s but state persistence failed: %w", result.URI, err)
	}
	return preview, nil
}
