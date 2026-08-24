package atproto

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var handlePattern = regexp.MustCompile(

	`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+` +
		`[a-zA-Z]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`,
)

func normalizeHandle(handle string) string {
	handle = strings.TrimSpace(handle)
	handle = strings.TrimPrefix(handle, "@")

	return strings.ToLower(handle)
}

func validateHandle(handle string) error {
	if len(handle) < 1 || len(handle) > 255 {
		return fmt.Errorf("handle must be between 1 and 255 characters")
	}

	if !handlePattern.MatchString(handle) {
		return fmt.Errorf("invalid handle format")
	}

	return nil
}

func (r *Resolver) resolveHandle(
	ctx context.Context,
	handle string,
) (string, error) {
	did, err := r.resolveHandleDNS(
		ctx,
		handle,
	)
	if err == nil {
		return did, nil
	}

	did, err = r.resolveHandleHTTPS(
		ctx,
		handle,
	)
	if err == nil {
		return did, nil
	}

	return "", fmt.Errorf(
		"failed to resolve handle via HTTPS and DNS: %w",
		err,
	)
}

func (r *Resolver) resolveHandleHTTPS(
	ctx context.Context,
	handle string,
) (string, error) {
	url := fmt.Sprintf(
		"https://%s/.well-known/atproto-did",
		handle,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(
		io.LimitReader(resp.Body, 2048),
	)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	did := strings.TrimSpace(string(body))
	if !strings.HasPrefix(did, "did:") {
		return "", fmt.Errorf("invalid DID format: %s", did)
	}

	return did, nil
}

func (r *Resolver) resolveHandleDNS(
	ctx context.Context,
	handle string,
) (string, error) {
	hostname := "_atproto." + handle

	records, err := r.dnsResolver.LookupTXT(
		ctx,
		hostname,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to lookup TXT records for %s: %w",
			hostname,
			err,
		)
	}

	return parseHandleDNSRecords(records)
}

func parseHandleDNSRecords(records []string) (string, error) {
	var did string

	for _, record := range records {

		value, found := strings.CutPrefix(record, "did=")
		if !found {
			continue
		}

		if value == "" {
			continue
		}

		if !isSupportedDID(value) {
			continue
		}

		if did != "" {
			return "", fmt.Errorf(
				"multiple DID records found in DNS TXT records",
			)
		}

		did = value
	}

	if did == "" {
		return "", fmt.Errorf(
			"DID not found in DNS TXT records",
		)
	}

	return did, nil
}
