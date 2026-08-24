package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/webxsid/pdg/internal/shared"
)

type oauthClient struct {
	client *http.Client
}

type protectedResourceMetadata struct {
	AuthorizationServers []string `json:"authorization_servers"`
}

type AuthorizationServer struct {
	Issuer                             string
	AuthorizationEndpoint              string
	TokenEndpoint                      string
	PushedAuthorizationRequestEndpoint string
	DpoPSigningAlgorithms              []string
}

type authorizationServerMetadata struct {
	Issuer                             string   `json:"issuer"`
	AuthorizationEndpoint              string   `json:"authorization_endpoint"`
	TokenEndpoint                      string   `json:"token_endpoint"`
	PushedAuthorizationRequestEndpoint string   `json:"pushed_authorization_request_endpoint"`
	CodeChallengeMethodsSupported      []string `json:"code_challenge_methods_supported"`
	DPoPSigningAlgValuesSupported      []string `json:"dpop_signing_alg_values_supported"`
}

func (c *oauthClient) DiscoverAuthorizationServer(
	ctx context.Context,
	pds string,
) (AuthorizationServer, error) {
	issuer, err := c.discoverIssuer(ctx, pds)
	if err != nil {
		return AuthorizationServer{}, fmt.Errorf(
			"failed to discover issuer: %w",
			err,
		)
	}

	return c.fetchAuthorizationServerMetadata(ctx, issuer)
}

func (c *oauthClient) fetchAuthorizationServerMetadata(
	ctx context.Context,
	issuer string,
) (AuthorizationServer, error) {
	endpoint :=
		issuer + "/.well-known/oauth-authorization-server"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return AuthorizationServer{}, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return AuthorizationServer{}, fmt.Errorf(
			"fetch authorization server metadata: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AuthorizationServer{}, fmt.Errorf(
			"unexpected authorization server metadata status: %s",
			resp.Status,
		)
	}

	var metadata authorizationServerMetadata

	decoder := json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	)

	if err := decoder.Decode(&metadata); err != nil {
		return AuthorizationServer{}, fmt.Errorf(
			"decode authorization server metadata: %w",
			err,
		)
	}

	if err := validateAuthorizationServerMetadata(
		issuer,
		metadata,
	); err != nil {
		return AuthorizationServer{}, fmt.Errorf(
			"validate authorization server metadata: %w",
			err,
		)
	}

	return AuthorizationServer{
		Issuer: issuer,

		AuthorizationEndpoint: metadata.AuthorizationEndpoint,

		TokenEndpoint: metadata.TokenEndpoint,

		PushedAuthorizationRequestEndpoint: metadata.PushedAuthorizationRequestEndpoint,

		DpoPSigningAlgorithms: metadata.DPoPSigningAlgValuesSupported,
	}, nil
}

func (c *oauthClient) discoverIssuer(
	ctx context.Context,
	pds string,
) (string, error) {
	endpoint := pds + "/.well-known/oauth-protected-resource"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"failed to perform request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"unexpected status code: %d",
			resp.StatusCode,
		)
	}

	var metadata protectedResourceMetadata

	decoder := json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	)

	if err := decoder.Decode(&metadata); err != nil {
		return "", fmt.Errorf(
			"failed to decode protected resource metadata: %w",
			err,
		)
	}

	if len(metadata.AuthorizationServers) == 0 {
		return "", fmt.Errorf(
			"no authorization servers found in metadata",
		)
	}

	if len(metadata.AuthorizationServers) > 1 {
		return "", fmt.Errorf(
			"expected exactly one authorization server, but found %d",
			len(metadata.AuthorizationServers),
		)
	}

	issuer := metadata.AuthorizationServers[0]

	if err := validateAuthorizationServer(issuer); err != nil {
		return "", fmt.Errorf(
			"invalid authorization server: %w",
			err,
		)
	}

	return issuer, nil
}

func validateAuthorizationServer(issuer string) error {
	parsed, err := url.Parse(issuer)
	if err != nil {
		return fmt.Errorf(
			"failed to parse authorization server URL: %w",
			err,
		)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf(
			"authorization server must use https scheme: %s",
			issuer,
		)
	}
	if parsed.Host == "" {
		return fmt.Errorf(
			"authorization server must have a valid host: %s",
			issuer,
		)
	}
	if parsed.User != nil {
		return fmt.Errorf(
			"authorization server must not contain user info: %s",
			issuer,
		)
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf(
			"authorization server must not contain query parameters: %s",
			issuer,
		)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf(
			"authorization server must not contain fragment: %s",
			issuer,
		)
	}
	if parsed.Port() == "443" {
		return fmt.Errorf(
			"default HTTPS port 443 should not be specified in authorization server URL: %s",
			issuer,
		)
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf(
			"authorization server must not contain path: %s",
			issuer,
		)
	}

	return nil
}

func validateAuthorizationServerMetadata(
	expectedIssuer string,
	metadata authorizationServerMetadata,
) error {
	if metadata.Issuer != expectedIssuer {
		return fmt.Errorf(
			"issuer mismatch: got %q, want %q",
			metadata.Issuer,
			expectedIssuer,
		)
	}

	if err := validateAuthorizationEndpoint(
		metadata.AuthorizationEndpoint,
	); err != nil {
		return fmt.Errorf(
			"invalid authorization endpoint: %w",
			err,
		)
	}

	if err := validateTokenEndpoint(
		metadata.TokenEndpoint,
	); err != nil {
		return fmt.Errorf(
			"invalid token endpoint: %w",
			err,
		)
	}

	if err := validatePAREndpoint(
		metadata.PushedAuthorizationRequestEndpoint,
	); err != nil {
		return fmt.Errorf(
			"invalid PAR endpoint: %w",
			err,
		)
	}

	if !slices.Contains(
		metadata.CodeChallengeMethodsSupported,
		"S256",
	) {
		return fmt.Errorf(
			"authorization server does not support S256 PKCE",
		)
	}

	if len(metadata.DPoPSigningAlgValuesSupported) == 0 {
		return fmt.Errorf(
			"authorization server does not advertise DPoP signing algorithms",
		)
	}

	return nil
}

func validateAuthorizationEndpoint(endpoint string) error {
	return shared.ValidateHTTPSEndpoint(endpoint)
}

func validateTokenEndpoint(endpoint string) error {
	return shared.ValidateHTTPSEndpoint(endpoint)
}

func validatePAREndpoint(endpoint string) error {
	return shared.ValidateHTTPSEndpoint(endpoint)
}
