package atproto

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestNewDPoPKey(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatalf(
			"newDPoPKey() unexpected error = %v",
			err,
		)
	}

	if key.privateKey == nil {
		t.Fatal("private key is nil")
	}

	jwk, err := key.publicJWK()
	if err != nil {
		t.Fatalf(
			"publicJWK() unexpected error = %v",
			err,
		)
	}

	if jwk.KTY != "EC" {
		t.Errorf(
			"KTY = %q, want %q",
			jwk.KTY,
			"EC",
		)
	}

	if jwk.CRV != "P-256" {
		t.Errorf(
			"CRV = %q, want %q",
			jwk.CRV,
			"P-256",
		)
	}

	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		t.Fatalf("decode X: %v", err)
	}

	y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		t.Fatalf("decode Y: %v", err)
	}

	if len(x) != 32 {
		t.Errorf(
			"X length = %d, want 32",
			len(x),
		)
	}

	if len(y) != 32 {
		t.Errorf(
			"Y length = %d, want 32",
			len(y),
		)
	}
}

func TestNewDPoPKeyGeneratesUniqueKeys(t *testing.T) {
	a, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	b, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	aJWK, err := a.publicJWK()
	if err != nil {
		t.Fatal(err)
	}

	bJWK, err := b.publicJWK()
	if err != nil {
		t.Fatal(err)
	}

	if aJWK.X == bJWK.X && aJWK.Y == bJWK.Y {
		t.Fatal("generated identical DPoP keys")
	}
}

func TestDPoPProof(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	const target = "https://auth.example.com/oauth/par"

	before := time.Now().Unix()

	proof, err := key.proof("post", target, "")
	if err != nil {
		t.Fatalf("proof() unexpected error = %v", err)
	}

	after := time.Now().Unix()

	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}

	var header dpopProofHeader
	decodeJWTPart(t, parts[0], &header)

	if header.Typ != "dpop+jwt" {
		t.Errorf("typ = %q, want dpop+jwt", header.Typ)
	}

	if header.Alg != "ES256" {
		t.Errorf("alg = %q, want ES256", header.Alg)
	}

	if header.JWK.KTY != "EC" {
		t.Errorf("jwk.kty = %q, want EC", header.JWK.KTY)
	}

	if header.JWK.CRV != "P-256" {
		t.Errorf("jwk.crv = %q, want P-256", header.JWK.CRV)
	}

	var claims dpopProofClaims
	decodeJWTPart(t, parts[1], &claims)

	if claims.JTI == "" {
		t.Error("jti is empty")
	}

	if claims.HTM != "post" {
		t.Errorf("htm = %q, want post", claims.HTM)
	}

	if claims.HTU != target {
		t.Errorf("htu = %q, want %q", claims.HTU, target)
	}

	if claims.IAT < before || claims.IAT > after {
		t.Errorf(
			"iat = %d, want between %d and %d",
			claims.IAT,
			before,
			after,
		)
	}
}

func TestResourceDPoPProofIncludesAccessTokenHash(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	const accessToken = "access-token"
	proof, err := key.resourceProof("GET", "https://pds.example.com/xrpc/com.atproto.server.getSession", "nonce", accessToken)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(proof, ".")
	var claims dpopProofClaims
	decodeJWTPart(t, parts[1], &claims)
	digest := sha256.Sum256([]byte(accessToken))
	want := base64.RawURLEncoding.EncodeToString(digest[:])
	if claims.Ath != want {
		t.Errorf("ath = %q, want %q", claims.Ath, want)
	}
	if claims.Nonce != "nonce" {
		t.Errorf("nonce = %q, want nonce", claims.Nonce)
	}
}

func TestDPoPProofSignature(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := key.proof(
		"POST",
		"https://auth.example.com/oauth/par",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	if len(signature) != 64 {
		t.Fatalf(
			"signature length = %d, want 64",
			len(signature),
		)
	}

	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))

	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	if !ecdsa.Verify(
		&key.privateKey.PublicKey,
		digest[:],
		r,
		s,
	) {
		t.Fatal("DPoP signature verification failed")
	}
}

func TestDPoPProofJWKMatchesSigningKey(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := key.proof(
		"POST",
		"https://auth.example.com/oauth/par",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(proof, ".")

	var header dpopProofHeader
	decodeJWTPart(t, parts[0], &header)

	want, err := key.publicJWK()
	if err != nil {
		t.Fatal(err)
	}

	if header.JWK != want {
		t.Errorf(
			"proof JWK = %+v, want %+v",
			header.JWK,
			want,
		)
	}
}

func TestDPoPProofGeneratesUniqueJTI(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	a, err := key.proof("POST", "https://auth.example.com/oauth/par", "")
	if err != nil {
		t.Fatal(err)
	}

	b, err := key.proof("POST", "https://auth.example.com/oauth/par", "")
	if err != nil {
		t.Fatal(err)
	}

	var claimsA dpopProofClaims
	var claimsB dpopProofClaims

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	decodeJWTPart(t, partsA[1], &claimsA)
	decodeJWTPart(t, partsB[1], &claimsB)

	if claimsA.JTI == claimsB.JTI {
		t.Fatal("DPoP proofs generated identical jti values")
	}
}

func TestDPoPProofIncludesNonce(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := key.proof(
		"POST",
		"https://auth.example.com/oauth/token",
		"test-nonce",
	)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(proof, ".")

	var claims dpopProofClaims
	decodeJWTPart(t, parts[1], &claims)

	if claims.Nonce != "test-nonce" {
		t.Errorf(
			"Nonce = %q, want %q",
			claims.Nonce,
			"test-nonce",
		)
	}
}

func TestDPoPProofOmitsEmptyNonce(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := key.proof(
		"POST",
		"https://auth.example.com/oauth/par",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(proof, ".")

	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	var claims map[string]any

	if err := json.Unmarshal(data, &claims); err != nil {
		t.Fatal(err)
	}

	if _, exists := claims["nonce"]; exists {
		t.Error("nonce claim present when nonce is empty")
	}
}

func decodeJWTPart(
	t *testing.T,
	value string,
	target any,
) {
	t.Helper()

	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode JWT part: %v", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("unmarshal JWT part: %v", err)
	}
}
