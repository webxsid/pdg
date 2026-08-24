package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

func (r *Resolver) resolveDID(
	ctx context.Context,
	did string,
) (didDocument, error) {
	switch {
	case strings.HasPrefix(did, "did:plc:"):
		return r.resolvePLCDID(ctx, did)
	case strings.HasPrefix(did, "did:web:"):
		return r.resolveWebDID(ctx, did)

	default:
		return didDocument{}, fmt.Errorf(
			"unsupported DID method: %s",
			did,
		)
	}
}

func (r *Resolver) resolvePLCDID(
	ctx context.Context,
	did string,
) (didDocument, error) {
	url := "https://plc.directory/" + did

	return r.resolveDIDDocument(ctx, url)
}

func (r *Resolver) resolveWebDID(
	ctx context.Context,
	did string,
) (didDocument, error) {
	host := strings.TrimPrefix(did, "did:web:")
	if host == "" {
		return didDocument{}, fmt.Errorf(
			"invalid DID format: %s",
			did,
		)
	}

	if strings.Contains(host, ":") {
		return didDocument{}, fmt.Errorf(
			"unsupported DID format with port: %s",
			did,
		)
	}

	url := fmt.Sprintf(
		"https://%s/.well-known/did.json",
		host,
	)

	return r.resolveDIDDocument(ctx, url)
}

func (r *Resolver) resolveDIDDocument(
	ctx context.Context,
	url string,
) (didDocument, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return didDocument{}, fmt.Errorf(
			"failed to create request: %w",
			err,
		)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return didDocument{}, fmt.Errorf(
			"failed to perform request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return didDocument{}, fmt.Errorf(
			"unexpected status code: %d",
			resp.StatusCode,
		)
	}

	var document didDocument
	decoder := json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	)

	if err := decoder.Decode(&document); err != nil {
		return didDocument{}, fmt.Errorf(
			"failed to decode DID document: %w",
			err,
		)
	}

	return document, nil
}

func verifyIdentity(
	handle string,
	did string,
	document didDocument,
) error {
	if document.ID != did {
		return fmt.Errorf(
			"DID document ID does not match DID: %s != %s",
			document.ID,
			did,
		)
	}

	expectedHandle := "at://" + handle

	if slices.Contains(document.AlsoKnownAs, expectedHandle) {
		return nil
	}

	return fmt.Errorf(
		"DID document does not contain expected handle: %s",
		expectedHandle,
	)
}

func isSupportedDID(did string) bool {
	if did != strings.TrimSpace(did) {
		return false
	}

	return strings.HasPrefix(did, "did:plc:") ||
		strings.HasPrefix(did, "did:web:")
}
