package shared

import (
	"fmt"
	"net/url"
)

func ValidateHTTPSEndpoint(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("must use HTTPS")
	}

	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}

	if parsed.User != nil {
		return fmt.Errorf("userinfo is not allowed")
	}

	if parsed.Fragment != "" {
		return fmt.Errorf("fragment is not allowed")
	}

	return nil
}
