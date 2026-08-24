package atproto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestExchangeAuthorizationCode(t *testing.T) {
	const tokenEndpoint = "https://auth.example.com/oauth/token"

	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	client := &oauthClient{
		client: &http.Client{
			Transport: roundTripFunc(
				func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost {
						t.Errorf(
							"method = %q, want POST",
							req.Method,
						)
					}

					if req.URL.String() != tokenEndpoint {
						t.Errorf(
							"URL = %q, want %q",
							req.URL.String(),
							tokenEndpoint,
						)
					}

					if got := req.Header.Get("Content-Type"); got !=
						"application/x-www-form-urlencoded" {
						t.Errorf(
							"Content-Type = %q",
							got,
						)
					}

					proof := req.Header.Get("DPoP")
					if proof == "" {
						t.Fatal("DPoP header is missing")
					}

					assertDPoPNonce(
						t,
						proof,
						"par-nonce",
					)

					if err := req.ParseForm(); err != nil {
						return nil, err
					}

					assertFormValue(
						t,
						req,
						"grant_type",
						"authorization_code",
					)

					assertFormValue(
						t,
						req,
						"code",
						"test-code",
					)

					assertFormValue(
						t,
						req,
						"code_verifier",
						"test-verifier",
					)

					assertFormValue(
						t,
						req,
						"redirect_uri",
						"http://127.0.0.1:49152/callback",
					)

					assertFormValue(
						t,
						req,
						"client_id",
						"http://localhost",
					)

					response := testJSONResponse(
						http.StatusOK,
						`{
							"access_token": "access-token",
							"refresh_token": "refresh-token",
							"token_type": "DPoP",
							"scope": "atproto",
							"sub": "did:plc:test"
						}`,
					)

					response.Header.Set(
						"DPoP-Nonce",
						"token-nonce",
					)

					return response, nil
				},
			),
		},
	}

	got, err := client.exchangeAuthorizationCode(
		context.Background(),
		AuthorizationServer{
			TokenEndpoint: tokenEndpoint,
		},
		authorizationResult{
			Code:         "test-code",
			CodeVerifier: "test-verifier",
			RedirectURI:  "http://127.0.0.1:49152/callback",
			ClientID:     "http://localhost",
			DPoPNonce:    "par-nonce",
			DPoPKey:      key,
			Subject:      "did:plc:test",
		},
	)
	if err != nil {
		t.Fatalf(
			"exchangeAuthorizationCode() unexpected error = %v",
			err,
		)
	}

	if got.AccessToken != "access-token" {
		t.Errorf(
			"AccessToken = %q, want access-token",
			got.AccessToken,
		)
	}

	if got.RefreshToken != "refresh-token" {
		t.Errorf(
			"RefreshToken = %q, want refresh-token",
			got.RefreshToken,
		)
	}

	if got.TokenType != "DPoP" {
		t.Errorf(
			"TokenType = %q, want DPoP",
			got.TokenType,
		)
	}

	if got.Subject != "did:plc:test" {
		t.Errorf(
			"Subject = %q, want did:plc:test",
			got.Subject,
		)
	}

	if got.DPoPNonce != "token-nonce" {
		t.Errorf(
			"DPoPNonce = %q, want token-nonce",
			got.DPoPNonce,
		)
	}
}

func TestExchangeAuthorizationCodeRetriesWithDPoPNonce(
	t *testing.T,
) {
	const tokenEndpoint = "https://auth.example.com/oauth/token"

	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	requestCount := 0

	client := &oauthClient{
		client: &http.Client{
			Transport: roundTripFunc(
				func(req *http.Request) (*http.Response, error) {
					requestCount++

					proof := req.Header.Get("DPoP")
					if proof == "" {
						t.Fatal("DPoP header missing")
					}

					switch requestCount {
					case 1:
						assertDPoPNonce(
							t,
							proof,
							"old-nonce",
						)

						response := testJSONResponse(
							http.StatusBadRequest,
							`{
								"error": "use_dpop_nonce",
								"error_description": "new nonce required"
							}`,
						)

						response.Header.Set(
							"DPoP-Nonce",
							"new-nonce",
						)

						return response, nil

					case 2:
						assertDPoPNonce(
							t,
							proof,
							"new-nonce",
						)

						response := testJSONResponse(
							http.StatusOK,
							`{
								"access_token": "access-token",
								"refresh_token": "refresh-token",
								"token_type": "DPoP",
								"scope": "atproto",
								"sub": "did:plc:test"
							}`,
						)

						response.Header.Set(
							"DPoP-Nonce",
							"final-nonce",
						)

						return response, nil

					default:
						return nil, fmt.Errorf(
							"unexpected token request %d",
							requestCount,
						)
					}
				},
			),
		},
	}

	got, err := client.exchangeAuthorizationCode(
		context.Background(),
		AuthorizationServer{
			TokenEndpoint: tokenEndpoint,
		},
		authorizationResult{
			Code:         "test-code",
			CodeVerifier: "test-verifier",
			RedirectURI:  "http://127.0.0.1:49152/callback",
			ClientID:     "http://localhost",
			DPoPNonce:    "old-nonce",
			DPoPKey:      key,
			Subject:      "did:plc:test",
		},
	)
	if err != nil {
		t.Fatalf(
			"exchangeAuthorizationCode() unexpected error = %v",
			err,
		)
	}

	if requestCount != 2 {
		t.Errorf(
			"request count = %d, want 2",
			requestCount,
		)
	}

	if got.AccessToken != "access-token" {
		t.Errorf(
			"AccessToken = %q",
			got.AccessToken,
		)
	}

	if got.DPoPNonce != "final-nonce" {
		t.Errorf(
			"DPoPNonce = %q, want final-nonce",
			got.DPoPNonce,
		)
	}
}

func TestExchangeAuthorizationCodeRejectsSubjectMismatch(
	t *testing.T,
) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	client := &oauthClient{
		client: &http.Client{
			Transport: roundTripFunc(
				func(req *http.Request) (*http.Response, error) {
					response := testJSONResponse(
						http.StatusOK,
						`{
							"access_token": "access-token",
							"refresh_token": "refresh-token",
							"token_type": "DPoP",
							"scope": "atproto",
							"sub": "did:plc:someone-else"
						}`,
					)

					response.Header.Set(
						"DPoP-Nonce",
						"token-nonce",
					)

					return response, nil
				},
			),
		},
	}

	_, err = client.exchangeAuthorizationCode(
		context.Background(),
		AuthorizationServer{
			TokenEndpoint: "https://auth.example.com/oauth/token",
		},
		authorizationResult{
			Code:         "test-code",
			CodeVerifier: "test-verifier",
			RedirectURI:  "http://127.0.0.1:49152/callback",
			ClientID:     "http://localhost",
			DPoPNonce:    "par-nonce",
			DPoPKey:      key,
			Subject:      "did:plc:expected",
		},
	)

	if err == nil {
		t.Fatal(
			"exchangeAuthorizationCode() error = nil, want error",
		)
	}
}

func TestExchangeAuthorizationCodeRejectsInvalidResponse(
	t *testing.T,
) {
	tests := []struct {
		name  string
		body  string
		nonce string
	}{
		{
			name: "missing access token",
			body: `{
				"token_type":"DPoP",
				"sub":"did:plc:test"
			}`,
			nonce: "nonce",
		},
		{
			name: "wrong token type",
			body: `{
				"access_token":"token",
				"token_type":"Bearer",
				"sub":"did:plc:test"
			}`,
			nonce: "nonce",
		},
		{
			name: "missing subject",
			body: `{
				"access_token":"token",
				"token_type":"DPoP"
			}`,
			nonce: "nonce",
		},
		{
			name: "missing DPoP nonce",
			body: `{
				"access_token":"token",
				"token_type":"DPoP",
				"sub":"did:plc:test"
			}`,
		},
		{
			name:  "malformed JSON",
			body:  `{`,
			nonce: "nonce",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := newDPoPKey()
			if err != nil {
				t.Fatal(err)
			}

			client := &oauthClient{
				client: &http.Client{
					Transport: roundTripFunc(
						func(req *http.Request) (*http.Response, error) {
							response := testJSONResponse(
								http.StatusOK,
								tt.body,
							)

							if tt.nonce != "" {
								response.Header.Set(
									"DPoP-Nonce",
									tt.nonce,
								)
							}

							return response, nil
						},
					),
				},
			}

			_, err = client.exchangeAuthorizationCode(
				context.Background(),
				AuthorizationServer{
					TokenEndpoint: "https://auth.example.com/oauth/token",
				},
				authorizationResult{
					Code:         "code",
					CodeVerifier: "verifier",
					RedirectURI:  "http://127.0.0.1/callback",
					ClientID:     "http://localhost",
					DPoPNonce:    "nonce",
					DPoPKey:      key,
					Subject:      "did:plc:test",
				},
			)

			if err == nil {
				t.Fatal(
					"exchangeAuthorizationCode() error = nil, want error",
				)
			}
		})
	}
}

func assertDPoPNonce(
	t *testing.T,
	proof string,
	want string,
) {
	t.Helper()

	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf(
			"DPoP JWT has %d parts, want 3",
			len(parts),
		)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf(
			"decode DPoP claims: %v",
			err,
		)
	}

	var claims dpopProofClaims

	if err := json.Unmarshal(
		payload,
		&claims,
	); err != nil {
		t.Fatalf(
			"unmarshal DPoP claims: %v",
			err,
		)
	}

	if claims.Nonce != want {
		t.Errorf(
			"DPoP nonce = %q, want %q",
			claims.Nonce,
			want,
		)
	}
}
