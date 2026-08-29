package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/sessionstore"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Inspect stored ATProto sessions",
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active ATProto session",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runAuthStatus(cmd.Context())
	},
}

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored ATProto sessions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runAuthList(cmd.Context())
	},
}

var authSwitchCmd = &cobra.Command{
	Use:   "switch <handle-or-did>",
	Short: "Switch the active ATProto account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthSwitch(cmd.Context(), args[0])
	},
}

func loadSessionStore() (*sessionstore.AuthStore, error) {
	store, err := sessionstore.DefaultAuthStore()
	if err != nil {
		return nil, fmt.Errorf("prepare session store: %w", err)
	}
	return store, nil
}

func runAuthStatus(ctx context.Context) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	session, err := store.LoadActiveSession(ctx)
	if err != nil {
		if errors.Is(err, sessionstore.ErrNoActiveAccount) {
			accounts, listErr := store.ListAccounts(ctx)
			if listErr == nil && len(accounts) > 0 {
				return fmt.Errorf("no active account is selected; %d account(s) are logged in, use `pdg auth switch <handle-or-did>`", len(accounts))
			}
			return fmt.Errorf("no account is logged in; use `pdg login <handle>` to authenticate")
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no account is logged in; use `pdg login <handle>` to authenticate")
		}
		if errors.Is(err, sessionstore.ErrCredentialsNotFound) {
			return fmt.Errorf("the active account has no secure credentials; run `pdg login <handle>` again")
		}
		return fmt.Errorf("load active session: %w", err)
	}
	fmt.Printf("Handle: %s\nDID: %s\nPDS: %s\nScope: %s\n", session.Identity.Handle, session.Identity.DID, session.Identity.PDS, session.Scope)
	return nil
}

func runAuthList(ctx context.Context) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	for _, account := range accounts {
		marker := " "
		if account.Active {
			marker = "*"
		}
		fmt.Printf("%s %s (%s)\n", marker, account.Handle, account.DID)
	}
	return nil
}

func runAuthSwitch(ctx context.Context, identifier string) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	for _, account := range accounts {
		if account.DID == identifier || account.Handle == identifier {
			if err := store.SetActive(ctx, account.DID); err != nil {
				return fmt.Errorf("switch active session: %w", err)
			}
			fmt.Printf("Active account: %s (%s)\n", account.Handle, account.DID)
			return nil
		}
	}
	return fmt.Errorf("account %q not found", identifier)
}

func init() {
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authSwitchCmd)
}
