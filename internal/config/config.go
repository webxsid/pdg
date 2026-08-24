package config

/**
* Fediverse config is only a stub for now, but it will be used in the future to configure the fediverse integration.
 */

// Config represents the configuration for the application.
type Config struct {
	Site      SiteConfig      `toml:"site"`
	ATProto   ATProtoConfig   `toml:"atproto"`
	Fediverse FediverseConfig `toml:"fediverse"`
}

// SiteConfig represents the configuration for the site.
type SiteConfig struct {
	URL       string `toml:"url"`
	OutputDir string `toml:"output_dir"`
}

// ATProtoConfig represents the configuration for the AT Protocol integration.
type ATProtoConfig struct {
	Identity string `toml:"identity"`
}

// FediverseConfig represents the configuration for the Fediverse integration.
type FediverseConfig struct {
	Enabled  bool   `toml:"enabled"`
	Username string `toml:"username"`
}
