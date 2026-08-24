package atproto

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func findPDS(document didDocument) (string, error) {
	for _, service := range document.Service {

		if service.Type != "AtprotoPersonalDataServer" {
			continue
		}

		if !strings.HasSuffix(
			service.ID,
			"#atproto_pds",
		) {
			continue
		}

		if err := validatePDSEndpoint(service.ServiceEndpoint); err != nil {
			continue
		}

		return strings.TrimSuffix(
			service.ServiceEndpoint,
			"/",
		), nil

	}

	return "", fmt.Errorf(
		"AtprotoPersonalDataServer service not found in DID document",
	)
}

func validatePDSEndpoint(pds string) error {
	parsed, err := url.Parse(pds)
	if err != nil {
		return fmt.Errorf(
			"failed to parse PDS endpoint: %w",
			err,
		)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf(
			"PDS endpoint must use https scheme: %s",
			pds,
		)
	}

	if parsed.Host == "" {
		return fmt.Errorf(
			"PDS endpoint must have a valid host: %s",
			pds,
		)
	}

	if parsed.User != nil {
		return fmt.Errorf(
			"PDS endpoint must not contain user info: %s",
			pds,
		)
	}

	if parsed.RawQuery != "" {
		return fmt.Errorf(
			"PDS endpoint must not contain query parameters: %s",
			pds,
		)
	}

	if parsed.Fragment != "" {
		return fmt.Errorf(
			"PDS endpoint must not contain fragment: %s",
			pds,
		)
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf(
			"PDS endpoint must not contain path: %s",
			pds,
		)
	}

	if net.ParseIP(parsed.Hostname()) == nil {
		host := parsed.Hostname()

		if host == "" || strings.ContainsAny(host, " \t\n\r") {
			return fmt.Errorf(
				"PDS endpoint must have a valid hostname: %s",
				pds,
			)
		}
	}

	return nil
}
