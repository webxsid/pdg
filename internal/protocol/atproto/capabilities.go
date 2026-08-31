package atproto

import "sort"

// Capability identifies an ATProto operation required by a project integration.
type Capability string

const (
	CapabilityStandardSitePublication Capability = "standard-site-publication"
	CapabilityStandardSiteDocuments   Capability = "standard-site-documents"
	CapabilityBlueskyPosting          Capability = "bluesky-posting"
)

const (
	standardSitePublicationScope = "repo:site.standard.publication?action=create&action=update&action=delete"
	standardSiteDocumentsScope   = "repo:site.standard.document?action=create&action=update"
	blueskyPostingScope          = "repo:app.bsky.feed.post?action=create&action=delete"
)

// ScopesForCapabilities returns the canonical OAuth scope set for capabilities.
func ScopesForCapabilities(capabilities ...Capability) []string {
	seen := map[string]struct{}{"atproto": {}}
	for _, capability := range capabilities {
		switch capability {
		case CapabilityStandardSitePublication:
			seen[standardSitePublicationScope] = struct{}{}
		case CapabilityStandardSiteDocuments:
			seen[standardSiteDocumentsScope] = struct{}{}
		case CapabilityBlueskyPosting:
			seen[blueskyPostingScope] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Strings(result)
	for i, scope := range result {
		if scope == "atproto" {
			result[0], result[i] = result[i], result[0]
			break
		}
	}
	return result
}
