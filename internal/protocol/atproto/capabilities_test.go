package atproto

import "testing"

func TestScopesForStandardSite(t *testing.T) {
	scopes := ScopesForCapabilities(CapabilityStandardSitePublication, CapabilityStandardSiteDocuments)
	want := map[string]bool{
		"atproto": true,
		"repo:site.standard.publication?action=create&action=update&action=delete": true,
		"repo:site.standard.document?action=create&action=update":                  true,
	}
	if len(scopes) != len(want) {
		t.Fatalf("ScopesForCapabilities() = %v, want %v", scopes, want)
	}
	for _, scope := range scopes {
		if !want[scope] {
			t.Errorf("unexpected scope %q", scope)
		}
	}
	if scopes[0] != "atproto" {
		t.Errorf("first scope = %q, want atproto", scopes[0])
	}
}
