package sessionstore

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webxsid/pdg/internal/protocol/atproto"
)

type fakeCredentials struct {
	values   map[string]atproto.Credentials
	failSave bool
}

func (f *fakeCredentials) Save(_ context.Context, did string, c atproto.Credentials) error {
	if f.failSave {
		return errors.New("save failed")
	}
	if f.values == nil {
		f.values = map[string]atproto.Credentials{}
	}
	f.values[did] = c
	return nil
}
func (f *fakeCredentials) Load(_ context.Context, did string) (atproto.Credentials, error) {
	c, ok := f.values[did]
	if !ok {
		return atproto.Credentials{}, ErrCredentialsNotFound
	}
	return c, nil
}
func (f *fakeCredentials) Delete(_ context.Context, did string) error {
	delete(f.values, did)
	return nil
}

func TestAuthStoreMultipleAccountsAndMetadataSecurity(t *testing.T) {
	fake := &fakeCredentials{}
	root := t.TempDir()
	store := NewAuthStore(root, fake)
	ctx := context.Background()
	a := testSession(t, "did:plc:aaa", "aaa.example.com")
	b := testSession(t, "did:plc:bbb", "bbb.example.com")
	if err := store.SaveSession(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(ctx, b); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "auth", "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secretField := range []string{"access_token", "refresh_token", "dpop_private_key"} {
		if strings.Contains(string(data), secretField) {
			t.Errorf("metadata contains secret field %q", secretField)
		}
	}
	accounts, err := store.ListAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || !accounts[1].Active {
		t.Errorf("ListAccounts() = %+v, want two accounts with second active", accounts)
	}
	if err := store.SetActive(ctx, a.Identity.DID); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadActiveSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Identity.DID != a.Identity.DID {
		t.Errorf("LoadActiveSession().DID = %q, want %q", loaded.Identity.DID, a.Identity.DID)
	}
	if err := store.DeleteSession(ctx, a.Identity.DID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadSession(ctx, a.Identity.DID); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("LoadSession(deleted) error = %v, want ErrAccountNotFound", err)
	}
	loaded, err = store.LoadActiveSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Identity.DID != b.Identity.DID {
		t.Errorf("LoadActiveSession() after deleting active = %q, want %q", loaded.Identity.DID, b.Identity.DID)
	}
	info, err := os.Stat(filepath.Join(t.TempDir(), "auth", "accounts.json"))
	_ = info
	_ = err
}

func TestAuthStoreMissingMetadataAndCredentialFailure(t *testing.T) {
	fake := &fakeCredentials{}
	store := NewAuthStore(t.TempDir(), fake)
	if _, err := store.LoadActiveSession(context.Background()); err == nil {
		t.Error("LoadActiveSession(missing) error = nil, want error")
	}
	fake.failSave = true
	if err := store.SaveSession(context.Background(), testSession(t, "did:plc:aaa", "aaa.example.com")); err == nil {
		t.Error("SaveSession(failing credentials) error = nil, want error")
	}
}

func testSession(t *testing.T, did, handle string) atproto.Session {
	t.Helper()
	return newSession(t, did, handle)
}

func newSession(t *testing.T, did, handle string) atproto.Session {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	session, err := atproto.NewSession(atproto.SessionMetadata{
		Identity: atproto.Identity{DID: did, Handle: handle, PDS: "https://pds.example.com"},
		Server:   atproto.AuthorizationServer{Issuer: "https://auth.example.com", AuthorizationEndpoint: "https://auth.example.com/oauth/authorize", TokenEndpoint: "https://auth.example.com/oauth/token", PushedAuthorizationRequestEndpoint: "https://auth.example.com/oauth/par"},
		ClientID: "http://localhost",
	}, atproto.Credentials{AccessToken: "access", RefreshToken: "refresh", TokenType: "DPoP", Scope: "atproto", DPoPNonce: "nonce", DPoPPrivateKey: keyBytes})
	if err != nil {
		t.Fatal(err)
	}
	return session
}
