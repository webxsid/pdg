package config

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type PublicationMode string

const (
	PublicationAll      PublicationMode = "all"
	PublicationExplicit PublicationMode = "explicit"
)

type PublicationPath struct {
	Path    string          `yaml:"path"`
	Publish PublicationMode `yaml:"publish"`
}

func (p *PublicationPath) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		p.Path, p.Publish = value.Value, PublicationAll
		return nil
	}
	type plain PublicationPath
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*p = PublicationPath(decoded)
	return nil
}

func NormalizePublicationPaths(paths []PublicationPath) ([]PublicationPath, error) {
	result := make([]PublicationPath, 0, len(paths))
	seen := make(map[string]PublicationMode, len(paths))
	for _, item := range paths {
		value := strings.TrimSpace(item.Path)
		if value == "" {
			return nil, fmt.Errorf("publication path cannot be empty")
		}
		if value != "/" {
			value = path.Clean("/" + value)
			value = strings.TrimPrefix(value, "/")
			if value == "." || strings.HasPrefix(value, "../") {
				return nil, fmt.Errorf("invalid publication path %q", item.Path)
			}
		}
		mode := item.Publish
		if mode == "" {
			return nil, fmt.Errorf("publication path %q has no publish mode", value)
		}
		if mode != PublicationAll && mode != PublicationExplicit {
			return nil, fmt.Errorf("publication path %q has unknown publish mode %q", value, mode)
		}
		if previous, ok := seen[value]; ok {
			if previous != mode {
				return nil, fmt.Errorf("publication path %q has conflicting publish modes", value)
			}
			continue
		}
		seen[value] = mode
		result = append(result, PublicationPath{Path: value, Publish: mode})
	}
	return result, nil
}
