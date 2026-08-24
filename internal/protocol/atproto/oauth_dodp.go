package atproto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type dpopKey struct {
	privateKey *ecdsa.PrivateKey
}

type ecJWK struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func newDPoPKey() (*dpopKey, error) {
	key, err := ecdsa.GenerateKey(
		elliptic.P256(),
		rand.Reader,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DPoP key: %w", err)
	}

	return &dpopKey{privateKey: key}, nil
}

func (k *dpopKey) getJWK() *ecJWK {
	pub := k.privateKey.PublicKey
	encoded, err := pub.Bytes()
	if err != nil {
		fmt.Printf("failed to encode public key: %v\n", err)
		return nil
	}

	const coordinateSize = 32

	x := encoded[1 : 1+coordinateSize]
	y := encoded[1+coordinateSize:]

	return &ecJWK{
		KTY: "EC",
		CRV: "P-256",
		X:   base64.RawStdEncoding.EncodeToString(x),
		Y:   base64.RawStdEncoding.EncodeToString(y),
	}
}

func (k *dpopKey) publicJWK() (ecJWK, error) {
	encoded, err := k.privateKey.PublicKey.Bytes()
	if err != nil {
		return ecJWK{}, fmt.Errorf(
			"failed to encode public key: %w",
			err,
		)
	}

	const (
		coordinateSize = 32
		encodedSize    = 1 + coordinateSize*2
	)

	if len(encoded) != encodedSize || encoded[0] != 0x04 {
		return ecJWK{}, fmt.Errorf(
			"unexpected P-256 public key encoding",
		)
	}

	x := encoded[1 : 1+coordinateSize]
	y := encoded[1+coordinateSize:]

	return ecJWK{
		KTY: "EC",
		CRV: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(x),
		Y:   base64.RawURLEncoding.EncodeToString(y),
	}, nil
}
