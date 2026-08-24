package atproto

import (
	"context"
	"net/http"
)

type Client struct {
	identity *Resolver
	oauth    *oauthClient
}

func NewClient(
	httpClient *http.Client,
) *Client {
	return &Client{
		identity: newResolver(httpClient),
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
