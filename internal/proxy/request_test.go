package proxy

import (
	"net/http"
	"net/url"
	"testing"
)

func TestParseS3Request(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		query      string
		wantBucket string
		wantKey    string
		wantAction string
	}{
		{
			name:       "GET object",
			method:     "GET",
			path:       "/mybucket/path/to/object.txt",
			wantBucket: "mybucket",
			wantKey:    "path/to/object.txt",
			wantAction: "s3:GetObject",
		},
		{
			name:       "PUT object",
			method:     "PUT",
			path:       "/mybucket/newfile.txt",
			wantBucket: "mybucket",
			wantKey:    "newfile.txt",
			wantAction: "s3:PutObject",
		},
		{
			name:       "DELETE object",
			method:     "DELETE",
			path:       "/mybucket/oldfile.txt",
			wantBucket: "mybucket",
			wantKey:    "oldfile.txt",
			wantAction: "s3:DeleteObject",
		},
		{
			name:       "LIST bucket",
			method:     "GET",
			path:       "/mybucket",
			query:      "list-type=2",
			wantBucket: "mybucket",
			wantKey:    "",
			wantAction: "s3:ListBucket",
		},
		{
			name:       "LIST bucket with prefix",
			method:     "GET",
			path:       "/mybucket",
			query:      "prefix=folder/",
			wantBucket: "mybucket",
			wantKey:    "",
			wantAction: "s3:ListBucket",
		},
		{
			name:       "HEAD object",
			method:     "HEAD",
			path:       "/mybucket/file.txt",
			wantBucket: "mybucket",
			wantKey:    "file.txt",
			wantAction: "s3:GetObject",
		},
		{
			name:       "GET object ACL",
			method:     "GET",
			path:       "/mybucket/file.txt",
			query:      "acl",
			wantBucket: "mybucket",
			wantKey:    "file.txt",
			wantAction: "s3:GetObjectAcl",
		},
		{
			name:       "GET bucket ACL",
			method:     "GET",
			path:       "/mybucket",
			query:      "acl",
			wantBucket: "mybucket",
			wantKey:    "",
			wantAction: "s3:GetBucketAcl",
		},
		{
			name:       "PUT bucket",
			method:     "PUT",
			path:       "/newbucket",
			wantBucket: "newbucket",
			wantKey:    "",
			wantAction: "s3:CreateBucket",
		},
		{
			name:       "DELETE bucket",
			method:     "DELETE",
			path:       "/oldbucket",
			wantBucket: "oldbucket",
			wantKey:    "",
			wantAction: "s3:DeleteBucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("http://localhost" + tt.path)
			if tt.query != "" {
				u.RawQuery = tt.query
			}

			req := &http.Request{
				Method: tt.method,
				URL:    u,
				Header: make(http.Header),
			}

			s3req, err := ParseS3Request(req)
			if err != nil {
				t.Fatalf("ParseS3Request() error = %v", err)
			}

			if s3req.Bucket != tt.wantBucket {
				t.Errorf("Bucket = %q, want %q", s3req.Bucket, tt.wantBucket)
			}
			if s3req.Key != tt.wantKey {
				t.Errorf("Key = %q, want %q", s3req.Key, tt.wantKey)
			}
			if s3req.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", s3req.Action, tt.wantAction)
			}
		})
	}
}

func TestS3Request_ToARN(t *testing.T) {
	tests := []struct {
		bucket  string
		key     string
		wantARN string
	}{
		{"mybucket", "", "arn:aws:s3:::mybucket"},
		{"mybucket", "file.txt", "arn:aws:s3:::mybucket/file.txt"},
		{"mybucket", "path/to/file.txt", "arn:aws:s3:::mybucket/path/to/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.wantARN, func(t *testing.T) {
			req := &S3Request{
				Bucket: tt.bucket,
				Key:    tt.key,
			}

			got := req.ToARN()
			if got != tt.wantARN {
				t.Errorf("ToARN() = %q, want %q", got, tt.wantARN)
			}
		})
	}
}
