package sessionstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/webxsid/pdg/internal/protocol/atproto"
	"github.com/zalando/go-keyring"
)

const serviceName = "projectdg"

var (
	ErrNoActiveAccount            = errors.New("no active account")
	ErrAccountNotFound            = errors.New("account not found")
	ErrCredentialsNotFound        = errors.New("credentials not found")
	ErrCredentialStoreUnavailable = errors.New("secure credential storage is unavailable")
)

// CredentialStore stores secret material in a secure backend.
type CredentialStore interface {
	Save(context.Context, string, atproto.Credentials) error
	Load(context.Context, string) (atproto.Credentials, error)
	Delete(context.Context, string) error
}

// NativeCredentialStore uses the OS keychain, Credential Manager, or Secret Service.
type NativeCredentialStore struct{}
type credentialEnvelope struct {
	Version        int    `json:"version"`
	AccessToken    string `json:"access_token"`
	RefreshToken   string `json:"refresh_token"`
	TokenType      string `json:"token_type"`
	DPoPPrivateKey string `json:"dpop_private_key"`
}

func (NativeCredentialStore) Save(ctx context.Context, did string, c atproto.Credentials) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(credentialEnvelope{1, c.AccessToken, c.RefreshToken, c.TokenType, base64.StdEncoding.EncodeToString(c.DPoPPrivateKey)})
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	if err := keyring.Set(serviceName, did, string(data)); err != nil {
		return fmt.Errorf("%w: %v", ErrCredentialStoreUnavailable, err)
	}
	return nil
}
func (NativeCredentialStore) Load(ctx context.Context, did string) (atproto.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return atproto.Credentials{}, err
	}
	value, err := keyring.Get(serviceName, did)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return atproto.Credentials{}, fmt.Errorf("%w: %s", ErrCredentialsNotFound, did)
		}
		return atproto.Credentials{}, fmt.Errorf("%w: %v", ErrCredentialStoreUnavailable, err)
	}
	var e credentialEnvelope
	if err := json.Unmarshal([]byte(value), &e); err != nil {
		return atproto.Credentials{}, fmt.Errorf("decode credentials for %s: %w", did, err)
	}
	if e.Version != 1 {
		return atproto.Credentials{}, fmt.Errorf("unsupported credential version: %d", e.Version)
	}
	key, err := base64.StdEncoding.DecodeString(e.DPoPPrivateKey)
	if err != nil {
		return atproto.Credentials{}, fmt.Errorf("decode DPoP private key for %s: %w", did, err)
	}
	return atproto.Credentials{AccessToken: e.AccessToken, RefreshToken: e.RefreshToken, TokenType: e.TokenType, DPoPPrivateKey: key}, nil
}
func (NativeCredentialStore) Delete(ctx context.Context, did string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := keyring.Delete(serviceName, did); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("%w: %v", ErrCredentialStoreUnavailable, err)
	}
	return nil
}

// AccountSummary contains non-secret account metadata.
type AccountSummary struct {
	DID    string
	Handle string
	PDS    string
	Active bool
}
type accountMetadata struct {
	Identity  atproto.Identity            `json:"identity"`
	Server    atproto.AuthorizationServer `json:"authorization_server"`
	ClientID  string                      `json:"client_id"`
	TokenType string                      `json:"token_type"`
	Scope     string                      `json:"scope"`
	DPoPNonce string                      `json:"dpop_nonce"`
}
type metadataIndex struct {
	Version   int                        `json:"version"`
	ActiveDID string                     `json:"active_did,omitempty"`
	Accounts  map[string]accountMetadata `json:"accounts"`
}

// AuthStore combines native credentials with non-secret account metadata.
type AuthStore struct {
	metadataPath string
	credentials  CredentialStore
}

func NewAuthStore(root string, credentials CredentialStore) *AuthStore {
	return &AuthStore{filepath.Join(root, "auth", "accounts.json"), credentials}
}
func DefaultAuthStore() (*AuthStore, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}
	return NewAuthStore(filepath.Join(root, "pdg"), NativeCredentialStore{}), nil
}

func (s *AuthStore) SaveSession(ctx context.Context, session atproto.Session) error {
	metadata := session.Metadata()
	credentials, err := session.Credentials()
	if err != nil {
		return fmt.Errorf("prepare credentials: %w", err)
	}
	previous, previousErr := s.credentials.Load(ctx, metadata.Identity.DID)
	hadPrevious := previousErr == nil
	if err := s.credentials.Save(ctx, metadata.Identity.DID, credentials); err != nil {
		return err
	}
	index, err := s.read()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.restorePrevious(ctx, metadata.Identity.DID, previous, hadPrevious)
		return err
	}
	if index.Accounts == nil {
		index = metadataIndex{Version: 1, Accounts: map[string]accountMetadata{}}
	}
	index.Version = 1
	index.Accounts[metadata.Identity.DID] = accountMetadata{metadata.Identity, metadata.Server, metadata.ClientID, session.TokenType, session.Scope, session.DPoPNonce}
	index.ActiveDID = metadata.Identity.DID
	if err := s.write(index); err != nil {
		s.restorePrevious(ctx, metadata.Identity.DID, previous, hadPrevious)
		return fmt.Errorf("save account metadata: %w", err)
	}
	return nil
}

func (s *AuthStore) restorePrevious(ctx context.Context, did string, credentials atproto.Credentials, existed bool) {
	if existed {
		_ = s.credentials.Save(ctx, did, credentials)
		return
	}
	_ = s.credentials.Delete(ctx, did)
}
func (s *AuthStore) LoadSession(ctx context.Context, did string) (atproto.Session, error) {
	index, err := s.read()
	if err != nil {
		return atproto.Session{}, err
	}
	metadata, ok := index.Accounts[did]
	if !ok {
		return atproto.Session{}, fmt.Errorf("load %s: %w", did, ErrAccountNotFound)
	}
	credentials, err := s.credentials.Load(ctx, did)
	if err != nil {
		return atproto.Session{}, err
	}
	credentials.Scope = metadata.Scope
	credentials.DPoPNonce = metadata.DPoPNonce
	credentials.TokenType = metadata.TokenType
	session, err := atproto.NewSession(atproto.SessionMetadata{Identity: metadata.Identity, Server: metadata.Server, ClientID: metadata.ClientID}, credentials)
	if err != nil {
		return atproto.Session{}, fmt.Errorf("restore %s: %w", did, err)
	}
	return session, nil
}
func (s *AuthStore) LoadActiveSession(ctx context.Context) (atproto.Session, error) {
	index, err := s.read()
	if err != nil {
		return atproto.Session{}, err
	}
	if index.ActiveDID == "" {
		return atproto.Session{}, ErrNoActiveAccount
	}
	return s.LoadSession(ctx, index.ActiveDID)
}
func (s *AuthStore) SetActive(ctx context.Context, did string) error {
	index, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := index.Accounts[did]; !ok {
		return fmt.Errorf("set active %s: %w", did, ErrAccountNotFound)
	}
	index.ActiveDID = did
	return s.write(index)
}
func (s *AuthStore) DeleteSession(ctx context.Context, did string) error {
	index, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := index.Accounts[did]; !ok {
		return fmt.Errorf("delete %s: %w", did, ErrAccountNotFound)
	}
	if err := s.credentials.Delete(ctx, did); err != nil {
		return err
	}
	delete(index.Accounts, did)
	if index.ActiveDID == did {
		index.ActiveDID = nextActiveDID(index.Accounts)
	}
	return s.write(index)
}

func nextActiveDID(accounts map[string]accountMetadata) string {
	keys := make([]string, 0, len(accounts))
	for did := range accounts {
		keys = append(keys, did)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}
func (s *AuthStore) ListAccounts(ctx context.Context) ([]AccountSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	index, err := s.read()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(index.Accounts))
	for did := range index.Accounts {
		keys = append(keys, did)
	}
	sort.Strings(keys)
	result := make([]AccountSummary, 0, len(keys))
	for _, did := range keys {
		a := index.Accounts[did]
		result = append(result, AccountSummary{a.Identity.DID, a.Identity.Handle, a.Identity.PDS, did == index.ActiveDID})
	}
	return result, nil
}
func (s *AuthStore) read() (metadataIndex, error) {
	data, err := os.ReadFile(s.metadataPath)
	if err != nil {
		return metadataIndex{}, fmt.Errorf("read account metadata: %w", err)
	}
	var index metadataIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return metadataIndex{}, fmt.Errorf("decode account metadata: %w", err)
	}
	if index.Version != 1 {
		return metadataIndex{}, fmt.Errorf("unsupported account metadata version: %d", index.Version)
	}
	if index.Accounts == nil {
		index.Accounts = map[string]accountMetadata{}
	}
	return index, nil
}
func (s *AuthStore) write(index metadataIndex) error {
	dir := filepath.Dir(s.metadataPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create account metadata directory: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("encode account metadata: %w", err)
	}
	file, err := os.CreateTemp(dir, ".accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("create account metadata temporary file: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.metadataPath)
}
