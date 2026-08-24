package atproto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type authorizationState struct {
	State         string
	CodeVerifier  string
	CodeChallenge string
}

func newAuthorizationState() (authorizationState, error) {
	state, err := randomBase64URL(32)
	if err != nil {
		return authorizationState{}, fmt.Errorf("failed to generate state: %w", err)
	}

	verifier, err := randomBase64URL(32)
	if err != nil {
		return authorizationState{}, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	challenge := pkceChallenge(verifier)

	return authorizationState{
		State:         state,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}, nil
}

func randomBase64URL(size int) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(
		hash[:],
	)
}
