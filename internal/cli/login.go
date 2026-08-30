package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/protocol"
	"github.com/webxsid/pdg/internal/sessionstore"
)

var loginCmd = &cobra.Command{
	Use:   "login [handle]",
	Short: "Authenticate an ATProto identity",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		handle := ""
		if len(args) == 1 {
			handle = args[0]
		} else if err := survey.AskOne(&survey.Input{Message: "ATProto handle:"}, &handle, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return fmt.Errorf("read ATProto handle: %w", err)
		}
		if handle == "" {
			return fmt.Errorf("ATProto handle cannot be empty")
		}
		return runLogin(cmd.Context(), handle)
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
	store, err := sessionstore.DefaultAuthStore()
	if err != nil {
		return fmt.Errorf("prepare session store: %w", err)
	}
	if err := store.SaveSession(ctx, session, true); err != nil {
		return fmt.Errorf("save ATProto session: %w", err)
	}

	fmt.Printf("Authenticated as %s (%s)\n", session.Identity.Handle, session.Identity.DID)
	fmt.Println("Session saved.")
	return nil
}
