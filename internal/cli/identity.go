package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/protocol"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Resolver an ATProtocol identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		handle := args[0]
		return runIdentity(cmd.Context(), handle)
	},
}

func runIdentity(ctx context.Context, handle string) error {
	protocols := protocol.New(http.DefaultClient)

	identity, err := protocols.ATProto.ResolveIdentity(
		ctx,
		handle,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to resolve identity: %w", err,
		)
	}

	fmt.Printf("Handle: %s\nDID: %s\nPDS: %s\n", identity.Handle, identity.DID, identity.PDS)
	return nil
}
