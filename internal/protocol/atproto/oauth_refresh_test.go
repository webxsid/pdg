package atproto

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestRefreshAccessTokenRotatesCredentialsAndNonce(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	server := AuthorizationServer{TokenEndpoint: "https://auth.example.com/oauth/token"}
	session := Session{Identity: Identity{DID: "did:plc:test"}, RefreshToken: "old-refresh", DPoPNonce: "old-nonce", dpopKey: key, clientID: "http://localhost"}
	requests := 0
	client := &oauthClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if err := req.ParseForm(); err != nil {
			return nil, err
		}
		if req.Form.Get("grant_type") != "refresh_token" || req.Form.Get("refresh_token") != "old-refresh" || req.Form.Get("client_id") != "http://localhost" {
			return nil, fmt.Errorf("unexpected refresh form: %s", req.Form.Encode())
		}
		if got := req.Header.Get("DPoP"); got == "" {
			return nil, fmt.Errorf("DPoP header missing")
		} else {
			assertDPoPNonce(t, got, "old-nonce")
		}
		response := testJSONResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"DPoP","scope":"atproto","sub":"did:plc:test","expires_in":3600}`)
		response.Header.Set("DPoP-Nonce", "new-nonce")
		return response, nil
	})}}
	token, err := client.refreshAccessToken(context.Background(), server, session)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("refresh requests = %d, want 1", requests)
	}
	if token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" || token.DPoPNonce != "new-nonce" {
		t.Errorf("refresh token response = %+v, want rotated credentials", token)
	}
	if token.AccessTokenExpiresAt == nil {
		t.Error("refresh expiry is nil, want expiry")
	}
}

func TestRefreshAccessTokenRetriesNonceOnce(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	server := AuthorizationServer{TokenEndpoint: "https://auth.example.com/oauth/token"}
	session := Session{Identity: Identity{DID: "did:plc:test"}, RefreshToken: "refresh", DPoPNonce: "old", dpopKey: key, clientID: "http://localhost"}
	requests := 0
	client := &oauthClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		proof := req.Header.Get("DPoP")
		if proof == "" {
			return nil, fmt.Errorf("DPoP header missing")
		}
		if requests == 1 {
			assertDPoPNonce(t, proof, "old")
			response := testJSONResponse(http.StatusBadRequest, `{"error":"use_dpop_nonce"}`)
			response.Header.Set("DPoP-Nonce", "new")
			return response, nil
		}
		if requests == 2 {
			assertDPoPNonce(t, proof, "new")
			response := testJSONResponse(http.StatusOK, `{"access_token":"access","token_type":"DPoP","scope":"atproto","sub":"did:plc:test"}`)
			response.Header.Set("DPoP-Nonce", "final")
			return response, nil
		}
		return nil, fmt.Errorf("unexpected third refresh request")
	})}}
	token, err := client.refreshAccessToken(context.Background(), server, session)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("refresh requests = %d, want 2", requests)
	}
	if token.RefreshToken != "refresh" {
		t.Errorf("refresh token = %q, want reused original", token.RefreshToken)
	}
	if token.DPoPNonce != "final" {
		t.Errorf("DPoP nonce = %q, want final", token.DPoPNonce)
	}
}
