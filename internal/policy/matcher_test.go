package policy

import "testing"

func TestMatchAction(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		patterns []string
		want     bool
	}{
		{
			name:     "exact match",
			action:   "s3:GetObject",
			patterns: []string{"s3:GetObject"},
			want:     true,
		},
		{
			name:     "wildcard match all",
			action:   "s3:GetObject",
			patterns: []string{"s3:*"},
			want:     true,
		},
		{
			name:     "wildcard match prefix",
			action:   "s3:GetObject",
			patterns: []string{"s3:Get*"},
			want:     true,
		},
		{
			name:     "no match",
			action:   "s3:GetObject",
			patterns: []string{"s3:PutObject"},
			want:     false,
		},
		{
			name:     "multiple patterns one matches",
			action:   "s3:GetObject",
			patterns: []string{"s3:PutObject", "s3:GetObject", "s3:DeleteObject"},
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			action:   "s3:ListBucket",
			patterns: []string{"s3:PutObject", "s3:GetObject"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchAction(tt.action, tt.patterns)
			if got != tt.want {
				t.Errorf("MatchAction(%q, %v) = %v, want %v", tt.action, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestMatchResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		patterns []string
		want     bool
	}{
		{
			name:     "exact bucket match",
			resource: "arn:aws:s3:::mybucket",
			patterns: []string{"arn:aws:s3:::mybucket"},
			want:     true,
		},
		{
			name:     "bucket with key exact match",
			resource: "arn:aws:s3:::mybucket/path/to/object",
			patterns: []string{"arn:aws:s3:::mybucket/path/to/object"},
			want:     true,
		},
		{
			name:     "wildcard bucket match",
			resource: "arn:aws:s3:::tenant-001-data",
			patterns: []string{"arn:aws:s3:::tenant-001-*"},
			want:     true,
		},
		{
			name:     "wildcard key match",
			resource: "arn:aws:s3:::mybucket/any/path/file.txt",
			patterns: []string{"arn:aws:s3:::mybucket/*"},
			want:     true,
		},
		{
			name:     "cross-tenant no match",
			resource: "arn:aws:s3:::tenant-002-data/file.txt",
			patterns: []string{"arn:aws:s3:::tenant-001-*", "arn:aws:s3:::tenant-001-*/*"},
			want:     false,
		},
		{
			name:     "full wildcard match",
			resource: "arn:aws:s3:::anybucket/anykey",
			patterns: []string{"arn:aws:s3:::*/*"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchResource(tt.resource, tt.patterns)
			if got != tt.want {
				t.Errorf("MatchResource(%q, %v) = %v, want %v", tt.resource, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestMatchScope(t *testing.T) {
	tests := []struct {
		name   string
		bucket string
		scopes []string
		want   bool
	}{
		{
			name:   "exact match",
			bucket: "tenant-001-data",
			scopes: []string{"tenant-001-data"},
			want:   true,
		},
		{
			name:   "wildcard match",
			bucket: "tenant-001-uploads",
			scopes: []string{"tenant-001-*"},
			want:   true,
		},
		{
			name:   "no match different tenant",
			bucket: "tenant-002-data",
			scopes: []string{"tenant-001-*"},
			want:   false,
		},
		{
			name:   "multiple scopes one matches",
			bucket: "shared-bucket",
			scopes: []string{"tenant-001-*", "shared-bucket"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchScope(tt.bucket, tt.scopes)
			if got != tt.want {
				t.Errorf("MatchScope(%q, %v) = %v, want %v", tt.bucket, tt.scopes, got, tt.want)
			}
		})
	}
}

func TestBuildResourceARN(t *testing.T) {
	tests := []struct {
		bucket string
		key    string
		want   string
	}{
		{"mybucket", "", "arn:aws:s3:::mybucket"},
		{"mybucket", "mykey", "arn:aws:s3:::mybucket/mykey"},
		{"mybucket", "path/to/object", "arn:aws:s3:::mybucket/path/to/object"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := BuildResourceARN(tt.bucket, tt.key)
			if got != tt.want {
				t.Errorf("BuildResourceARN(%q, %q) = %q, want %q", tt.bucket, tt.key, got, tt.want)
			}
		})
	}
}

func TestParseResourceARN(t *testing.T) {
	tests := []struct {
		arn        string
		wantBucket string
		wantKey    string
		wantOK     bool
	}{
		{"arn:aws:s3:::mybucket", "mybucket", "", true},
		{"arn:aws:s3:::mybucket/mykey", "mybucket", "mykey", true},
		{"arn:aws:s3:::mybucket/path/to/object", "mybucket", "path/to/object", true},
		{"invalid-arn", "", "", false},
		{"arn:aws:ec2:::something", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.arn, func(t *testing.T) {
			bucket, key, ok := ParseResourceARN(tt.arn)
			if ok != tt.wantOK {
				t.Errorf("ParseResourceARN(%q) ok = %v, want %v", tt.arn, ok, tt.wantOK)
			}
			if bucket != tt.wantBucket {
				t.Errorf("ParseResourceARN(%q) bucket = %q, want %q", tt.arn, bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("ParseResourceARN(%q) key = %q, want %q", tt.arn, key, tt.wantKey)
			}
		})
	}
}
