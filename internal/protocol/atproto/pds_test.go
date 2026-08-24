package atproto

import "testing"

func TestValidatePDSEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid endpoint",
			endpoint: "https://pds.example.com",
		},
		{
			name:     "valid endpoint with trailing slash",
			endpoint: "https://pds.example.com/",
		},
		{
			name:     "http",
			endpoint: "http://pds.example.com",
			wantErr:  true,
		},
		{
			name:     "missing scheme",
			endpoint: "pds.example.com",
			wantErr:  true,
		},
		{
			name:     "missing host",
			endpoint: "https://",
			wantErr:  true,
		},
		{
			name:     "path",
			endpoint: "https://pds.example.com/xrpc",
			wantErr:  true,
		},
		{
			name:     "query",
			endpoint: "https://pds.example.com?foo=bar",
			wantErr:  true,
		},
		{
			name:     "fragment",
			endpoint: "https://pds.example.com#foo",
			wantErr:  true,
		},
		{
			name:     "userinfo",
			endpoint: "https://user:pass@pds.example.com",
			wantErr:  true,
		},
		{
			name:     "empty",
			endpoint: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePDSEndpoint(tt.endpoint)

			if tt.wantErr && err == nil {
				t.Fatalf(
					"validatePDSEndpoint(%q) error = nil, want error",
					tt.endpoint,
				)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf(
					"validatePDSEndpoint(%q) unexpected error = %v",
					tt.endpoint,
					err,
				)
			}
		})
	}
}

func TestFindPDS(t *testing.T) {
	document := didDocument{
		Service: []didService{
			{
				ID:              "#something_else",
				Type:            "OtherService",
				ServiceEndpoint: "https://other.example.com",
			},
			{
				ID:              "#atproto_pds",
				Type:            "AtprotoPersonalDataServer",
				ServiceEndpoint: "https://pds.example.com/",
			},
		},
	}

	got, err := findPDS(document)
	if err != nil {
		t.Fatalf("findPDS() unexpected error = %v", err)
	}

	want := "https://pds.example.com"

	if got != want {
		t.Errorf(
			"findPDS() = %q, want %q",
			got,
			want,
		)
	}
}
