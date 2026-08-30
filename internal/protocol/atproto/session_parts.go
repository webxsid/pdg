package atproto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"fmt"
	"time"
)

// SessionMetadata contains non-secret data needed to restore a session.
type SessionMetadata struct {
	Identity             Identity
	Server               AuthorizationServer
	ClientID             string
	AccessTokenExpiresAt *time.Time
}

// Credentials contains secret OAuth material kept by a secure credential store.
type Credentials struct {
	AccessToken    string
	RefreshToken   string
	TokenType      string
	Scope          string
	DPoPNonce      string
	DPoPPrivateKey []byte
}

// Metadata returns the non-secret restoration data for the session.
func (s Session) Metadata() SessionMetadata {
	return SessionMetadata{Identity: s.Identity, Server: s.server, ClientID: s.clientID, AccessTokenExpiresAt: s.AccessTokenExpiresAt}
}

// Credentials returns the secret restoration data for the session.
func (s Session) Credentials() (Credentials, error) {
	key, err := s.dpopKey.marshalPrivateKey()
	if err != nil {
		return Credentials{}, fmt.Errorf("marshal DPoP private key: %w", err)
	}
	return Credentials{AccessToken: s.AccessToken, RefreshToken: s.RefreshToken, TokenType: s.TokenType, Scope: s.Scope, DPoPNonce: s.DPoPNonce, DPoPPrivateKey: key}, nil
}

// NewSession reconstructs a validated runtime session from metadata and credentials.
func NewSession(metadata SessionMetadata, credentials Credentials) (Session, error) {
	if !isSupportedDID(metadata.Identity.DID) {
		return Session{}, fmt.Errorf("invalid session DID")
	}
	if err := validateHandle(metadata.Identity.Handle); err != nil {
		return Session{}, fmt.Errorf("invalid session handle: %w", err)
	}
	if err := validatePDSEndpoint(metadata.Identity.PDS); err != nil {
		return Session{}, fmt.Errorf("invalid session PDS: %w", err)
	}
	if err := validateAuthorizationServer(metadata.Server.Issuer); err != nil {
		return Session{}, fmt.Errorf("invalid session authorization server: %w", err)
	}
	if err := validateAuthorizationEndpoint(metadata.Server.AuthorizationEndpoint); err != nil {
		return Session{}, fmt.Errorf("invalid session authorization endpoint: %w", err)
	}
	if err := validateTokenEndpoint(metadata.Server.TokenEndpoint); err != nil {
		return Session{}, fmt.Errorf("invalid session token endpoint: %w", err)
	}
	if err := validatePAREndpoint(metadata.Server.PushedAuthorizationRequestEndpoint); err != nil {
		return Session{}, fmt.Errorf("invalid session PAR endpoint: %w", err)
	}
	if credentials.AccessToken == "" || credentials.TokenType != "DPoP" || metadata.ClientID == "" {
		return Session{}, fmt.Errorf("session is missing required OAuth fields")
	}
	key, err := parseDPoPPrivateKey(credentials.DPoPPrivateKey)
	if err != nil {
		return Session{}, fmt.Errorf("parse DPoP private key: %w", err)
	}
	return Session{Identity: metadata.Identity, AccessToken: credentials.AccessToken, RefreshToken: credentials.RefreshToken, TokenType: credentials.TokenType, Scope: credentials.Scope, DPoPNonce: credentials.DPoPNonce, AccessTokenExpiresAt: metadata.AccessTokenExpiresAt, dpopKey: key, server: metadata.Server, clientID: metadata.ClientID}, nil
}

func (k *dpopKey) marshalPrivateKey() ([]byte, error) {
	if k == nil || k.privateKey == nil {
		return nil, fmt.Errorf("DPoP private key is nil")
	}
	return x509.MarshalPKCS8PrivateKey(k.privateKey)
}

func parseDPoPPrivateKey(data []byte) (*dpopKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		return nil, err
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ECDSA")
	}
	if ecdsaKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("private key is not P-256")
	}
	return &dpopKey{privateKey: ecdsaKey}, nil
}
