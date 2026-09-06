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
	Enabled  bool   `yaml:"enabled"`
	Identity string `yaml:"identity"`
}

// SiteConfig represents the configuration for the site.
type SiteConfig struct {
	URL string `yaml:"url"`
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
