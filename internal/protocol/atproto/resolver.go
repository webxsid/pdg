package atproto

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

type didDocument struct {
	ID          string       `json:"id"`
	AlsoKnownAs []string     `json:"alsoKnownAs"`
	Service     []didService `json:"service"`
}

type didService struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

type dnsResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type Resolver struct {
	client      *http.Client
	dnsResolver dnsResolver
}

func newResolver(client *http.Client) *Resolver {
	if client == nil {
		client = http.DefaultClient
	}
	return &Resolver{
		client:      client,
		dnsResolver: net.DefaultResolver,
	}
}

func (r *Resolver) resolve(
	ctx context.Context,
	handle string,
) (Identity, error) {
	handle = normalizeHandle(handle)

	did, err := r.resolveHandle(
		ctx,
		handle,
	)
	if err != nil {
		return Identity{}, fmt.Errorf("failed to resolve handle: %w", err)
	}

	pds := ""
	document, err := r.resolveDID(
		ctx,
		did,
	)
	if err != nil {
		return Identity{}, fmt.Errorf("failed to resolve DID: %w", err)
	}

	if err := verifyIdentity(
		handle,
		did,
		document,
	); err != nil {
		return Identity{}, fmt.Errorf("failed to verify identity: %w", err)
	}

	pds, err = findPDS(document)
	if err != nil {
		return Identity{}, fmt.Errorf("failed to find PDS: %w", err)
	}

	return Identity{
		Handle: handle,
		DID:    did,
		PDS:    pds,
	}, nil
}
