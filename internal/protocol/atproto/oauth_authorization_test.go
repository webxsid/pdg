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

func TestNewAuthorizationState(t *testing.T) {
	got, err := newAuthorizationState()
	if err != nil {
		t.Fatalf(
			"newAuthorizationState() returned error: %v",
			err,
		)
	}

	if got.State == "" {
		t.Errorf("State is empty")
	}

	if got.CodeVerifier == "" {
		t.Errorf("CodeVerifier is empty")
	}

	if got.CodeChallenge == "" {
		t.Errorf("CodeChallenge is empty")
	}

	if strings.Contains(got.State, "=") {
		t.Errorf("State contains base64 padding")
	}

	if strings.Contains(got.CodeVerifier, "=") {
		t.Errorf("CodeVerifier contains base64 padding")
	}

	wantChallenge := pkceChallenge(got.CodeVerifier)
	if got.CodeChallenge != wantChallenge {
		t.Errorf(
			"CodeChallenge = %q, want %q",
			got.CodeChallenge,
			wantChallenge,
		)
	}
}

func TestNewAuthorizationStateGeneratesUniqueValues(t *testing.T) {
	a, err := newAuthorizationState()
	if err != nil {
		t.Fatal(err)
	}

	b, err := newAuthorizationState()
	if err != nil {
		t.Fatal(err)
	}

	if a.State == b.State {
		t.Error("generated identical OAuth states")
	}

	if a.CodeVerifier == b.CodeVerifier {
		t.Error("generated identical PKCE verifiers")
	}
}

func TestBuildAuthorizationURL(t *testing.T) {
	server := AuthorizationServer{
		AuthorizationEndpoint: "https://auth.example.com/oauth/authorize",
	}

	got, err := buildAuthorizationURL(
		server,
		"http://localhost?redirect_uri=http%3A%2F%2F127.0.0.1%3A49152%2Fcallback&scope=atproto",
		"urn:ietf:params:oauth:request_uri:abc",
	)
	if err != nil {
		t.Fatalf(
			"buildAuthorizationURL() unexpected error = %v",
			err,
		)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Scheme != "https" {
		t.Errorf(
			"scheme = %q, want https",
			parsed.Scheme,
		)
	}

	if parsed.Host != "auth.example.com" {
		t.Errorf(
			"host = %q, want auth.example.com",
			parsed.Host,
		)
	}

	if parsed.Path != "/oauth/authorize" {
		t.Errorf(
			"path = %q, want /oauth/authorize",
			parsed.Path,
		)
	}

	query := parsed.Query()

	if query.Get("request_uri") !=
		"urn:ietf:params:oauth:request_uri:abc" {
		t.Errorf(
			"request_uri = %q",
			query.Get("request_uri"),
		)
	}

	if query.Get("client_id") == "" {
		t.Error("client_id is missing")
	}
}

func TestAuthorize(t *testing.T) {
	const (
		pds    = "https://pds.example.com"
		issuer = "https://auth.example.com"
		did    = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"
	)

	var capturedState string

	httpClient := &http.Client{
		Transport: roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case pds + "/.well-known/oauth-protected-resource":
					return testJSONResponse(
						http.StatusOK,
						`{
							"authorization_servers": [
								"https://auth.example.com"
							]
						}`,
					), nil

				case issuer + "/.well-known/oauth-authorization-server":
					return testJSONResponse(
						http.StatusOK,
						`{
							"issuer": "https://auth.example.com",
							"authorization_endpoint": "https://auth.example.com/oauth/authorize",
							"token_endpoint": "https://auth.example.com/oauth/token",
							"pushed_authorization_request_endpoint": "https://auth.example.com/oauth/par",
							"code_challenge_methods_supported": ["S256"],
							"dpop_signing_alg_values_supported": ["ES256"]
						}`,
					), nil

				case issuer + "/oauth/par":
					if err := req.ParseForm(); err != nil {
						return nil, err
					}

					capturedState = req.Form.Get("state")

					if capturedState == "" {
						return nil, fmt.Errorf("state is missing")
					}

					response := testJSONResponse(
						http.StatusCreated,
						`{
							"request_uri": "urn:ietf:params:oauth:request_uri:test",
							"expires_in": 90
						}`,
					)

					response.Header.Set(
						"DPoP-Nonce",
						"test-nonce",
					)

					return response, nil

				default:
					return nil, fmt.Errorf(
						"unexpected request: %s",
						req.URL,
					)
				}
			},
		),
	}

	client := &oauthClient{
		client: httpClient,
	}

	openURL := func(rawURL string) error {
		authorizationURL, err := url.Parse(rawURL)
		if err != nil {
			return err
		}

		query := authorizationURL.Query()

		if query.Get("request_uri") !=
			"urn:ietf:params:oauth:request_uri:test" {
			return fmt.Errorf(
				"unexpected request_uri: %q",
				query.Get("request_uri"),
			)
		}

		clientID := query.Get("client_id")
		if clientID == "" {
			return fmt.Errorf("client_id is missing")
		}

		/*
			The callback URI is encoded inside the localhost
			client_id, so recover it from there.
		*/
		parsedClientID, err := url.Parse(clientID)
		if err != nil {
			return err
		}

		redirectURI := parsedClientID.Query().Get("redirect_uri")

		if redirectURI == "" {
			return fmt.Errorf(
				"redirect_uri missing from client_id",
			)
		}

		callbackURL, err := url.Parse(redirectURI)
		if err != nil {
			return err
		}

		callbackQuery := callbackURL.Query()
		callbackQuery.Set("code", "test-authorization-code")
		callbackQuery.Set("state", capturedState)

		callbackURL.RawQuery = callbackQuery.Encode()

		go func() {
			resp, err := http.Get(callbackURL.String())
			if err != nil {
				t.Errorf(
					"GET callback: %v",
					err,
				)
				return
			}

			_ = resp.Body.Close()
		}()

		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second*5,
	)
	defer cancel()

	result, err := client.authorize(
		ctx,
		Identity{
			DID: did,
			PDS: pds,
		},
		openURL,
	)
	if err != nil {
		t.Fatalf(
			"Authorize() unexpected error = %v",
			err,
		)
	}

	if result.Code != "test-authorization-code" {
		t.Errorf(
			"result.Code = %q, want %q",
			result.Code,
			"test-authorization-code",
		)
	}

	if result.CodeVerifier == "" {
		t.Error("result.CodeVerifier is empty")
	}

	if result.RedirectURI == "" {
		t.Error("result.RedirectURI is empty")
	}

	if result.ClientID == "" {
		t.Error("result.ClientID is empty")
	}

	if result.DPoPNonce != "test-nonce" {
		t.Errorf(
			"result.DPoPNonce = %q, want %q",
			result.DPoPNonce,
			"test-nonce",
		)
	}

	if result.DPoPKey == nil {
		t.Error("result.DPoPKey is nil")
	}

	if result.Server.Issuer != issuer {
		t.Errorf(
			"result.Server.Issuer = %q, want %q",
			result.Server.Issuer,
			issuer,
		)
	}

	if result.Subject != did {
		t.Errorf(
			"result.Subject = %q, want %q",
			result.Subject,
			did,
		)
	}
}

func testJSONResponse(
	status int,
	body string,
) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status: fmt.Sprintf(
			"%d %s",
			status,
			http.StatusText(status),
		),
		Header: http.Header{
			"Content-Type": []string{
				"application/json",
			},
		},
		Body: io.NopCloser(
			strings.NewReader(body),
		),
	}
}
