package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/webxsid/pdg/internal/shared"
)

// WriteConfig writes the configuration to the specified file path.
func WriteConfig(path string, config Config) error {
	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o644,
	)
	if err != nil {
		return fmt.Errorf("failed to open config file for writing: %w", err)
	}
	defer shared.SafeCloseFile(file)

	if err := toml.NewEncoder(file).Encode(config); err != nil {
		return fmt.Errorf("failed to encode config to TOML: %w", err)
	}

	return nil
}
