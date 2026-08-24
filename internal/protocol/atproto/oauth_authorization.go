package atproto

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"
)

type authorizationState struct {
	State         string
	CodeVerifier  string
	CodeChallenge string
}

type authorizationResult struct {
	Code         string
	CodeVerifier string
	RedirectURI  string
	ClientID     string
	DPoPNonce    string
	DPoPKey      *dpopKey
	Subject      string
}

type authorizationURLOpener func(string) error

func buildAuthorizationURL(
	server AuthorizationServer,
	clientID string,
	requestURI string,
) (string, error) {
	if server.AuthorizationEndpoint == "" {
		return "", fmt.Errorf(
			"authorization endpoint is empty",
		)
	}

	if clientID == "" {
		return "", fmt.Errorf(
			"client ID is empty",
		)
	}

	if requestURI == "" {
		return "", fmt.Errorf(
			"request URI is empty",
		)
	}

	parsed, err := url.Parse(
		server.AuthorizationEndpoint,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to parse authorization endpoint: %w",
			err,
		)
	}

	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("response_type", "code")
	query.Set("request_uri", requestURI)

	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func newAuthorizationState() (authorizationState, error) {
	state, err := randomBase64URL(32)
	if err != nil {
		return authorizationState{}, fmt.Errorf("failed to generate state: %w", err)
	}

	verifier, err := randomBase64URL(32)
	if err != nil {
		return authorizationState{}, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	challenge := pkceChallenge(verifier)

	return authorizationState{
		State:         state,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}, nil
}

func randomBase64URL(size int) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(
		hash[:],
	)
}

func (c *oauthClient) authorize(
	ctx context.Context,
	identity Identity,
	openURL authorizationURLOpener,
) (authorizationResult, error) {
	if openURL == nil {
		return authorizationResult{}, fmt.Errorf(
			"openURL is nil",
		)
	}

	server, err := c.DiscoverAuthorizationServer(
		ctx,
		identity.PDS,
	)
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"discover authorization server: %w",
			err,
		)
	}

	callbackServer, err := newCallbackServer()
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"create callback server: %w",
			err,
		)
	}

	callbackServer.start()

	defer func() {
		shiutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()
		_ = callbackServer.close(shiutdownCtx)
	}()

	redirectURI := callbackServer.redirectURI()

	const scope = "atproto"

	clientID := localhostClientID(
		redirectURI,
		scope,
	)

	state, err := newAuthorizationState()
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"create authorization state: %w",
			err,
		)
	}
	key, err := newDPoPKey()
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"create DPoP key: %w", err,
		)
	}

	par, err := c.pushAuthorizationRequest(
		ctx,
		server,
		parRequest{
			ClientID:      clientID,
			RedirectURI:   redirectURI,
			Scope:         scope,
			LoginHint:     identity.DID,
			State:         state.State,
			CodeChallenge: state.CodeChallenge,
		},
		key,
	)
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"push authorization request: %w",
			err,
		)
	}

	authorizationURL, err := buildAuthorizationURL(
		server,
		clientID,
		par.RequestURI,
	)
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"build authorization URL: %w",
			err,
		)
	}

	if err := openURL(authorizationURL); err != nil {
		return authorizationResult{}, fmt.Errorf(
			"open authorization URL: %w",
			err,
		)
	}

	callback, err := callbackServer.wait(ctx)
	if err != nil {
		return authorizationResult{}, fmt.Errorf(
			"wait for callback: %w",
			err,
		)
	}

	if callback.State != state.State {
		return authorizationResult{}, fmt.Errorf(
			"state mismatch: got %q, want %q",
			callback.State,
			state.State,
		)
	}

	return authorizationResult{
		Code:         callback.Code,
		CodeVerifier: state.CodeVerifier,
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		DPoPNonce:    par.DPoPNonce,
		DPoPKey:      key,
		Subject:      identity.DID,
	}, nil
}

func localhostClientID(
	redirectURI string,
	scope string,
) string {
	values := url.Values{}

	values.Set("redirect_uri", redirectURI)
	values.Set("scope", scope)

	return "http://localhost?" + values.Encode()
}
