package atproto

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeXRPCSessions struct{ session Session }

func (f *fakeXRPCSessions) LoadActiveSession(context.Context) (Session, error) { return f.session, nil }
func (f *fakeXRPCSessions) RefreshSession(context.Context, *Client, string) (Session, error) {
	return f.session, nil
}
func (f *fakeXRPCSessions) SaveSession(_ context.Context, session Session, _ bool) error {
	f.session = session
	return nil
}

func TestXRPCClientBuildsAuthenticatedRequest(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	var gotAuth, gotDPoP string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotDPoP = r.Header.Get("DPoP")
		if r.Method != http.MethodPost || r.URL.Path != "/xrpc/com.example.echo" {
			t.Errorf("request = %s %s, want POST /xrpc/com.example.echo", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sessions := &fakeXRPCSessions{session: Session{
		Identity:    Identity{DID: "did:plc:test", Handle: "test.example", PDS: server.URL},
		AccessToken: "access-token", TokenType: "DPoP", DPoPNonce: "nonce", dpopKey: key,
	}}
	client := NewXRPCClient(server.Client(), sessions)
	var result map[string]bool
	if err := client.Do(context.Background(), http.MethodPost, "com.example.echo", map[string]string{"value": "test"}, &result); err != nil {
		t.Fatal(err)
	}
	if !result["ok"] {
		t.Fatal("response was not decoded")
	}
	if gotAuth != "DPoP access-token" {
		t.Errorf("Authorization = %q, want DPoP access-token", gotAuth)
	}
	if !strings.Contains(gotDPoP, ".") {
		t.Fatal("DPoP proof is missing")
	}
}
