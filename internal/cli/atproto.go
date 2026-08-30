package cli

import "github.com/spf13/cobra"

var atprotoCmd = &cobra.Command{
	Use:   "atproto",
	Short: "Manage ATProto integration",
}

func init() {
	atprotoCmd.AddCommand(identityCmd)
	atprotoCmd.AddCommand(loginCmd)
	atprotoCmd.AddCommand(authCmd)
	atprotoCmd.AddCommand(logoutCmd)
}
