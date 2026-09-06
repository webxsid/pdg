package cli

import "testing"

func TestValidateSiteURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "https", url: "https://example.com", want: true},
		{name: "http", url: "http://localhost:8080/site", want: true},
		{name: "missing scheme", url: "example.com", want: false},
		{name: "missing host", url: "https:///site", want: false},
		{name: "unsupported scheme", url: "ftp://example.com", want: false},
		{name: "userinfo", url: "https://user@example.com", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSiteURL(tt.url)
			if (err == nil) != tt.want {
				t.Errorf("validateSiteURL(%q) error = %v, want valid = %t", tt.url, err, tt.want)
			}
		})
	}
}
