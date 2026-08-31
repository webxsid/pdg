package integration

import (
	"fmt"
	"sort"

	"github.com/webxsid/pdg/internal/protocol/atproto"
)

// Integration identifies a project-level publishing integration.
type Integration string

const (
	StandardSite Integration = "standard-site"
	Bluesky      Integration = "bluesky"
)

// Definition describes the protocol and authorization capabilities required by an integration.
type Definition struct {
	Name         Integration
	Capabilities []atproto.Capability
}

var definitions = map[Integration]Definition{
	StandardSite: {Name: StandardSite, Capabilities: []atproto.Capability{
		atproto.CapabilityStandardSitePublication,
		atproto.CapabilityStandardSiteDocuments,
	}},
	Bluesky: {Name: Bluesky, Capabilities: []atproto.Capability{atproto.CapabilityBlueskyPosting}},
}

func Resolve(name string) (Definition, error) {
	d, ok := definitions[Integration(name)]
	if !ok {
		return Definition{}, fmt.Errorf("unknown integration %q (supported: standard-site, bluesky)", name)
	}
	return d, nil
}

func Parse(names []string) ([]Definition, error) {
	seen := make(map[Integration]struct{}, len(names))
	result := make([]Definition, 0, len(names))
	for _, name := range names {
		d, err := Resolve(name)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[d.Name]; ok {
			return nil, fmt.Errorf("integration %q was specified more than once", name)
		}
		seen[d.Name] = struct{}{}
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
