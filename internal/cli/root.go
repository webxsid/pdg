package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pdg",
	Short: "Connect independent websites to the open social web",
}

func init() {
	rootCmd.AddCommand(scnCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(identityCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(logoutCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
