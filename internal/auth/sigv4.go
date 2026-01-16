package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SigV4Components holds the parsed components of an AWS Signature V4 Authorization header
type SigV4Components struct {
	AccessKey     string
	Date          string // YYYYMMDD format
	Region        string
	Service       string
	SignedHeaders []string
	Signature     string
}

// AuthContext represents the authenticated request context
type AuthContext struct {
	ClientID  string
	TenantID  string
	AccessKey string
	Policies  []string
	Scopes    []string
	Timestamp time.Time
	RequestID string
}

// SignatureValidator validates AWS Signature V4 requests
type SignatureValidator interface {
	// ParseAndValidate parses the Authorization header and validates the signature
	ParseAndValidate(req *http.Request, credential *Credential) (*SigV4Components, error)
	// ParseAuthHeader extracts components from Authorization header
	ParseAuthHeader(authHeader string) (*SigV4Components, error)
}

// DefaultSignatureValidator implements SignatureValidator
type DefaultSignatureValidator struct{}

// NewSignatureValidator creates a new signature validator
func NewSignatureValidator() *DefaultSignatureValidator {
	return &DefaultSignatureValidator{}
}

// authHeaderRegex matches AWS4-HMAC-SHA256 Authorization header
var authHeaderRegex = regexp.MustCompile(
	`AWS4-HMAC-SHA256\s+` +
		`Credential=([^/]+)/(\d{8})/([^/]+)/([^/]+)/aws4_request,\s*` +
		`SignedHeaders=([^,]+),\s*` +
		`Signature=([a-f0-9]+)`,
)

// ParseAuthHeader parses the AWS Signature V4 Authorization header
func (v *DefaultSignatureValidator) ParseAuthHeader(authHeader string) (*SigV4Components, error) {
	matches := authHeaderRegex.FindStringSubmatch(authHeader)
	if matches == nil {
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	return &SigV4Components{
		AccessKey:     matches[1],
		Date:          matches[2],
		Region:        matches[3],
		Service:       matches[4],
		SignedHeaders: strings.Split(matches[5], ";"),
		Signature:     matches[6],
	}, nil
}

// ParseAndValidate parses and validates the signature
func (v *DefaultSignatureValidator) ParseAndValidate(req *http.Request, credential *Credential) (*SigV4Components, error) {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	components, err := v.ParseAuthHeader(authHeader)
	if err != nil {
		return nil, err
	}

	// Validate the access key matches
	if components.AccessKey != credential.AccessKey {
		return nil, fmt.Errorf("access key mismatch")
	}

	// Get the request timestamp
	amzDate := req.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return nil, fmt.Errorf("missing X-Amz-Date header")
	}

	// Validate timestamp (allow 15 minute clock skew)
	requestTime, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return nil, fmt.Errorf("invalid X-Amz-Date format: %w", err)
	}

	now := time.Now().UTC()
	if requestTime.Before(now.Add(-15*time.Minute)) || requestTime.After(now.Add(15*time.Minute)) {
		return nil, fmt.Errorf("request timestamp is outside allowed window")
	}

	// Compute and verify signature
	expectedSignature, err := v.computeSignature(req, credential.SecretKey, components, amzDate)
	if err != nil {
		return nil, fmt.Errorf("failed to compute signature: %w", err)
	}

	if !hmac.Equal([]byte(expectedSignature), []byte(components.Signature)) {
		return nil, fmt.Errorf("signature mismatch")
	}

	return components, nil
}

// computeSignature computes the AWS Signature V4
func (v *DefaultSignatureValidator) computeSignature(req *http.Request, secretKey string, components *SigV4Components, amzDate string) (string, error) {
	// Step 1: Create canonical request
	canonicalRequest, err := v.createCanonicalRequest(req, components)
	if err != nil {
		return "", err
	}

	// Step 2: Create string to sign
	stringToSign := v.createStringToSign(amzDate, components, canonicalRequest)

	// Step 3: Calculate signature
	signature := v.calculateSignature(secretKey, components.Date, components.Region, components.Service, stringToSign)

	return signature, nil
}

// createCanonicalRequest creates the canonical request string
func (v *DefaultSignatureValidator) createCanonicalRequest(req *http.Request, components *SigV4Components) (string, error) {
	// HTTP method
	method := req.Method

	// Canonical URI (URL-encoded path)
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalURI = escapePath(canonicalURI)

	// Canonical query string
	canonicalQueryString := createCanonicalQueryString(req.URL.Query())

	// Canonical headers
	canonicalHeaders := createCanonicalHeaders(req, components.SignedHeaders)

	// Signed headers
	signedHeaders := strings.Join(components.SignedHeaders, ";")

	// Payload hash
	payloadHash := req.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		// Compute hash of request body
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		payloadHash = hashSHA256(bodyBytes)
	}

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	return canonicalRequest, nil
}

// createStringToSign creates the string to sign
func (v *DefaultSignatureValidator) createStringToSign(amzDate string, components *SigV4Components, canonicalRequest string) string {
	scope := fmt.Sprintf("%s/%s/%s/aws4_request", components.Date, components.Region, components.Service)

	return strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hashSHA256([]byte(canonicalRequest)),
	}, "\n")
}

// calculateSignature calculates the final signature
func (v *DefaultSignatureValidator) calculateSignature(secretKey, date, region, service, stringToSign string) string {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	signature := hmacSHA256(kSigning, []byte(stringToSign))
	return hex.EncodeToString(signature)
}

// createCanonicalHeaders creates the canonical headers string
func createCanonicalHeaders(req *http.Request, signedHeaders []string) string {
	var headers []string
	for _, h := range signedHeaders {
		h = strings.ToLower(h)
		var value string
		if h == "host" {
			value = req.Host
		} else {
			value = req.Header.Get(h)
		}
		// Trim spaces and collapse multiple spaces
		value = strings.TrimSpace(value)
		headers = append(headers, fmt.Sprintf("%s:%s", h, value))
	}
	return strings.Join(headers, "\n") + "\n"
}

// createCanonicalQueryString creates the canonical query string
func createCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		vs := values[k]
		sort.Strings(vs)
		for _, v := range vs {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(pairs, "&")
}

// escapePath URI-encodes the path
func escapePath(path string) string {
	// Split by "/" and encode each segment
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

// hashSHA256 computes SHA256 hash and returns hex string
func hashSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// hmacSHA256 computes HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
