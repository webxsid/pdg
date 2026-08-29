package atproto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"testing"
)

func TestDPoPPrivateKeyRoundTrip(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	data, err := key.marshalPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := parseDPoPPrivateKey(data)
	if err != nil {
		t.Fatal(err)
	}
	want, err := key.publicJWK()
	if err != nil {
		t.Fatal(err)
	}
	got, err := restored.publicJWK()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("restored public JWK = %+v, want %+v", got, want)
	}
}

func TestParseDPoPPrivateKeyRejectsInvalidKeys(t *testing.T) {
	if _, err := parseDPoPPrivateKey([]byte("invalid")); err == nil {
		t.Error("parseDPoPPrivateKey(invalid) error = nil, want error")
	}
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	data, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseDPoPPrivateKey(data); err == nil {
		t.Error("parseDPoPPrivateKey(P-384) error = nil, want error")
	}
}
