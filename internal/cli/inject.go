package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/inject"
	"github.com/webxsid/pdg/internal/state"
)

var injectCmd = &cobra.Command{
	Use: "inject [integration]", Short: "Materialize integration artifacts into generated site output", Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		return runInject(name)
	},
}

func runInject(name string) error {
	if name != "" && name != "standard-site" {
		if name == "bluesky" {
			return fmt.Errorf("Bluesky does not currently expose injectable site artifacts")
		}
		return fmt.Errorf("unknown inject integration %q", name)
	}
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if !cfg.Integrations.StandardSite.Enabled {
		return fmt.Errorf("Standard.site is not configured for this project; run `pdg add standard-site`")
	}
	project, err := state.Load(filepath.Join(".pdg", "state.json"))
	if err != nil {
		return fmt.Errorf("load project state: %w", err)
	}
	root := cfg.Integrations.StandardSite.PublicDir
	if root == "" {
		root = "."
	}
	summary := inject.StandardSite(".", root, *cfg, project)
	for _, result := range summary.Results {
		fmt.Printf("  %s %s\n", result.Path, result.Action)
	}
	for _, injectErr := range summary.Errors {
		fmt.Printf("  ! %s\n", injectErr)
	}
	if len(summary.Errors) > 0 {
		return fmt.Errorf("Standard.site injection completed with %d error(s)", len(summary.Errors))
	}
	fmt.Printf("Standard.site artifacts ready (%d item(s))\n", len(summary.Results))
	return nil
}

func init() { rootCmd.AddCommand(injectCmd) }
