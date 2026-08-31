package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/state"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a PDG project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		siteURL, _ := cmd.Flags().GetString("url")
		if siteURL == "" {
			if err := survey.AskOne(&survey.Input{Message: "Site URL:", Help: "The canonical URL of this site"}, &siteURL, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
				return fmt.Errorf("prompt for site URL: %w", err)
			}
		}
		if siteURL == "" {
			return fmt.Errorf("site URL cannot be empty")
		}
		cfg := config.Config{Site: config.SiteConfig{URL: siteURL}}
		if err := config.WriteConfig(config.DefaultFilename, cfg); err != nil {
			return err
		}
		statePath := filepath.Join(".pdg", "state.json")
		if _, err := os.Stat(statePath); os.IsNotExist(err) {
			if err := state.Write(statePath, state.New()); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("inspect project state: %w", err)
		} else if _, err := state.Load(statePath); err != nil {
			return err
		}
		fmt.Printf("PDG initialized. Configuration written to %s\n", config.DefaultFilename)
		return nil
	},
}

func init() {
	initCmd.Flags().String("url", "", "canonical website URL")
}
