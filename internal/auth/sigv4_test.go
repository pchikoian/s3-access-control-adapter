package auth

import "testing"

func TestParseAuthHeader(t *testing.T) {
	validator := NewSignatureValidator()

	tests := []struct {
		name       string
		header     string
		wantAccess string
		wantDate   string
		wantRegion string
		wantErr    bool
	}{
		{
			name:       "valid header",
			header:     "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=abcdef1234567890",
			wantAccess: "AKIAIOSFODNN7EXAMPLE",
			wantDate:   "20130524",
			wantRegion: "us-east-1",
			wantErr:    false,
		},
		{
			name:       "different region",
			header:     "AWS4-HMAC-SHA256 Credential=AKIAI44QH8DHBEXAMPLE/20240115/eu-west-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=fedcba0987654321",
			wantAccess: "AKIAI44QH8DHBEXAMPLE",
			wantDate:   "20240115",
			wantRegion: "eu-west-1",
			wantErr:    false,
		},
		{
			name:    "invalid format",
			header:  "Basic dXNlcjpwYXNz",
			wantErr: true,
		},
		{
			name:    "empty header",
			header:  "",
			wantErr: true,
		},
		{
			name:    "partial header",
			header:  "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components, err := validator.ParseAuthHeader(tt.header)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if components.AccessKey != tt.wantAccess {
				t.Errorf("AccessKey = %q, want %q", components.AccessKey, tt.wantAccess)
			}
			if components.Date != tt.wantDate {
				t.Errorf("Date = %q, want %q", components.Date, tt.wantDate)
			}
			if components.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", components.Region, tt.wantRegion)
			}
		})
	}
}

func TestHashSHA256(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := hashSHA256([]byte(tt.input))
			if got != tt.want {
				t.Errorf("hashSHA256(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
