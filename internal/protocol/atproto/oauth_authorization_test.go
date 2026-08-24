package atproto

import (
	"strings"
	"testing"
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
