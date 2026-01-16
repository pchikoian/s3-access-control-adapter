package errors

import (
	"encoding/xml"
	"fmt"
	"net/http"
)

// DenyReason represents the reason for denying access
type DenyReason string

const (
	DenyTenantBoundary  DenyReason = "DENY_TENANT_BOUNDARY"
	DenyPolicy          DenyReason = "DENY_POLICY"
	DenyInvalidResource DenyReason = "DENY_INVALID_RESOURCE"
	DenyAuthFailed      DenyReason = "DENY_AUTH_FAILED"
	DenyInternalError   DenyReason = "DENY_INTERNAL_ERROR"
)

// AccessDeniedError represents an access denied error
type AccessDeniedError struct {
	Reason    DenyReason
	Message   string
	Resource  string
	RequestID string
}

func (e *AccessDeniedError) Error() string {
	return fmt.Sprintf("%s: %s", e.Reason, e.Message)
}

// NewAccessDeniedError creates a new access denied error
func NewAccessDeniedError(reason DenyReason, message, resource, requestID string) *AccessDeniedError {
	return &AccessDeniedError{
		Reason:    reason,
		Message:   message,
		Resource:  resource,
		RequestID: requestID,
	}
}

// S3Error represents an S3 XML error response
type S3Error struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId"`
}

// ToS3Error converts an AccessDeniedError to an S3Error
func (e *AccessDeniedError) ToS3Error() *S3Error {
	code := "AccessDenied"
	message := "Access Denied"

	switch e.Reason {
	case DenyTenantBoundary:
		message = "Access denied: resource is outside your tenant boundary"
	case DenyPolicy:
		message = "Access denied: action not permitted by policy"
	case DenyInvalidResource:
		code = "InvalidRequest"
		message = "Invalid resource"
	case DenyAuthFailed:
		code = "SignatureDoesNotMatch"
		message = "The request signature we calculated does not match the signature you provided"
	case DenyInternalError:
		code = "InternalError"
		message = "We encountered an internal error. Please try again."
	}

	return &S3Error{
		Code:      code,
		Message:   message,
		Resource:  e.Resource,
		RequestID: e.RequestID,
	}
}

// HTTPStatusCode returns the appropriate HTTP status code
func (e *AccessDeniedError) HTTPStatusCode() int {
	switch e.Reason {
	case DenyAuthFailed:
		return http.StatusForbidden
	case DenyTenantBoundary, DenyPolicy:
		return http.StatusForbidden
	case DenyInvalidResource:
		return http.StatusBadRequest
	case DenyInternalError:
		return http.StatusInternalServerError
	default:
		return http.StatusForbidden
	}
}

// WriteS3Error writes an S3 XML error response to the response writer
func WriteS3Error(w http.ResponseWriter, err *AccessDeniedError) {
	s3Err := err.ToS3Error()
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", err.RequestID)
	w.WriteHeader(err.HTTPStatusCode())
	xml.NewEncoder(w).Encode(s3Err)
}

// WriteS3ErrorFromCode writes an S3 error from a code and message
func WriteS3ErrorFromCode(w http.ResponseWriter, statusCode int, code, message, requestID string) {
	s3Err := &S3Error{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", requestID)
	w.WriteHeader(statusCode)
	xml.NewEncoder(w).Encode(s3Err)
}
