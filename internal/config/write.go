package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/webxsid/pdg/internal/shared"
	"gopkg.in/yaml.v3"
)

// WriteConfig writes the configuration to the specified file path.
func WriteConfig(path string, config Config) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".pdg-config-*")
	if err != nil {
		return fmt.Errorf("failed to open config file for writing: %w", err)
	}
	temporaryPath := file.Name()
	defer func() { shared.SafeCloseFile(file) }()
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	if err := yaml.NewEncoder(file).Encode(config); err != nil {
		return fmt.Errorf("failed to encode config to YAML: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync config file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close config file: %w", err)
	}
	file = nil
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	return nil
}
