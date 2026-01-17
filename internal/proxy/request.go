package proxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/s3-access-control-adapter/internal/policy"
)

// S3Request represents a parsed S3 request
type S3Request struct {
	Bucket        string
	Key           string
	Action        string
	HTTPMethod    string
	Headers       http.Header
	Body          io.ReadCloser
	QueryParams   url.Values
	ContentLength int64
}

// ToARN returns the S3 resource ARN for this request
func (r *S3Request) ToARN() string {
	return policy.BuildResourceARN(r.Bucket, r.Key)
}

// ParseS3Request parses an HTTP request into an S3Request
// Supports path-style URLs: /bucket/key
func ParseS3Request(req *http.Request) (*S3Request, error) {
	bucket, key := parsePath(req.URL.Path)

	s3req := &S3Request{
		Bucket:        bucket,
		Key:           key,
		HTTPMethod:    req.Method,
		Headers:       req.Header.Clone(),
		Body:          req.Body,
		QueryParams:   req.URL.Query(),
		ContentLength: req.ContentLength,
	}

	s3req.Action = determineAction(req.Method, bucket, key, req.URL.Query())

	return s3req, nil
}

// parsePath extracts bucket and key from the URL path
// Path format: /bucket/key/path/to/object
func parsePath(path string) (bucket, key string) {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return "", ""
	}

	parts := strings.SplitN(path, "/", 2)
	bucket = parts[0]

	if len(parts) > 1 {
		key = parts[1]
	}

	return bucket, key
}

// determineAction maps HTTP method and query params to S3 action
func determineAction(method, bucket, key string, query url.Values) string {
	// Check for specific query parameters that indicate special operations
	if query.Has("acl") {
		if method == http.MethodGet {
			if key == "" {
				return "s3:GetBucketAcl"
			}
			return "s3:GetObjectAcl"
		}
		if method == http.MethodPut {
			if key == "" {
				return "s3:PutBucketAcl"
			}
			return "s3:PutObjectAcl"
		}
	}

	if query.Has("versioning") {
		if method == http.MethodGet {
			return "s3:GetBucketVersioning"
		}
		if method == http.MethodPut {
			return "s3:PutBucketVersioning"
		}
	}

	if query.Has("lifecycle") {
		if method == http.MethodGet {
			return "s3:GetLifecycleConfiguration"
		}
		if method == http.MethodPut {
			return "s3:PutLifecycleConfiguration"
		}
		if method == http.MethodDelete {
			return "s3:DeleteLifecycleConfiguration"
		}
	}

	if query.Has("policy") {
		if method == http.MethodGet {
			return "s3:GetBucketPolicy"
		}
		if method == http.MethodPut {
			return "s3:PutBucketPolicy"
		}
		if method == http.MethodDelete {
			return "s3:DeleteBucketPolicy"
		}
	}

	if query.Has("tagging") {
		if method == http.MethodGet {
			if key == "" {
				return "s3:GetBucketTagging"
			}
			return "s3:GetObjectTagging"
		}
		if method == http.MethodPut {
			if key == "" {
				return "s3:PutBucketTagging"
			}
			return "s3:PutObjectTagging"
		}
		if method == http.MethodDelete {
			if key == "" {
				return "s3:DeleteBucketTagging"
			}
			return "s3:DeleteObjectTagging"
		}
	}

	if query.Has("uploads") {
		if method == http.MethodPost {
			return "s3:PutObject" // Initiate multipart upload
		}
		if method == http.MethodGet {
			return "s3:ListBucketMultipartUploads"
		}
	}

	if query.Has("uploadId") {
		if method == http.MethodPut {
			return "s3:PutObject" // Upload part
		}
		if method == http.MethodPost {
			return "s3:PutObject" // Complete multipart upload
		}
		if method == http.MethodDelete {
			return "s3:AbortMultipartUpload"
		}
		if method == http.MethodGet {
			return "s3:ListMultipartUploadParts"
		}
	}

	// Check for list operations (bucket level with no key)
	if key == "" {
		if method == http.MethodGet {
			if query.Has("list-type") || query.Has("prefix") || query.Has("delimiter") {
				return "s3:ListBucket"
			}
			// Plain GET on bucket is also ListBucket
			return "s3:ListBucket"
		}
		if method == http.MethodHead {
			return "s3:ListBucket"
		}
		if method == http.MethodPut {
			return "s3:CreateBucket"
		}
		if method == http.MethodDelete {
			return "s3:DeleteBucket"
		}
	}

	// Object-level operations
	switch method {
	case http.MethodGet:
		return "s3:GetObject"
	case http.MethodHead:
		return "s3:GetObject"
	case http.MethodPut:
		// Check for copy operation
		if _, ok := query["copy"]; ok {
			return "s3:PutObject"
		}
		return "s3:PutObject"
	case http.MethodPost:
		return "s3:PutObject"
	case http.MethodDelete:
		return "s3:DeleteObject"
	default:
		return "s3:Unknown"
	}
}
