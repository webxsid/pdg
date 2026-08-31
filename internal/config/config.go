package config

/**
* Fediverse config is only a stub for now, but it will be used in the future to configure the fediverse integration.
 */

// Config represents the configuration for the application.
type Config struct {
	Site         SiteConfig        `yaml:"site"`
	ATProto      ATProtoConfig     `yaml:"atproto,omitempty"`
	Integrations IntegrationConfig `yaml:"integrations,omitempty"`
	Fediverse    FediverseConfig   `yaml:"fediverse,omitempty"`
	Scan         ScanConfig        `yaml:"scan,omitempty"`
}

type IntegrationConfig struct {
	StandardSite StandardSiteIntegration `yaml:"standard_site"`
	Bluesky      BlueskyIntegration      `yaml:"bluesky"`
}

type StandardSiteIntegration struct {
	Enabled     bool              `yaml:"enabled"`
	Identity    string            `yaml:"identity"`
	Publication string            `yaml:"publication"`
	PublicDir   string            `yaml:"public_dir"`
	Paths       []PublicationPath `yaml:"paths"`
}

type BlueskyIntegration struct {
	Enabled  bool              `yaml:"enabled"`
	Identity string            `yaml:"identity"`
	Paths    []PublicationPath `yaml:"paths"`
}

// SiteConfig represents the configuration for the site.
type SiteConfig struct {
	URL string `yaml:"url"`
}

type ScanConfig struct {
	Dist     string        `yaml:"dist"`
	Scopes   []ScanScope   `yaml:"scope"`
	Metadata MetadataRules `yaml:"metadata"`
}

type ScanScope struct {
	Path    string   `yaml:"path"`
	Mode    string   `yaml:"mode"`
	Targets []string `yaml:"targets"`
}

type MetadataRules struct {
	Title       ExtractionRule `yaml:"title"`
	Description ExtractionRule `yaml:"description"`
	PublishedAt ExtractionRule `yaml:"published_at"`
	UpdatedAt   ExtractionRule `yaml:"updated_at"`
	Tags        ExtractionRule `yaml:"tags"`
}

type ExtractionRule struct {
	Selector  string `yaml:"selector"`
	Source    string `yaml:"source"`
	Attribute string `yaml:"attribute"`
}

// ATProtoConfig represents the configuration for the AT Protocol integration.
type ATProtoConfig struct {
	Enabled      bool               `yaml:"enabled"`
	Identity     string             `yaml:"identity"`
	StandardSite StandardSiteConfig `yaml:"standard_site"`
	Bluesky      BlueskyConfig      `yaml:"bluesky"`
}

type StandardSiteConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Publication string `yaml:"publication"`
}

type BlueskyConfig struct {
	Enabled bool `yaml:"enabled"`
}

// FediverseConfig represents the configuration for the Fediverse integration.
type FediverseConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
}
