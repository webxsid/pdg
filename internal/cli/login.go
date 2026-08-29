package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/protocol"
)

var loginCmd = &cobra.Command{
	Use:   "login <handle>",
	Short: "Authenticate an ATProto identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogin(cmd.Context(), args[0])
	},
}

func runLogin(ctx context.Context, handle string) error {
	fmt.Printf("Resolving %s...\n", handle)
	client := protocol.New(http.DefaultClient).ATProto

	openURL := func(rawURL string) error {
		fmt.Printf("Open this URL in your browser to authorize:\n%s\n", rawURL)
		fmt.Print("Press Enter after authorization completes: ")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			return fmt.Errorf("wait for authorization confirmation: %w", err)
		}
		return nil
	}

	fmt.Println("Waiting for authorization...")
	session, err := client.Login(ctx, handle, openURL)
	if err != nil {
		return err
	}

	fmt.Printf("Authenticated as %s (%s)\n", session.Identity.Handle, session.Identity.DID)
	fmt.Println("Session authentication completed in memory; credentials were not persisted.")
	return nil
}
