package post

import (
	"testing"

	"github.com/webxsid/pdg/internal/state"
)

func TestBuildPreviewUsesBlueskyCopyAndURL(t *testing.T) {
	document := &state.DocumentState{Path: "/blog/foo", CanonicalURL: "https://example.com/blog/foo", Metadata: state.DocumentMetadata{Title: "Title", Bluesky: "Custom copy."}, Targets: map[state.PublicationTarget]state.TargetState{state.TargetStandardSite: {URI: "at://did:plc:a/site.standard.document/doc"}}}
	preview, err := BuildPreview(document, "")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Text != "Custom copy.\nhttps://example.com/blog/foo" {
		t.Errorf("Text = %q", preview.Text)
	}
}

func TestBuildPreviewContentOverrideDoesNotDuplicateURL(t *testing.T) {
	document := &state.DocumentState{Path: "/blog/foo", CanonicalURL: "https://example.com/blog/foo", Metadata: state.DocumentMetadata{Title: "Title"}, Targets: map[state.PublicationTarget]state.TargetState{state.TargetStandardSite: {URI: "at://did:plc:a/site.standard.document/doc"}}}
	preview, err := BuildPreview(document, "Custom https://example.com/blog/foo")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Text != "Custom https://example.com/blog/foo" {
		t.Errorf("Text = %q", preview.Text)
	}
}
