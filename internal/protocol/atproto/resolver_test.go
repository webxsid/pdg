package atproto

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockDNSResolver struct {
	records map[string][]string
}

func (m *mockDNSResolver) LookupTXT(
	_ context.Context,
	name string,
) ([]string, error) {
	records, ok := m.records[name]
	if !ok {
		return nil, fmt.Errorf("DNS record not found")
	}

	return records, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(
	req *http.Request,
) (*http.Response, error) {
	return f(req)
}

func TestIdentityResolverResolve(t *testing.T) {
	const (
		handle = "sid.example.com"
		did    = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"
		pds    = "https://pds.example.com"
	)

	dns := &mockDNSResolver{
		records: map[string][]string{
			"_atproto." + handle: {
				"did=" + did,
			},
		},
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case "https://plc.directory/" + did:
					body := fmt.Sprintf(
						`{
						"id": %q,
						"alsoKnownAs": [
							%q
						],
						"service": [
							{
								"id": "#atproto_pds",
								"type": "AtprotoPersonalDataServer",
								"serviceEndpoint": %q
							}
						]
					}`,
						did,
						"at://"+handle,
						pds,
					)

					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Body: io.NopCloser(
							strings.NewReader(body),
						),
						Header: make(http.Header),
					}, nil

				default:
					return nil, fmt.Errorf(
						"unexpected HTTP request: %s",
						req.URL,
					)
				}
			},
		),
	}

	resolver := &Resolver{
		client:      httpClient,
		dnsResolver: dns,
	}

	got, err := resolver.resolve(
		context.Background(),
		handle,
	)
	if err != nil {
		t.Fatalf(
			"resolve() unexpected error = %v",
			err,
		)
	}

	if got.Handle != handle {
		t.Errorf(
			"Handle = %q, want %q",
			got.Handle,
			handle,
		)
	}

	if got.DID != did {
		t.Errorf(
			"DID = %q, want %q",
			got.DID,
			did,
		)
	}

	if got.PDS != pds {
		t.Errorf(
			"PDS = %q, want %q",
			got.PDS,
			pds,
		)
	}
}

func TestIdentityResolverRejectsUnverifiedHandle(t *testing.T) {
	const (
		handle = "sid.example.com"
		did    = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"
	)

	dns := &mockDNSResolver{
		records: map[string][]string{
			"_atproto." + handle: {
				"did=" + did,
			},
		},
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				body := fmt.Sprintf(`{
					"id": %q,
					"alsoKnownAs": [
						"at://someone-else.example.com"
					],
					"service": [
						{
							"id": "#atproto_pds",
							"type": "AtprotoPersonalDataServer",
							"serviceEndpoint": "https://pds.example.com"
						}
					]
				}`, did)

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body: io.NopCloser(
						strings.NewReader(body),
					),
					Header: make(http.Header),
				}, nil
			},
		),
	}

	resolver := &Resolver{
		client:      httpClient,
		dnsResolver: dns,
	}

	_, err := resolver.resolve(
		context.Background(),
		handle,
	)

	if err == nil {
		t.Fatal(
			"resolve() error = nil, want identity verification error",
		)
	}
}
