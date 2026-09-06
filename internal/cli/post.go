package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/post"
	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/sessionstore"
	"github.com/webxsid/pdg/internal/state"
)

var postContent string
var postDryRun bool

var postCmd = &cobra.Command{
	Use: "post <path>", Short: "Create an explicit Bluesky post for a published document", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { return runPost(cmd.Context(), args[0]) },
}

func runPost(ctx context.Context, rawPath string) error {
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if !cfg.Integrations.Bluesky.Enabled || cfg.Integrations.Bluesky.Identity == "" {
		return errors.New("Bluesky is not configured; run `pdg add bluesky` first")
	}
	project, err := state.Load(filepath.Join(".pdg", "state.json"))
	if err != nil {
		return fmt.Errorf("load project state: %w", err)
	}
	path := state.NormalizePath(rawPath)
	document := project.Documents[path]
	if document == nil {
		return fmt.Errorf("document %s was not found in .pdg/state.json; run `pdg scan`", path)
	}
	if document.Targets[state.TargetStandardSite].URI == "" {
		return fmt.Errorf("%s has not been published to Standard.site; run `pdg publish standard-site`", path)
	}
	if err := validatePostDocumentURI(document.Targets[state.TargetStandardSite].URI, cfg.Integrations.StandardSite.Identity); err != nil {
		return err
	}
	preview, err := post.BuildPreview(document, postContent)
	if err != nil {
		return err
	}
	fmt.Println("Bluesky post preview:")
	fmt.Println(preview.Text)
	fmt.Printf("Standard.site document: %s\n", preview.StandardSiteURI)
	if postDryRun {
		fmt.Println("No post created.")
		return nil
	}
	store, err := sessionstore.DefaultAuthStore()
	if err != nil {
		return fmt.Errorf("initialize credential store: %w", err)
	}
	session, err := store.LoadSession(ctx, cfg.Integrations.Bluesky.Identity)
	if err != nil {
		return fmt.Errorf("load Bluesky credentials for %s: %w", cfg.Integrations.Bluesky.Identity, err)
	}
	requiredScope := strings.Join(atproto.ScopesForCapabilities(atproto.CapabilityBlueskyPosting), " ")
	if !hasScopes(session.Scope, requiredScope) {
		return fmt.Errorf("Bluesky authorization for %s is missing the feed-posting scope; run `pdg add bluesky` to authorize the required capability", cfg.Integrations.Bluesky.Identity)
	}
	client := atproto.NewXRPCClient(http.DefaultClient, store)
	_, err = post.Create(ctx, document, postContent, session, client, func(value state.PostState) error {
		document.Posts["bluesky"] = value
		project.Documents[path] = document
		return state.Write(filepath.Join(".pdg", "state.json"), project)
	})
	if err != nil {
		return err
	}
	fmt.Printf("✓ Posted to Bluesky\n")
	return nil
}

func validatePostDocumentURI(raw, did string) error {
	parts := strings.Split(strings.TrimPrefix(raw, "at://"), "/")
	if !strings.HasPrefix(raw, "at://") || len(parts) != 3 || parts[0] != did || parts[1] != "site.standard.document" || parts[2] == "" {
		return fmt.Errorf("invalid Standard.site document URI for configured identity")
	}
	return nil
}

func init() {
	postCmd.Flags().StringVar(&postContent, "content", "", "override Bluesky post copy")
	postCmd.Flags().BoolVar(&postDryRun, "dry-run", false, "preview without creating a post")
	rootCmd.AddCommand(postCmd)
}
