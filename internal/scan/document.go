package scan

import (
	"time"

	"github.com/webxsid/pdg/internal/state"
)

type Document struct {
	CanonicalURL    string                    `json:"canonical_url"`
	Path            string                    `json:"path"`
	Title           string                    `json:"title"`
	Description     string                    `json:"description"`
	TextContent     string                    `json:"text_content"`
	PublishedAt     time.Time                 `json:"published_at"`
	UpdatedAt       *time.Time                `json:"updated_at"`
	Tags            []string                  `json:"tags"`
	Targets         []state.PublicationTarget `json:"targets"`
	TargetsExplicit bool                      `json:"targets_explicit"`
	TargetsNone     bool                      `json:"targets_none"`
}
