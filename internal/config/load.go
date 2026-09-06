package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultFilename = "pdg.yaml"

// LoadConfig loads the configuration from the specified file path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}
	if config.Integrations.StandardSite.Paths, err = NormalizePublicationPaths(config.Integrations.StandardSite.Paths); err != nil {
		return nil, fmt.Errorf("validate Standard.site paths: %w", err)
	}
	return &config, nil
}
