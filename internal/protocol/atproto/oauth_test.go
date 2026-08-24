package atproto

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/webxsid/pdg/internal/shared"
)

func TestDiscoverAuthorizationServer(t *testing.T) {
	const (
		pds    = "https://pds.example.com"
		issuer = "https://auth.example.com"
	)

	httpClient := &http.Client{
		Transport: roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				var body string

				switch req.URL.String() {
				case pds + "/.well-known/oauth-protected-resource":
					body = `{
						"authorization_servers": [
							"https://auth.example.com"
						]
					}`

				case issuer + "/.well-known/oauth-authorization-server":
					body = `{
						"issuer": "https://auth.example.com",
						"authorization_endpoint": "https://auth.example.com/oauth/authorize",
						"token_endpoint": "https://auth.example.com/oauth/token",
						"pushed_authorization_request_endpoint": "https://auth.example.com/oauth/par",
						"code_challenge_methods_supported": ["S256"],
						"dpop_signing_alg_values_supported": ["ES256"]
					}`

				default:
					return nil, fmt.Errorf(
						"unexpected request: %s",
						req.URL,
					)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body: io.NopCloser(
						strings.NewReader(body),
					),
					Header: make(http.Header),
				}, nil
			},
		),
	}

	client := &oauthClient{
		client: httpClient,
	}

	got, err := client.DiscoverAuthorizationServer(
		context.Background(),
		pds,
	)
	if err != nil {
		t.Fatalf(
			"discoverAuthorizationServer() unexpected error = %v",
			err,
		)
	}

	if got.Issuer != issuer {
		t.Errorf(
			"Issuer = %q, want %q",
			got.Issuer,
			issuer,
		)
	}

	if got.AuthorizationEndpoint !=
		"https://auth.example.com/oauth/authorize" {
		t.Errorf(
			"AuthorizationEndpoint = %q",
			got.AuthorizationEndpoint,
		)
	}

	if got.TokenEndpoint !=
		"https://auth.example.com/oauth/token" {
		t.Errorf(
			"TokenEndpoint = %q",
			got.TokenEndpoint,
		)
	}

	if got.PushedAuthorizationRequestEndpoint !=
		"https://auth.example.com/oauth/par" {
		t.Errorf(
			"PushedAuthorizationRequestEndpoint = %q",
			got.PushedAuthorizationRequestEndpoint,
		)
	}
}

func TestDiscoverAuthorizationServerRejectsInvalidMetadata(
	t *testing.T,
) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing authorization server",
			body: `{
				"authorization_servers": []
			}`,
		},
		{
			name: "multiple authorization servers",
			body: `{
				"authorization_servers": [
					"https://auth-a.example.com",
					"https://auth-b.example.com"
				]
			}`,
		},
		{
			name: "http authorization server",
			body: `{
				"authorization_servers": [
					"http://auth.example.com"
				]
			}`,
		},
		{
			name: "authorization server with path",
			body: `{
				"authorization_servers": [
					"https://auth.example.com/oauth"
				]
			}`,
		},
		{
			name: "authorization server with query",
			body: `{
				"authorization_servers": [
					"https://auth.example.com?foo=bar"
				]
			}`,
		},
		{
			name: "authorization server with fragment",
			body: `{
				"authorization_servers": [
					"https://auth.example.com#foo"
				]
			}`,
		},
		{
			name: "authorization server with userinfo",
			body: `{
				"authorization_servers": [
					"https://user:pass@auth.example.com"
				]
			}`,
		},
		{
			name: "explicit default https port",
			body: `{
				"authorization_servers": [
					"https://auth.example.com:443"
				]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const pds = "https://pds.example.com"

			httpClient := &http.Client{
				Transport: roundTripFunc(
					func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Status:     "200 OK",
							Body: io.NopCloser(
								strings.NewReader(tt.body),
							),
							Header: make(http.Header),
						}, nil
					},
				),
			}

			client := &oauthClient{
				client: httpClient,
			}

			_, err := client.discoverIssuer(
				context.Background(),
				pds,
			)

			if err == nil {
				t.Fatal(
					"discoverAuthorizationServer() error = nil, want error",
				)
			}
		})
	}
}

func TestDiscoverAuthorizationServerRejectsMalformedJSON(
	t *testing.T,
) {
	const pds = "https://pds.example.com"

	httpClient := &http.Client{
		Transport: roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body: io.NopCloser(
						strings.NewReader(`{"authorization_servers":`),
					),
					Header: make(http.Header),
				}, nil
			},
		),
	}

	client := &oauthClient{
		client: httpClient,
	}

	_, err := client.discoverIssuer(
		context.Background(),
		pds,
	)

	if err == nil {
		t.Fatal(
			"discoverAuthorizationServer() error = nil, want error",
		)
	}
}

func TestValidateAuthorizationServerMetadata(t *testing.T) {
	const issuer = "https://auth.example.com"

	valid := authorizationServerMetadata{
		Issuer:                             issuer,
		AuthorizationEndpoint:              "https://auth.example.com/oauth/authorize",
		TokenEndpoint:                      "https://auth.example.com/oauth/token",
		PushedAuthorizationRequestEndpoint: "https://auth.example.com/oauth/par",
		CodeChallengeMethodsSupported:      []string{"S256"},
		DPoPSigningAlgValuesSupported:      []string{"ES256"},
	}

	tests := []struct {
		name     string
		metadata authorizationServerMetadata
		wantErr  bool
	}{
		{
			name:     "valid metadata",
			metadata: valid,
		},
		{
			name: "issuer mismatch",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.Issuer = "https://other.example.com"

				return m
			}(),
			wantErr: true,
		},
		{
			name: "missing authorization endpoint",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.AuthorizationEndpoint = ""

				return m
			}(),
			wantErr: true,
		},
		{
			name: "insecure authorization endpoint",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.AuthorizationEndpoint = "http://auth.example.com/oauth/authorize"

				return m
			}(),
			wantErr: true,
		},
		{
			name: "missing token endpoint",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.TokenEndpoint = ""

				return m
			}(),
			wantErr: true,
		},
		{
			name: "missing PAR endpoint",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.PushedAuthorizationRequestEndpoint = ""

				return m
			}(),
			wantErr: true,
		},
		{
			name: "missing S256",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.CodeChallengeMethodsSupported = []string{"plain"}

				return m
			}(),
			wantErr: true,
		},
		{
			name: "missing PKCE methods",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.CodeChallengeMethodsSupported = nil

				return m
			}(),
			wantErr: true,
		},
		{
			name: "missing DPoP algorithms",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.DPoPSigningAlgValuesSupported = nil

				return m
			}(),
			wantErr: true,
		},
		{
			name: "authorization endpoint with userinfo",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.AuthorizationEndpoint = "https://user:pass@auth.example.com/oauth/authorize"

				return m
			}(),
			wantErr: true,
		},
		{
			name: "token endpoint with fragment",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.TokenEndpoint = "https://auth.example.com/oauth/token#foo"

				return m
			}(),
			wantErr: true,
		},
		{
			name: "PAR endpoint without host",
			metadata: func() authorizationServerMetadata {
				m := valid
				m.PushedAuthorizationRequestEndpoint = "https:///oauth/par"

				return m
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuthorizationServerMetadata(
				issuer,
				tt.metadata,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal(
						"validateAuthorizationServerMetadata() error = nil, want error",
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"validateAuthorizationServerMetadata() unexpected error = %v",
					err,
				)
			}
		})
	}
}

func TestValidateHTTPSEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:  "valid",
			value: "https://auth.example.com/oauth/token",
		},
		{
			name:  "valid root",
			value: "https://auth.example.com",
		},
		{
			name:    "empty",
			value:   "",
			wantErr: true,
		},
		{
			name:    "http",
			value:   "http://auth.example.com/oauth/token",
			wantErr: true,
		},
		{
			name:    "missing host",
			value:   "https:///oauth/token",
			wantErr: true,
		},
		{
			name:    "userinfo",
			value:   "https://user:pass@auth.example.com/oauth/token",
			wantErr: true,
		},
		{
			name:    "fragment",
			value:   "https://auth.example.com/oauth/token#foo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := shared.ValidateHTTPSEndpoint(tt.value)

			if tt.wantErr && err == nil {
				t.Fatalf(
					"validateHTTPSEndpoint(%q) error = nil, want error",
					tt.value,
				)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf(
					"validateHTTPSEndpoint(%q) unexpected error = %v",
					tt.value,
					err,
				)
			}
		})
	}
}
