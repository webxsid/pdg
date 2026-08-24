package atproto

import "testing"

func TestParseHandleDNSRecords(t *testing.T) {
	tests := []struct {
		name    string
		records []string
		want    string
		wantErr bool
	}{
		{
			name: "valid did plc",
			records: []string{
				"did=did:plc:abc123",
			},
			want: "did:plc:abc123",
		},
		{
			name: "valid did web",
			records: []string{
				"did=did:web:example.com",
			},
			want: "did:web:example.com",
		},
		{
			name: "ignores unrelated records",
			records: []string{
				"google-site-verification=abc123",
				"did=did:plc:abc123",
				"some-other-record=value",
			},
			want: "did:plc:abc123",
		},
		{
			name: "missing did record",
			records: []string{
				"google-site-verification=abc123",
			},
			wantErr: true,
		},
		{
			name:    "empty records",
			records: []string{},
			wantErr: true,
		},
		{
			name: "empty did value",
			records: []string{
				"did=",
			},
			wantErr: true,
		},
		{
			name: "unsupported did method",
			records: []string{
				"did=did:key:abc123",
			},
			wantErr: true,
		},
		{
			name: "multiple did records",
			records: []string{
				"did=did:plc:abc123",
				"did=did:plc:def456",
			},
			wantErr: true,
		},
		{
			name: "multiple identical did records",
			records: []string{
				"did=did:plc:abc123",
				"did=did:plc:abc123",
			},
			wantErr: true,
		},
		{
			name: "does not accept leading whitespace",
			records: []string{
				" did=did:plc:abc123",
			},
			wantErr: true,
		},
		{
			name: "does not accept trailing whitespace",
			records: []string{
				"did=did:plc:abc123 ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHandleDNSRecords(tt.records)

			if tt.wantErr {
				if err == nil {
					t.Fatalf(
						"parseHandleDNSRecords() error = nil, want error",
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"parseHandleDNSRecords() unexpected error = %v",
					err,
				)
			}

			if got != tt.want {
				t.Errorf(
					"parseHandleDNSRecords() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
