package policy

import (
	"regexp"
	"strings"
)

// MatchAction checks if the given action matches any of the action patterns
func MatchAction(action string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(action, pattern) {
			return true
		}
	}
	return false
}

// MatchResource checks if the given resource ARN matches any of the resource patterns
func MatchResource(resource string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(resource, pattern) {
			return true
		}
	}
	return false
}

// MatchScope checks if the bucket matches any of the scope patterns
// Scopes are simpler patterns like "tenant-001-*" for bucket name matching
func MatchScope(bucket string, scopes []string) bool {
	for _, scope := range scopes {
		if matchScopePattern(bucket, scope) {
			return true
		}
	}
	return false
}

// matchPattern matches a string against a pattern with wildcards
// Supports:
// - "*" matches any sequence of characters
// - "?" matches any single character
func matchPattern(str, pattern string) bool {
	// Convert pattern to regex
	regexPattern := patternToRegex(pattern)
	matched, _ := regexp.MatchString("^"+regexPattern+"$", str)
	return matched
}

// matchScopePattern matches a bucket name against a scope pattern
// Scope patterns are like "tenant-001-*" or "shared-bucket/prefix/*"
func matchScopePattern(bucket string, scopePattern string) bool {
	// Handle patterns with path components
	parts := strings.SplitN(scopePattern, "/", 2)
	bucketPattern := parts[0]

	return matchPattern(bucket, bucketPattern)
}

// patternToRegex converts an IAM-style pattern to a regex pattern
func patternToRegex(pattern string) string {
	var result strings.Builder
	for _, ch := range pattern {
		switch ch {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '^', '$', '[', ']', '(', ')', '{', '}', '|', '\\':
			result.WriteRune('\\')
			result.WriteRune(ch)
		default:
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// BuildResourceARN builds an S3 resource ARN from bucket and key
func BuildResourceARN(bucket, key string) string {
	if key == "" {
		return "arn:aws:s3:::" + bucket
	}
	return "arn:aws:s3:::" + bucket + "/" + key
}

// ParseResourceARN parses an S3 resource ARN and returns bucket and key
func ParseResourceARN(arn string) (bucket, key string, ok bool) {
	// ARN format: arn:aws:s3:::bucket or arn:aws:s3:::bucket/key
	prefix := "arn:aws:s3:::"
	if !strings.HasPrefix(arn, prefix) {
		return "", "", false
	}

	rest := strings.TrimPrefix(arn, prefix)
	parts := strings.SplitN(rest, "/", 2)

	bucket = parts[0]
	if len(parts) > 1 {
		key = parts[1]
	}

	return bucket, key, true
}
