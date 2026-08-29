package atproto

import (
	"context"
	"fmt"
	"net/http"
)

type Client struct {
	identity *Resolver
	oauth    *oauthClient
}

func NewClient(
	httpClient *http.Client,
) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		identity: newResolver(httpClient),
		oauth:    &oauthClient{client: httpClient},
	}
}

func (c *Client) ResolveIdentity(
	ctx context.Context,
	handle string,
) (Identity, error) {
	return c.identity.resolve(ctx, handle)
}

func (c *Client) DiscoverAuthorizationServer(
	ctx context.Context,
	identity Identity,
) (AuthorizationServer, error) {
	return c.oauth.DiscoverAuthorizationServer(
		ctx, identity.PDS,
	)
}

// Session contains the in-memory credentials and protocol metadata from an
// authenticated ATProto login.
type Session struct {
	Identity     Identity
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	DPoPNonce    string

	dpopKey *dpopKey
	server  AuthorizationServer
}

// Login resolves a handle, completes OAuth authorization, and returns an
// in-memory authenticated session.
func (c *Client) Login(ctx context.Context, handle string, openURL authorizationURLOpener) (Session, error) {
	identity, err := c.ResolveIdentity(ctx, handle)
	if err != nil {
		return Session{}, fmt.Errorf("resolve ATProto identity: %w", err)
	}

	authorization, err := c.oauth.authorize(ctx, identity, openURL)
	if err != nil {
		return Session{}, fmt.Errorf("authorize ATProto identity: %w", err)
	}

	token, err := c.oauth.exchangeAuthorizationCode(ctx, authorization.Server, authorization)
	if err != nil {
		return Session{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	return Session{
		Identity: identity, AccessToken: token.AccessToken,
		RefreshToken: token.RefreshToken, TokenType: token.TokenType,
		Scope: token.Scope, DPoPNonce: token.DPoPNonce,
		dpopKey: authorization.DPoPKey, server: authorization.Server,
	}, nil
}
