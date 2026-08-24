package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize PDG for an existing website",

	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		output, _ := cmd.Flags().GetString("output")

		cfg := config.Config{
			Site: config.SiteConfig{
				URL:       url,
				OutputDir: output,
			},
		}

		if err := config.WriteConfig(
			config.DefaultFilename,
			cfg,
		); err != nil {
			return err
		}

		fmt.Printf("PDG initialized successfully. Configuration written to %s\n", config.DefaultFilename)
		return nil
	},
}

func init() {
	initCmd.Flags().String(
		"url",
		"",
		"canonical website URL",
	)

	initCmd.Flags().String(
		"output",
		"./dist",
		"output directory for generated files",
	)

	initCmd.MarkFlagRequired("url")
}
