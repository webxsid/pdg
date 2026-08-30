package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/protocol"
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
	Use:   "switch [handle-or-did]",
	Short: "Switch the active ATProto account",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthSwitch(cmd.Context(), args)
	},
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh [handle-or-did]",
	Short: "Refresh the active ATProto session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runAuthRefresh(cmd.Context(), args) },
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
				return fmt.Errorf("no active account is selected; %d account(s) are logged in, use `pdg atproto auth switch <handle-or-did>`", len(accounts))
			}
			return fmt.Errorf("no account is logged in; use `pdg atproto login <handle>` to authenticate")
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no account is logged in; use `pdg atproto login <handle>` to authenticate")
		}
		if errors.Is(err, sessionstore.ErrCredentialsNotFound) {
			return fmt.Errorf("the active account has no secure credentials; run `pdg atproto login <handle>` again")
		}
		return fmt.Errorf("load active session: %w", err)
	}
	fmt.Printf("Handle: %s\nDID: %s\nPDS: %s\nScope: %s\n", session.Identity.Handle, session.Identity.DID, session.Identity.PDS, session.Scope)
	fmt.Println("Status: authenticated")
	if session.AccessTokenExpiresAt == nil {
		fmt.Println("Access token expiry: unknown")
	} else if !session.AccessTokenExpiresAt.After(time.Now()) {
		fmt.Println("Status: access token expired\n\nRefresh the session with:\n  pdg atproto auth refresh")
	} else {
		fmt.Printf("Access token expires: %s\n", session.AccessTokenExpiresAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func runAuthRefresh(ctx context.Context, args []string) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	identifier, err := selectAccountIdentifier(ctx, store, args, "Select an ATProto account to refresh:")
	if err != nil {
		return err
	}
	current, err := store.LoadSession(ctx, identifier)
	if err != nil {
		return fmt.Errorf("load account %q: %w", identifier, err)
	}
	if current.RefreshToken == "" {
		return fmt.Errorf("no refresh token is available for %s; run `pdg atproto login %s` to authenticate again", current.Identity.Handle, current.Identity.Handle)
	}
	client := protocol.New(http.DefaultClient).ATProto
	fmt.Printf("Refreshing ATProto session for %s...\n", current.Identity.Handle)
	updated, err := store.RefreshSession(ctx, client, current.Identity.DID)
	if err != nil {
		return fmt.Errorf("refresh ATProto session: %w", err)
	}
	fmt.Println("Session refreshed successfully.")
	if updated.AccessTokenExpiresAt != nil {
		fmt.Printf("Access token expires: %s\n", updated.AccessTokenExpiresAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func selectAccountIdentifier(ctx context.Context, store *sessionstore.AuthStore, args []string, message string) (string, error) {
	if len(args) == 1 {
		accounts, err := store.ListAccounts(ctx)
		if err != nil {
			return "", fmt.Errorf("list accounts: %w", err)
		}
		for _, account := range accounts {
			if account.DID == args[0] || account.Handle == args[0] {
				return account.DID, nil
			}
		}
		return "", fmt.Errorf("account %q not found", args[0])
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		return "", fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no account is logged in; use `pdg atproto login <handle>` to authenticate")
	}
	options := make([]string, len(accounts))
	for i, account := range accounts {
		options[i] = fmt.Sprintf("%s (%s)", account.Handle, account.DID)
	}
	selected := ""
	err = survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
		Default: 0,
	}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr))
	if err != nil {
		return "", fmt.Errorf("select account: %w", err)
	}
	for i, option := range options {
		if option == selected {
			return accounts[i].DID, nil
		}
	}
	return "", fmt.Errorf("selected account is no longer available")
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

func runAuthSwitch(ctx context.Context, args []string) error {
	store, err := loadSessionStore()
	if err != nil {
		return err
	}
	identifier, err := selectAccountIdentifier(ctx, store, args, "Select an ATProto account to make active:")
	if err != nil {
		return err
	}
	if err := store.SetActive(ctx, identifier); err != nil {
		return fmt.Errorf("switch active session: %w", err)
	}
	session, err := store.LoadSession(ctx, identifier)
	if err != nil {
		return fmt.Errorf("load switched session: %w", err)
	}
	fmt.Printf("Active account: %s (%s)\n", session.Identity.Handle, session.Identity.DID)
	return nil
}

func init() {
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authSwitchCmd)
	authCmd.AddCommand(authRefreshCmd)
}
