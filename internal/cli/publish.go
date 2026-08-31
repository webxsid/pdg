package cli

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/publish"
	"github.com/webxsid/pdg/internal/sessionstore"
	"github.com/webxsid/pdg/internal/state"
)

var publishCmd = &cobra.Command{
	Use:   "publish [integration]",
	Short: "Publish scanned documents to configured integrations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		integration := ""
		if len(args) == 1 {
			integration = args[0]
		}
		return runPublish(cmd.Context(), integration)
	},
}

func runPublish(ctx context.Context, requested string) error {
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	project, err := state.Load(filepath.Join(".pdg", "state.json"))
	if err != nil {
		return fmt.Errorf("load project state: %w", err)
	}
	if len(project.Documents) == 0 {
		return fmt.Errorf("no scanned documents found; run `pdg scan` first")
	}
	if requested == "bluesky" {
		return fmt.Errorf("Bluesky publishing is not implemented yet")
	}
	if requested != "" && requested != "standard-site" {
		return fmt.Errorf("unknown publish integration %q", requested)
	}
	if requested == "" && !cfg.Integrations.StandardSite.Enabled {
		if cfg.Integrations.Bluesky.Enabled {
			return fmt.Errorf("Bluesky publishing is not implemented yet")
		}
		return fmt.Errorf("no configured publishing integration has an implementation")
	}
	if !cfg.Integrations.StandardSite.Enabled {
		return fmt.Errorf("Standard.site is not configured for this project; run `pdg add standard-site`")
	}
	store, err := sessionstore.DefaultAuthStore()
	if err != nil {
		return fmt.Errorf("initialize ATProto session store: %w", err)
	}
	session, err := store.LoadSession(ctx, cfg.Integrations.StandardSite.Identity)
	if err != nil {
		return fmt.Errorf("load Standard.site credentials for %s: %w", cfg.Integrations.StandardSite.Identity, err)
	}
	xrpc := newPublishRecordClient(http.DefaultClient, store)
	summary := publish.StandardSite(ctx, &project, session, xrpc, func(updated state.ProjectState) error {
		return state.Write(filepath.Join(".pdg", "state.json"), updated)
	})
	for _, result := range summary.Results {
		fmt.Printf("  %s %s\n", result.Path, result.Action)
	}
	for _, publishErr := range summary.Errors {
		fmt.Printf("  ! %s\n", publishErr)
	}
	if len(summary.Errors) > 0 {
		return fmt.Errorf("Standard.site publication completed with %d error(s)", len(summary.Errors))
	}
	fmt.Printf("Standard.site publication complete (%d document(s))\n", len(summary.Results))
	return nil
}

func newPublishRecordClient(httpClient *http.Client, store *sessionstore.AuthStore) *atproto.XRPCClient {
	return atproto.NewXRPCClient(httpClient, store)
}
