package proxy

import (
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/s3-access-control-adapter/internal/audit"
	"github.com/s3-access-control-adapter/internal/auth"
	"github.com/s3-access-control-adapter/internal/errors"
	"github.com/s3-access-control-adapter/internal/policy"
)

// Gateway is the main HTTP handler for the S3 proxy
type Gateway struct {
	credStore    auth.CredentialStore
	sigValidator auth.SignatureValidator
	policyEngine policy.Engine
	s3Client     *S3Client
	auditLogger  audit.Logger
}

// NewGateway creates a new Gateway
func NewGateway(
	credStore auth.CredentialStore,
	sigValidator auth.SignatureValidator,
	policyEngine policy.Engine,
	s3Client *S3Client,
	auditLogger audit.Logger,
) *Gateway {
	return &Gateway{
		credStore:    credStore,
		sigValidator: sigValidator,
		policyEngine: policyEngine,
		s3Client:     s3Client,
		auditLogger:  auditLogger,
	}
}

// ServeHTTP handles incoming HTTP requests
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestID := uuid.New().String()

	// Add request ID to response headers
	w.Header().Set("x-amz-request-id", requestID)

	// Health check endpoint
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// Parse S3 request
	s3req, err := ParseS3Request(r)
	if err != nil {
		g.handleError(w, requestID, "", "", s3req, errors.DenyInvalidResource, err, startTime, r)
		return
	}

	// Check if bucket is empty (listing buckets is not supported)
	if s3req.Bucket == "" {
		g.handleError(w, requestID, "", "", s3req, errors.DenyInvalidResource,
			nil, startTime, r)
		return
	}

	// Authenticate request
	authCtx, err := g.authenticate(r)
	if err != nil {
		log.Printf("[%s] Authentication failed: %v", requestID, err)
		g.handleError(w, requestID, "", "", s3req, errors.DenyAuthFailed, err, startTime, r)
		return
	}

	// Check tenant boundary
	if !g.checkTenantBoundary(authCtx, s3req) {
		log.Printf("[%s] Tenant boundary violation: client=%s tenant=%s bucket=%s",
			requestID, authCtx.ClientID, authCtx.TenantID, s3req.Bucket)
		g.handleError(w, requestID, authCtx.ClientID, authCtx.TenantID, s3req,
			errors.DenyTenantBoundary, nil, startTime, r)
		return
	}

	// Evaluate policy
	evalCtx := &policy.EvalContext{
		ClientID: authCtx.ClientID,
		TenantID: authCtx.TenantID,
		Action:   s3req.Action,
		Resource: s3req.ToARN(),
		Bucket:   s3req.Bucket,
		Key:      s3req.Key,
		Conditions: map[string]string{
			"aws:SourceIp": getClientIP(r),
		},
	}

	decision := g.policyEngine.Evaluate(evalCtx, authCtx.Policies)
	if !decision.Allowed {
		log.Printf("[%s] Policy denied: client=%s action=%s resource=%s reason=%s",
			requestID, authCtx.ClientID, s3req.Action, s3req.ToARN(), decision.DenyReason)
		g.handleError(w, requestID, authCtx.ClientID, authCtx.TenantID, s3req,
			decision.DenyReason, nil, startTime, r)
		return
	}

	// Forward to S3
	resp, err := g.s3Client.Forward(r.Context(), s3req)
	if err != nil {
		log.Printf("[%s] S3 forward error: %v", requestID, err)
		g.handleS3Error(w, requestID, authCtx.ClientID, authCtx.TenantID, s3req, err, startTime, r)
		return
	}

	// Log successful request
	g.auditLogger.Log(audit.NewAllowEntry(
		requestID,
		authCtx.ClientID,
		authCtx.TenantID,
		s3req.Action,
		s3req.Bucket,
		s3req.Key,
		getClientIP(r),
		r.UserAgent(),
		time.Since(startTime),
		resp.StatusCode,
	))

	// Write response
	g.writeResponse(w, resp)
}

// authenticate validates the request signature and returns the auth context
func (g *Gateway) authenticate(r *http.Request) (*auth.AuthContext, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.NewAccessDeniedError(errors.DenyAuthFailed,
			"missing Authorization header", "", "")
	}

	// Parse the authorization header to get the access key
	components, err := g.sigValidator.ParseAuthHeader(authHeader)
	if err != nil {
		return nil, err
	}

	// Look up the credential
	cred, err := g.credStore.GetCredential(components.AccessKey)
	if err != nil {
		return nil, err
	}

	// Validate the signature
	_, err = g.sigValidator.ParseAndValidate(r, cred)
	if err != nil {
		return nil, err
	}

	return &auth.AuthContext{
		ClientID:  cred.ClientID,
		TenantID:  cred.TenantID,
		AccessKey: cred.AccessKey,
		Policies:  cred.Policies,
		Scopes:    cred.Scopes,
	}, nil
}

// checkTenantBoundary verifies that the request is within the client's allowed scope
func (g *Gateway) checkTenantBoundary(authCtx *auth.AuthContext, s3req *S3Request) bool {
	if len(authCtx.Scopes) == 0 {
		return false // No scopes means no access
	}

	return policy.MatchScope(s3req.Bucket, authCtx.Scopes)
}

// handleError writes an error response and logs the denial
func (g *Gateway) handleError(
	w http.ResponseWriter,
	requestID, clientID, tenantID string,
	s3req *S3Request,
	reason errors.DenyReason,
	err error,
	startTime time.Time,
	r *http.Request,
) {
	bucket := ""
	key := ""
	action := ""
	if s3req != nil {
		bucket = s3req.Bucket
		key = s3req.Key
		action = s3req.Action
	}

	// Log the denial
	g.auditLogger.Log(audit.NewDenyEntry(
		requestID,
		clientID,
		tenantID,
		action,
		bucket,
		key,
		getClientIP(r),
		r.UserAgent(),
		string(reason),
		time.Since(startTime),
	))

	// Write error response
	accessErr := errors.NewAccessDeniedError(reason, "", bucket+"/"+key, requestID)
	errors.WriteS3Error(w, accessErr)
}

// handleS3Error handles errors from the upstream S3
func (g *Gateway) handleS3Error(
	w http.ResponseWriter,
	requestID, clientID, tenantID string,
	s3req *S3Request,
	err error,
	startTime time.Time,
	r *http.Request,
) {
	// Log the error
	entry := audit.NewDenyEntry(
		requestID,
		clientID,
		tenantID,
		s3req.Action,
		s3req.Bucket,
		s3req.Key,
		getClientIP(r),
		r.UserAgent(),
		"S3_ERROR",
		time.Since(startTime),
	)
	entry.ErrorMsg = err.Error()
	g.auditLogger.Log(entry)

	// Check if it's a not found error
	errStr := err.Error()
	if strings.Contains(errStr, "NoSuchKey") || strings.Contains(errStr, "NotFound") {
		errors.WriteS3ErrorFromCode(w, http.StatusNotFound, "NoSuchKey",
			"The specified key does not exist.", requestID)
		return
	}
	if strings.Contains(errStr, "NoSuchBucket") {
		errors.WriteS3ErrorFromCode(w, http.StatusNotFound, "NoSuchBucket",
			"The specified bucket does not exist.", requestID)
		return
	}

	// Generic internal error
	errors.WriteS3ErrorFromCode(w, http.StatusInternalServerError, "InternalError",
		"We encountered an internal error. Please try again.", requestID)
}

// writeResponse writes the S3 response to the HTTP response writer
func (g *Gateway) writeResponse(w http.ResponseWriter, resp *S3Response) {
	// Copy headers
	for key, values := range resp.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy body if present
	if resp.Body != nil {
		defer resp.Body.Close()
		io.Copy(w, resp.Body)
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	// Remove port if present
	if colonIdx := strings.LastIndex(addr, ":"); colonIdx != -1 {
		addr = addr[:colonIdx]
	}
	return addr
}
