package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout [handle-or-did]",
	Short: "Delete the active ATProto session",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogout(cmd.Context(), args)
	},
}

func runLogout(ctx context.Context, args []string) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	identifier, err := selectAccountIdentifier(ctx, store, args, "Select an ATProto account to log out:")
	if err != nil {
		return err
	}
	session, err := store.LoadSession(ctx, identifier)
	if err != nil {
		return fmt.Errorf("load account %q: %w", identifier, err)
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
