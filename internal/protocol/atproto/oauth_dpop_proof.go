package atproto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type dpopProofHeader struct {
	Typ string `json:"typ"`
	Alg string `json:"alg"`
	JWK ecJWK  `json:"jwk"`
}

type dpopProofClaims struct {
	JTI   string `json:"jti"`
	HTM   string `json:"htm"`
	HTU   string `json:"htu"`
	IAT   int64  `json:"iat"`
	Nonce string `json:"nonce,omitempty"`
	Ath   string `json:"ath,omitempty"`
}

func (k *dpopKey) proof(
	method string,
	targetURL string,
	nonce string,
) (string, error) {
	jti, err := randomBase64URL(32)
	if err != nil {
		return "", fmt.Errorf(
			"failed to generate jti: %w",
			err,
		)
	}

	jwk, err := k.publicJWK()
	if err != nil {
		return "", fmt.Errorf(
			"failed to get public JWK: %w",
			err,
		)
	}

	header := dpopProofHeader{
		Typ: "dpop+jwt",
		Alg: "ES256",
		JWK: jwk,
	}

	claims := dpopProofClaims{
		JTI:   jti,
		HTM:   method,
		HTU:   targetURL,
		IAT:   nowUnix(),
		Nonce: nonce,
	}

	return k.signProof(header, claims)
}

func (k *dpopKey) resourceProof(method, targetURL, nonce, accessToken string) (string, error) {
	jti, err := randomBase64URL(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate jti: %w", err)
	}
	jwk, err := k.publicJWK()
	if err != nil {
		return "", fmt.Errorf("failed to get public JWK: %w", err)
	}
	digest := sha256.Sum256([]byte(accessToken))
	return k.signProof(dpopProofHeader{Typ: "dpop+jwt", Alg: "ES256", JWK: jwk}, dpopProofClaims{
		JTI: jti, HTM: method, HTU: targetURL, IAT: nowUnix(), Nonce: nonce,
		Ath: base64.RawURLEncoding.EncodeToString(digest[:]),
	})
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func (k *dpopKey) signProof(
	header dpopProofHeader,
	claims dpopProofClaims,
) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf(
			"marshal DPoP header: %w",
			err,
		)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf(
			"marshal DPoP claims: %w",
			err,
		)
	}

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput :=
		headerEncoded + "." + claimsEncoded

	digest := sha256.Sum256(
		[]byte(signingInput),
	)

	r, s, err := ecdsa.Sign(
		rand.Reader,
		k.privateKey,
		digest[:],
	)
	if err != nil {
		return "", fmt.Errorf(
			"sign DPoP proof: %w",
			err,
		)
	}

	signature := make([]byte, 64)

	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])

	return signingInput + "." +
		base64.RawURLEncoding.EncodeToString(signature), nil
}
