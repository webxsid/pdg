package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/integration"
	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/webxsid/pdg/internal/sessionstore"
	"github.com/webxsid/pdg/internal/state"
)

var addCmd = &cobra.Command{Use: "add [integration]", Short: "Add a project integration", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return runAdd(cmd.Context(), args) }}
var removeCmd = &cobra.Command{Use: "remove <integration...>", Short: "Remove project integrations", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return runRemove(args) }}

func runAdd(ctx context.Context, args []string) error {
	if len(args) == 0 {
		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "Select an integration to add:",
			Options: []string{"standard-site", "bluesky"},
			Default: "standard-site",
		}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return fmt.Errorf("select integrations: %w", err)
		}
		args = []string{selected}
	}
	definitions, err := integration.Parse(args)
	if err != nil {
		return err
	}
	store, err := sessionstore.DefaultAuthStore()
	if err != nil {
		return err
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list ATProto accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no authenticated ATProto identities found; run `pdg atproto login <handle>` first")
	}
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	projectState, err := state.Load(filepath.Join(".pdg", "state.json"))
	if err != nil {
		return fmt.Errorf("load project state: %w", err)
	}
	fmt.Println("Preparing project integrations...")
	client := atproto.NewClient(http.DefaultClient)
	xrpc := atproto.NewXRPCClient(http.DefaultClient, store)
	identities := map[integration.Integration]string{}
	did, err := chooseIdentity(accounts, string(definitions[0].Name))
	if err != nil {
		return err
	}
	identities[definitions[0].Name] = did
	groups := map[string][]atproto.Capability{}
	for _, definition := range definitions {
		groups[identities[definition.Name]] = append(groups[identities[definition.Name]], definition.Capabilities...)
	}
	sessions := map[string]atproto.Session{}
	for did, capabilities := range groups {
		scope := strings.Join(atproto.ScopesForCapabilities(capabilities...), " ")
		session, loadErr := store.LoadSession(ctx, did)
		if loadErr != nil || !hasScopes(session.Scope, scope) {
			fmt.Printf("Authorizing %s...\n", did)
			account := findAccount(accounts, did)
			if account == nil {
				return fmt.Errorf("account %s is unavailable", did)
			}
			session, err = authorizeAndSave(ctx, client, store, account.Handle, scope)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Authorization completed for %s\n", did)
		} else {
			fmt.Printf("✓ Existing authorization is sufficient for %s\n", did)
		}
		sessions[did] = session
	}
	for _, definition := range definitions {
		name, did := definition.Name, identities[definition.Name]
		session := sessions[did]
		if name == integration.StandardSite {
			if cfg.Integrations.StandardSite.Enabled && cfg.Integrations.StandardSite.Publication != "" {
				if existing, ok := projectState.Integrations["standard-site"]; ok && existing.Publication != nil && existing.Publication.URI != cfg.Integrations.StandardSite.Publication && existing.Publication.URI != "" {
					return fmt.Errorf("Standard.site publication conflicts with .pdg state")
				}
				projectState.Integrations["standard-site"] = state.IntegrationState{Publication: &state.PublicationState{URI: cfg.Integrations.StandardSite.Publication}}
				continue
			}
			standard := cfg.Integrations.StandardSite
			if standard.PublicDir == "" {
				if err := survey.AskOne(&survey.Input{Message: "Public directory:", Default: "public"}, &standard.PublicDir, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
					return err
				}
			}
			if len(standard.Paths) == 0 {
				standard.Paths, err = promptPublicationPaths("Standard.site")
				if err != nil {
					return err
				}
			}
			uri, err := xrpc.CreateRecord(ctx, session, "site.standard.publication", map[string]any{"$type": "site.standard.publication", "url": cfg.Site.URL, "name": integrationSiteName(cfg.Site.URL)})
			if err != nil {
				return fmt.Errorf("create Standard.site publication: %w", err)
			}
			fmt.Printf("✓ Standard.site publication registered: %s\n", uri)
			if err := injectWellKnown(standard.PublicDir, uri); err != nil {
				return fmt.Errorf("inject Standard.site verification file: %w", err)
			}
			fmt.Printf("✓ Wrote Standard.site verification file in %s\n", standard.PublicDir)
			standard.Enabled, standard.Identity, standard.Publication = true, did, uri
			cfg.Integrations.StandardSite = standard
			projectState.Integrations["standard-site"] = state.IntegrationState{Publication: &state.PublicationState{URI: uri}}
		} else {
			bluesky := cfg.Integrations.Bluesky
			if len(bluesky.Paths) == 0 {
				bluesky.Paths, err = promptPublicationPaths("Bluesky")
				if err != nil {
					return err
				}
			}
			bluesky.Enabled, bluesky.Identity = true, did
			cfg.Integrations.Bluesky = bluesky
		}
	}
	if err := config.WriteConfig(config.DefaultFilename, *cfg); err != nil {
		return fmt.Errorf("save project integrations: %w", err)
	}
	if err := state.Write(filepath.Join(".pdg", "state.json"), projectState); err != nil {
		return fmt.Errorf("save project state: %w", err)
	}
	fmt.Printf("✓ Project configuration written to %s\n", config.DefaultFilename)
	fmt.Printf("Added %d integration(s).\n", len(definitions))
	return nil
}

func promptPublicationPaths(integrationName string) ([]config.PublicationPath, error) {
	var result []config.PublicationPath
	for {
		var input string
		if err := survey.AskOne(&survey.Input{Message: "Paths for " + integrationName + " (comma-separated):", Default: "/"}, &input, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return nil, err
		}
		paths, err := parsePublicationPathInput(input)
		if err != nil {
			return nil, err
		}
		var selected string
		if err := survey.AskOne(&survey.Select{Message: "Publish:", Options: []string{"All pages", "Explicit pages only"}, Default: "All pages"}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return nil, err
		}
		mode := config.PublicationAll
		if selected == "Explicit pages only" {
			mode = config.PublicationExplicit
		}
		for _, value := range paths {
			result = append(result, config.PublicationPath{Path: value, Publish: mode})
		}
		var more bool
		if err := survey.AskOne(&survey.Confirm{Message: "Add paths with a different publication setting?", Default: false}, &more, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return nil, err
		}
		if !more {
			return config.NormalizePublicationPaths(result)
		}
	}
}

func parsePublicationPathInput(input string) ([]string, error) {
	parts := strings.Split(input, ",")
	paths := make([]config.PublicationPath, 0, len(parts))
	for _, value := range parts {
		paths = append(paths, config.PublicationPath{Path: value, Publish: config.PublicationAll})
	}
	normalized, err := config.NormalizePublicationPaths(paths)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(normalized))
	for i, value := range normalized {
		result[i] = value.Path
	}
	return result, nil
}

func injectWellKnown(publicDir, publication string) error {
	// A leading slash is the user-facing token for the project/public root,
	// not the host filesystem root.
	if publicDir == "/" {
		publicDir = "."
	}
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	target, err := filepath.Abs(publicDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("public directory %q is outside the project root", publicDir)
	}
	wellKnown := filepath.Join(target, ".well-known", "site.standard.publication")
	if err := os.MkdirAll(filepath.Dir(wellKnown), 0o755); err != nil {
		return err
	}
	contents := []byte(publication + "\n")
	if existing, err := os.ReadFile(wellKnown); err == nil {
		if string(existing) != string(contents) {
			return fmt.Errorf("%s already exists with different contents", wellKnown)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(wellKnown), ".publication-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(contents); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, wellKnown)
}

func runRemove(args []string) error {
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range args {
		if _, err := integration.Resolve(name); err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("integration %q was specified more than once", name)
		}
		seen[name] = true
	}
	for name := range seen {
		if name == "standard-site" {
			cfg.Integrations.StandardSite = config.StandardSiteIntegration{}
		} else {
			cfg.Integrations.Bluesky = config.BlueskyIntegration{}
		}
	}
	return config.WriteConfig(config.DefaultFilename, *cfg)
}

func chooseIdentity(accounts []sessionstore.AccountSummary, integration string) (string, error) {
	options := make([]string, len(accounts))
	for i, account := range accounts {
		options[i] = fmt.Sprintf("%s (%s)", account.Handle, account.DID)
	}
	selected := ""
	if err := survey.AskOne(&survey.Select{Message: "Select ATProto identity for " + integration + ":", Options: options}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
		return "", err
	}
	for i, option := range options {
		if option == selected {
			return accounts[i].DID, nil
		}
	}
	return "", fmt.Errorf("selected identity is unavailable")
}

func authorizeAndSave(ctx context.Context, client *atproto.Client, store *sessionstore.AuthStore, handle, scope string) (atproto.Session, error) {
	opener := func(rawURL string) error {
		fmt.Printf("Open this URL in your browser to authorize:\n%s\n", rawURL)
		fmt.Print("Press Enter after authorization completes: ")
		_, err := bufio.NewReader(os.Stdin).ReadString('\n')
		return err
	}
	session, err := client.LoginWithScope(ctx, handle, scope, opener)
	if err != nil {
		return atproto.Session{}, fmt.Errorf("authorize %s: %w", handle, err)
	}
	if err := store.SaveSession(ctx, session, true); err != nil {
		return atproto.Session{}, fmt.Errorf("save authorization: %w", err)
	}
	return session, nil
}

func hasScopes(granted, required string) bool {
	have := map[string]bool{}
	for _, value := range strings.Fields(granted) {
		have[value] = true
	}
	for _, value := range strings.Fields(required) {
		if !have[value] {
			return false
		}
	}
	return true
}
func findAccount(accounts []sessionstore.AccountSummary, did string) *sessionstore.AccountSummary {
	for i := range accounts {
		if accounts[i].DID == did {
			return &accounts[i]
		}
	}
	return nil
}
func integrationSiteName(raw string) string { parsed, _ := url.Parse(raw); return parsed.Hostname() }
func init()                                 { rootCmd.AddCommand(addCmd); rootCmd.AddCommand(removeCmd) }
