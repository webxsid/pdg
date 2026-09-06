package cli

import "testing"

func TestMergeScopesPreservesExistingCapabilities(t *testing.T) {
	existing := "atproto repo:site.standard.document?action=create&action=update"
	requested := "atproto repo:app.bsky.feed.post?action=create"

	got := mergeScopes(existing, requested)
	if got != "atproto repo:site.standard.document?action=create&action=update repo:app.bsky.feed.post?action=create" {
		t.Errorf("mergeScopes() = %q, want existing and requested scopes", got)
	}
}

func TestHasScopesRequiresEveryRequestedScope(t *testing.T) {
	granted := "atproto repo:site.standard.document?action=create&action=update"
	required := "atproto repo:site.standard.document?action=create&action=update repo:app.bsky.feed.post?action=create"

	if hasScopes(granted, required) {
		t.Fatal("hasScopes() = true, want false when a requested scope is missing")
	}
}
