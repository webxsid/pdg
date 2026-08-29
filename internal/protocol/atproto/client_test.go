package atproto

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClientLogin(t *testing.T) {
	const (
		handle = "login.example.com"
		did    = "did:web:login.example.com"
		pds    = "https://pds.example.com"
		issuer = "https://auth.example.com"
	)

	var state string
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://login.example.com/.well-known/atproto-did":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(did)),
				Header:     make(http.Header),
			}, nil
		case "https://login.example.com/.well-known/did.json":
			return testJSONResponse(http.StatusOK, fmt.Sprintf(`{
				"id": %q,
				"alsoKnownAs": ["at://%s"],
				"service": [{"id":"#atproto_pds","type":"AtprotoPersonalDataServer","serviceEndpoint":%q}]
			}`, did, handle, pds)), nil
		case pds + "/.well-known/oauth-protected-resource":
			return testJSONResponse(http.StatusOK, `{"authorization_servers":["`+issuer+`"]}`), nil
		case issuer + "/.well-known/oauth-authorization-server":
			return testJSONResponse(http.StatusOK, `{
				"issuer":"`+issuer+`",
				"authorization_endpoint":"`+issuer+`/oauth/authorize",
				"token_endpoint":"`+issuer+`/oauth/token",
				"pushed_authorization_request_endpoint":"`+issuer+`/oauth/par",
				"code_challenge_methods_supported":["S256"],
				"dpop_signing_alg_values_supported":["ES256"]
			}`), nil
		case issuer + "/oauth/par":
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			state = req.Form.Get("state")
			response := testJSONResponse(http.StatusCreated, `{"request_uri":"urn:ietf:params:oauth:request_uri:test","expires_in":90}`)
			response.Header.Set("DPoP-Nonce", "par-nonce")
			return response, nil
		case issuer + "/oauth/token":
			return func() *http.Response {
				response := testJSONResponse(http.StatusOK, `{"access_token":"access","refresh_token":"refresh","token_type":"DPoP","scope":"atproto","sub":"`+did+`"}`)
				response.Header.Set("DPoP-Nonce", "token-nonce")
				return response
			}(), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s", req.URL)
		}
	})})

	openURL := func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		clientID, err := url.QueryUnescape(parsed.Query().Get("client_id"))
		if err != nil {
			return err
		}
		clientIDURL, err := url.Parse(clientID)
		if err != nil {
			return err
		}
		callback, err := url.Parse(clientIDURL.Query().Get("redirect_uri"))
		if err != nil {
			return err
		}
		query := callback.Query()
		query.Set("code", "authorization-code")
		query.Set("state", state)
		callback.RawQuery = query.Encode()
		go func() {
			response, requestErr := http.Get(callback.String())
			if requestErr == nil {
				_ = response.Body.Close()
			}
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Login(ctx, handle, openURL)
	if err != nil {
		t.Fatalf("Client.Login(%q) unexpected error = %v", handle, err)
	}

	if session.Identity.DID != did {
		t.Errorf("Client.Login(%q).Identity.DID = %q, want %q", handle, session.Identity.DID, did)
	}
	if session.Identity.PDS != pds {
		t.Errorf("Client.Login(%q).Identity.PDS = %q, want %q", handle, session.Identity.PDS, pds)
	}
	if session.TokenType != "DPoP" {
		t.Errorf("Client.Login(%q).TokenType = %q, want %q", handle, session.TokenType, "DPoP")
	}
	if session.Scope != "atproto" {
		t.Errorf("Client.Login(%q).Scope = %q, want %q", handle, session.Scope, "atproto")
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Errorf("Client.Login(%q) returned empty session credentials", handle)
	}
	if session.DPoPNonce != "token-nonce" {
		t.Errorf("Client.Login(%q).DPoPNonce = %q, want %q", handle, session.DPoPNonce, "token-nonce")
	}
	if session.dpopKey == nil {
		t.Errorf("Client.Login(%q).dpopKey is nil", handle)
	}
	if session.server.Issuer != issuer {
		t.Errorf("Client.Login(%q).server.Issuer = %q, want %q", handle, session.server.Issuer, issuer)
	}
}
