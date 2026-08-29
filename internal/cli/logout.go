package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/sessionstore"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete the active ATProto session",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runLogout(cmd.Context())
	},
}

func runLogout(ctx context.Context) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	session, err := store.LoadActiveSession(ctx)
	if err != nil {
		if errors.Is(err, sessionstore.ErrNoActiveAccount) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no active account to log out")
		}
		return fmt.Errorf("load active session: %w", err)
	}
	if err := store.DeleteSession(ctx, session.Identity.DID); err != nil {
		return fmt.Errorf("delete active session: %w", err)
	}
	fmt.Printf("Logged out %s.\n", session.Identity.Handle)
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list remaining accounts: %w", err)
	}
	for _, account := range accounts {
		if account.Active {
			fmt.Printf("Active account is now %s (%s).\n", account.Handle, account.DID)
			return nil
		}
	}
	fmt.Println("No active account remains.")
	return nil
}
