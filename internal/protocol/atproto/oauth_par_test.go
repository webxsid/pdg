package atproto

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPushAuthorizationRequest(t *testing.T) {
	const (
		parEndpoint = "https://auth.example.com/oauth/par"
		clientID    = "http://localhost?redirect_uri=http%3A%2F%2F127.0.0.1%3A49152%2Fcallback&scope=atproto"
		redirectURI = "http://127.0.0.1:49152/callback"
	)

	httpClient := &http.Client{
		Transport: roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Errorf(
						"method = %q, want POST",
						req.Method,
					)
				}

				if req.URL.String() != parEndpoint {
					t.Errorf(
						"URL = %q, want %q",
						req.URL.String(),
						parEndpoint,
					)
				}

				if got := req.Header.Get("Content-Type"); got !=
					"application/x-www-form-urlencoded" {
					t.Errorf(
						"Content-Type = %q",
						got,
					)
				}

				if req.Header.Get("DPoP") == "" {
					t.Error("DPoP header is missing")
				}

				if err := req.ParseForm(); err != nil {
					return nil, err
				}

				assertFormValue(
					t,
					req,
					"client_id",
					clientID,
				)

				assertFormValue(
					t,
					req,
					"response_type",
					"code",
				)

				assertFormValue(
					t,
					req,
					"redirect_uri",
					redirectURI,
				)

				assertFormValue(
					t,
					req,
					"scope",
					"atproto",
				)

				assertFormValue(
					t,
					req,
					"state",
					"test-state",
				)

				assertFormValue(
					t,
					req,
					"code_challenge",
					"test-challenge",
				)

				assertFormValue(
					t,
					req,
					"code_challenge_method",
					"S256",
				)

				assertFormValue(
					t,
					req,
					"login_hint",
					"did:plc:ewvi7nxzyoun6zhxrhs64oiz",
				)

				response := &http.Response{
					StatusCode: http.StatusCreated,
					Status:     "201 Created",
					Header:     make(http.Header),
					Body: io.NopCloser(
						strings.NewReader(`{
							"request_uri": "urn:ietf:params:oauth:request_uri:abc",
							"expires_in": 60
						}`),
					),
				}

				response.Header.Set("DPoP-Nonce", "nonce-123")

				return response, nil
			},
		),
	}

	client := &oauthClient{
		client: httpClient,
	}

	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.pushAuthorizationRequest(
		context.Background(),
		AuthorizationServer{
			PushedAuthorizationRequestEndpoint: parEndpoint,
		},
		parRequest{
			ClientID:      clientID,
			RedirectURI:   redirectURI,
			Scope:         "atproto",
			LoginHint:     "did:plc:ewvi7nxzyoun6zhxrhs64oiz",
			State:         "test-state",
			CodeChallenge: "test-challenge",
		},
		key,
	)
	if err != nil {
		t.Fatalf(
			"pushAuthorizationRequest() unexpected error = %v",
			err,
		)
	}

	if got.RequestURI !=
		"urn:ietf:params:oauth:request_uri:abc" {
		t.Errorf(
			"RequestURI = %q",
			got.RequestURI,
		)
	}

	if got.ExpiresIn != 60 {
		t.Errorf(
			"ExpiresIn = %d, want 60",
			got.ExpiresIn,
		)
	}

	if got.DPoPNonce != "nonce-123" {
		t.Errorf(
			"DPoPNonce = %q, want nonce-123",
			got.DPoPNonce,
		)
	}
}

func TestPushAuthorizationRequestRejectsInvalidResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		nonce      string
	}{
		{
			name:       "non 201 response",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"invalid_request"}`,
			nonce:      "nonce-123",
		},
		{
			name:       "missing request uri",
			statusCode: http.StatusCreated,
			body:       `{"expires_in":60}`,
			nonce:      "nonce-123",
		},
		{
			name:       "missing dpop nonce",
			statusCode: http.StatusCreated,
			body: `{
				"request_uri":"urn:ietf:params:oauth:request_uri:abc",
				"expires_in":60
			}`,
		},
		{
			name:       "invalid expires in",
			statusCode: http.StatusCreated,
			body: `{
				"request_uri":"urn:ietf:params:oauth:request_uri:abc",
				"expires_in":0
			}`,
			nonce: "nonce-123",
		},
		{
			name:       "malformed json",
			statusCode: http.StatusCreated,
			body:       `{`,
			nonce:      "nonce-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{
				Transport: roundTripFunc(
					func(req *http.Request) (*http.Response, error) {
						response := &http.Response{
							StatusCode: tt.statusCode,
							Status:     http.StatusText(tt.statusCode),
							Header:     make(http.Header),
							Body: io.NopCloser(
								strings.NewReader(tt.body),
							),
						}

						if tt.nonce != "" {
							response.Header.Set(
								"DPoP-Nonce",
								tt.nonce,
							)
						}

						return response, nil
					},
				),
			}

			client := &oauthClient{
				client: httpClient,
			}

			key, err := newDPoPKey()
			if err != nil {
				t.Fatal(err)
			}

			_, err = client.pushAuthorizationRequest(
				context.Background(),
				AuthorizationServer{
					PushedAuthorizationRequestEndpoint: "https://auth.example.com/oauth/par",
				},
				parRequest{
					ClientID:      "http://localhost",
					RedirectURI:   "http://127.0.0.1:49152/callback",
					Scope:         "atproto",
					State:         "test-state",
					CodeChallenge: "test-challenge",
				},
				key,
			)

			if err == nil {
				t.Fatal(
					"pushAuthorizationRequest() error = nil, want error",
				)
			}
		})
	}
}

func assertFormValue(
	t *testing.T,
	req *http.Request,
	key string,
	want string,
) {
	t.Helper()

	got := req.Form.Get(key)

	if got != want {
		t.Errorf(
			"%s = %q, want %q",
			key,
			got,
			want,
		)
	}
}
