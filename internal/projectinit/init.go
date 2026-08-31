package projectinit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/state"
)

const publicationCollection = "site.standard.publication"

// Options contains the non-interactive inputs for project initialization.
type Options struct {
	SiteURL               string
	ATProtoEnabled        bool
	ATProtoDID            string
	StandardSiteEnabled   bool
	BlueskyEnabled        bool
	ConfigureIntegrations bool
}

type sessionStore interface {
	LoadSession(context.Context, string) (atproto.Session, error)
}

type publicationClient interface {
	CreateRecord(context.Context, atproto.Session, string, any) (string, error)
}

// Service initializes a PDG project and optionally bootstraps Standard.site.
type Service struct {
	ConfigPath string
	Sessions   sessionStore
	XRPC       publicationClient
}

// Run validates and persists project configuration.
func (s *Service) Run(ctx context.Context, options Options) (config.Config, error) {
	if s.Sessions == nil {
		return config.Config{}, errors.New("ATProto session store is not configured")
	}
	statePath := ""
	if path := s.ConfigPath; path != "" {
		statePath = filepath.Join(filepath.Dir(path), ".pdg", "state.json")
	} else {
		statePath = filepath.Join(".pdg", "state.json")
	}
	if _, err := os.Stat(statePath); errors.Is(err, os.ErrNotExist) {
		if err := state.Write(statePath, state.New()); err != nil {
			return config.Config{}, fmt.Errorf("initialize project state: %w", err)
		}
	} else if err != nil {
		return config.Config{}, fmt.Errorf("inspect project state: %w", err)
	} else if _, err := state.Load(statePath); err != nil {
		return config.Config{}, err
	}
	if s.XRPC == nil {
		return config.Config{}, errors.New("ATProto XRPC client is not configured")
	}
	path := s.ConfigPath
	if path == "" {
		path = config.DefaultFilename
	}
	current, err := config.LoadConfig(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config.Config{}, err
	}
	if current == nil {
		current = &config.Config{}
	}
	if options.SiteURL != "" {
		current.Site.URL = options.SiteURL
	}
	if options.ATProtoDID != "" {
		if current.ATProto.StandardSite.Publication != "" && current.ATProto.Identity != "" && current.ATProto.Identity != options.ATProtoDID {
			return config.Config{}, fmt.Errorf("cannot change ATProto identity from %s while Standard.site publication %s exists", current.ATProto.Identity, current.ATProto.StandardSite.Publication)
		}
		current.ATProto.Identity = options.ATProtoDID
	}
	if options.ConfigureIntegrations {
		current.ATProto.Enabled = options.ATProtoEnabled
		current.ATProto.StandardSite.Enabled = options.StandardSiteEnabled && options.ATProtoEnabled
		current.ATProto.Bluesky.Enabled = options.BlueskyEnabled && options.ATProtoEnabled
	} else if current.ATProto.Identity != "" {
		// Preserve the pre-V2 shape, where an identity implied ATProto enablement.
		current.ATProto.Enabled = true
	}
	if current.Site.URL == "" {
		return config.Config{}, errors.New("site URL is required")
	}
	if _, err := parseSiteURL(current.Site.URL); err != nil {
		return config.Config{}, err
	}
	if !current.ATProto.Enabled {
		current.ATProto.Identity = ""
		current.ATProto.StandardSite = config.StandardSiteConfig{}
		current.ATProto.Bluesky = config.BlueskyConfig{}
	} else if current.ATProto.Identity == "" {
		return config.Config{}, errors.New("ATProto identity DID is required when ATProto is enabled")
	}
	if current.ATProto.StandardSite.Enabled {
		if current.ATProto.StandardSite.Publication == "" {
			session, err := s.Sessions.LoadSession(ctx, current.ATProto.Identity)
			if err != nil {
				return config.Config{}, fmt.Errorf("load credentials for configured ATProto DID %s: %w; authenticate with `pdg atproto login <handle>`", current.ATProto.Identity, err)
			}
			record := map[string]any{
				"$type": publicationCollection,
				"url":   current.Site.URL,
				"name":  siteName(current.Site.URL),
			}
			uri, err := s.XRPC.CreateRecord(ctx, session, publicationCollection, record)
			if err != nil {
				return config.Config{}, fmt.Errorf("create Standard.site publication: %w", err)
			}
			if err := validatePublicationURI(uri, current.ATProto.Identity); err != nil {
				return config.Config{}, fmt.Errorf("validate created Standard.site publication %q: %w", uri, err)
			}
			current.ATProto.StandardSite.Publication = uri
		} else if err := validatePublicationURI(current.ATProto.StandardSite.Publication, current.ATProto.Identity); err != nil {
			return config.Config{}, fmt.Errorf("validate configured Standard.site publication: %w", err)
		}
	}
	if err := config.WriteConfig(path, *current); err != nil {
		if current.ATProto.StandardSite.Publication != "" {
			return config.Config{}, fmt.Errorf("write project config after creating Standard.site publication %s: %w", current.ATProto.StandardSite.Publication, err)
		}
		return config.Config{}, err
	}
	return *current, nil
}

func parseSiteURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid site URL %q", raw)
	}
	return parsed, nil
}

func siteName(raw string) string {
	parsed, _ := parseSiteURL(raw)
	return parsed.Hostname()
}

func validatePublicationURI(raw, did string) error {
	if !strings.HasPrefix(raw, "at://") {
		return errors.New("publication must be an AT URI for the configured DID")
	}
	parts := strings.Split(strings.TrimPrefix(raw, "at://"), "/")
	if len(parts) != 3 || parts[0] != did {
		return errors.New("publication must be an AT URI for the configured DID")
	}
	if parts[1] != publicationCollection || parts[2] == "" || strings.ContainsAny(parts[2], "/?#") {
		return errors.New("publication must use collection site.standard.publication and include an rkey")
	}
	return nil
}
