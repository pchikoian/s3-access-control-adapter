package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/s3-access-control-adapter/internal/config"
)

func TestNewLogger_Disabled(t *testing.T) {
	cfg := &config.AuditConfig{
		Enabled: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if logger.enabled {
		t.Error("expected logger to be disabled")
	}
	if len(logger.writers) != 0 {
		t.Errorf("expected no writers, got %d", len(logger.writers))
	}
}

func TestNewLogger_Stdout(t *testing.T) {
	cfg := &config.AuditConfig{
		Enabled: true,
		Output:  "stdout",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if !logger.enabled {
		t.Error("expected logger to be enabled")
	}
	if len(logger.writers) != 1 {
		t.Errorf("expected 1 writer, got %d", len(logger.writers))
	}
}

func TestNewLogger_File(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.AuditConfig{
		Enabled:  true,
		Output:   "file",
		FilePath: filePath,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if !logger.enabled {
		t.Error("expected logger to be enabled")
	}
	if len(logger.writers) != 1 {
		t.Errorf("expected 1 writer, got %d", len(logger.writers))
	}
	if logger.file == nil {
		t.Error("expected file to be set")
	}
}

func TestNewLogger_Both(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.AuditConfig{
		Enabled:  true,
		Output:   "both",
		FilePath: filePath,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if !logger.enabled {
		t.Error("expected logger to be enabled")
	}
	if len(logger.writers) != 2 {
		t.Errorf("expected 2 writers, got %d", len(logger.writers))
	}
}

func TestNewLogger_DefaultOutput(t *testing.T) {
	cfg := &config.AuditConfig{
		Enabled: true,
		Output:  "unknown",
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if len(logger.writers) != 1 {
		t.Errorf("expected 1 writer (default stdout), got %d", len(logger.writers))
	}
}

func TestNewLogger_FileError(t *testing.T) {
	cfg := &config.AuditConfig{
		Enabled:  true,
		Output:   "file",
		FilePath: "/nonexistent/path/audit.log",
	}

	_, err := NewLogger(cfg)
	if err == nil {
		t.Error("expected error for invalid file path")
	}
}

func TestJSONLogger_Log(t *testing.T) {
	var buf bytes.Buffer

	logger := &JSONLogger{
		enabled: true,
		writers: []io.Writer{&buf},
	}

	entry := &Entry{
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		RequestID:  "req-123",
		ClientID:   "client-a",
		TenantID:   "tenant-001",
		Action:     "s3:GetObject",
		Resource:   "arn:aws:s3:::mybucket/mykey",
		Bucket:     "mybucket",
		Key:        "mykey",
		Decision:   "allow",
		SourceIP:   "192.168.1.1",
		UserAgent:  "aws-sdk-go/1.0",
		DurationMs: 50,
		StatusCode: 200,
	}

	err := logger.Log(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Error("expected output to end with newline")
	}

	var decoded Entry
	err = json.Unmarshal([]byte(output), &decoded)
	if err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if decoded.RequestID != "req-123" {
		t.Errorf("expected requestId 'req-123', got '%s'", decoded.RequestID)
	}
	if decoded.Decision != "allow" {
		t.Errorf("expected decision 'allow', got '%s'", decoded.Decision)
	}
	if decoded.Action != "s3:GetObject" {
		t.Errorf("expected action 's3:GetObject', got '%s'", decoded.Action)
	}
}

func TestJSONLogger_Log_Disabled(t *testing.T) {
	var buf bytes.Buffer

	logger := &JSONLogger{
		enabled: false,
		writers: []io.Writer{&buf},
	}

	entry := &Entry{
		RequestID: "req-123",
		Decision:  "allow",
	}

	err := logger.Log(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Error("expected no output when logger is disabled")
	}
}

func TestJSONLogger_Log_NoWriters(t *testing.T) {
	logger := &JSONLogger{
		enabled: true,
		writers: []io.Writer{},
	}

	entry := &Entry{
		RequestID: "req-123",
		Decision:  "allow",
	}

	err := logger.Log(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJSONLogger_Log_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "audit.log")

	cfg := &config.AuditConfig{
		Enabled:  true,
		Output:   "file",
		FilePath: filePath,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := &Entry{
		Timestamp:  time.Now().UTC(),
		RequestID:  "req-456",
		ClientID:   "client-b",
		TenantID:   "tenant-002",
		Action:     "s3:PutObject",
		Resource:   "arn:aws:s3:::bucket/key",
		Bucket:     "bucket",
		Key:        "key",
		Decision:   "deny",
		DenyReason: "DENY_POLICY",
		SourceIP:   "10.0.0.1",
		DurationMs: 10,
	}

	err = logger.Log(entry)
	if err != nil {
		t.Fatalf("failed to log: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var decoded Entry
	err = json.Unmarshal(content, &decoded)
	if err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if decoded.RequestID != "req-456" {
		t.Errorf("expected requestId 'req-456', got '%s'", decoded.RequestID)
	}
	if decoded.DenyReason != "DENY_POLICY" {
		t.Errorf("expected denyReason 'DENY_POLICY', got '%s'", decoded.DenyReason)
	}
}

func TestJSONLogger_Close_NoFile(t *testing.T) {
	logger := &JSONLogger{
		enabled: true,
		writers: []io.Writer{os.Stdout},
	}

	err := logger.Close()
	if err != nil {
		t.Errorf("unexpected error closing logger without file: %v", err)
	}
}

func TestNewAllowEntry(t *testing.T) {
	duration := 100 * time.Millisecond

	entry := NewAllowEntry(
		"req-789",
		"client-c",
		"tenant-003",
		"s3:GetObject",
		"mybucket",
		"path/to/object.txt",
		"172.16.0.1",
		"curl/7.68.0",
		duration,
		200,
	)

	if entry.RequestID != "req-789" {
		t.Errorf("expected requestId 'req-789', got '%s'", entry.RequestID)
	}
	if entry.ClientID != "client-c" {
		t.Errorf("expected clientId 'client-c', got '%s'", entry.ClientID)
	}
	if entry.TenantID != "tenant-003" {
		t.Errorf("expected tenantId 'tenant-003', got '%s'", entry.TenantID)
	}
	if entry.Action != "s3:GetObject" {
		t.Errorf("expected action 's3:GetObject', got '%s'", entry.Action)
	}
	if entry.Bucket != "mybucket" {
		t.Errorf("expected bucket 'mybucket', got '%s'", entry.Bucket)
	}
	if entry.Key != "path/to/object.txt" {
		t.Errorf("expected key 'path/to/object.txt', got '%s'", entry.Key)
	}
	if entry.Resource != "arn:aws:s3:::mybucket/path/to/object.txt" {
		t.Errorf("expected resource 'arn:aws:s3:::mybucket/path/to/object.txt', got '%s'", entry.Resource)
	}
	if entry.Decision != "allow" {
		t.Errorf("expected decision 'allow', got '%s'", entry.Decision)
	}
	if entry.SourceIP != "172.16.0.1" {
		t.Errorf("expected sourceIp '172.16.0.1', got '%s'", entry.SourceIP)
	}
	if entry.UserAgent != "curl/7.68.0" {
		t.Errorf("expected userAgent 'curl/7.68.0', got '%s'", entry.UserAgent)
	}
	if entry.DurationMs != 100 {
		t.Errorf("expected durationMs 100, got %d", entry.DurationMs)
	}
	if entry.StatusCode != 200 {
		t.Errorf("expected statusCode 200, got %d", entry.StatusCode)
	}
	if entry.DenyReason != "" {
		t.Errorf("expected empty denyReason, got '%s'", entry.DenyReason)
	}
}

func TestNewAllowEntry_BucketOnly(t *testing.T) {
	entry := NewAllowEntry(
		"req-001",
		"client",
		"tenant",
		"s3:ListBucket",
		"mybucket",
		"",
		"10.0.0.1",
		"",
		50*time.Millisecond,
		200,
	)

	if entry.Resource != "arn:aws:s3:::mybucket" {
		t.Errorf("expected resource 'arn:aws:s3:::mybucket', got '%s'", entry.Resource)
	}
	if entry.Key != "" {
		t.Errorf("expected empty key, got '%s'", entry.Key)
	}
}

func TestNewDenyEntry(t *testing.T) {
	duration := 25 * time.Millisecond

	entry := NewDenyEntry(
		"req-999",
		"client-d",
		"tenant-004",
		"s3:DeleteObject",
		"restricted-bucket",
		"secret.txt",
		"192.168.100.50",
		"python-requests/2.25.1",
		"DENY_TENANT_BOUNDARY",
		duration,
	)

	if entry.RequestID != "req-999" {
		t.Errorf("expected requestId 'req-999', got '%s'", entry.RequestID)
	}
	if entry.ClientID != "client-d" {
		t.Errorf("expected clientId 'client-d', got '%s'", entry.ClientID)
	}
	if entry.TenantID != "tenant-004" {
		t.Errorf("expected tenantId 'tenant-004', got '%s'", entry.TenantID)
	}
	if entry.Action != "s3:DeleteObject" {
		t.Errorf("expected action 's3:DeleteObject', got '%s'", entry.Action)
	}
	if entry.Bucket != "restricted-bucket" {
		t.Errorf("expected bucket 'restricted-bucket', got '%s'", entry.Bucket)
	}
	if entry.Key != "secret.txt" {
		t.Errorf("expected key 'secret.txt', got '%s'", entry.Key)
	}
	if entry.Resource != "arn:aws:s3:::restricted-bucket/secret.txt" {
		t.Errorf("expected resource 'arn:aws:s3:::restricted-bucket/secret.txt', got '%s'", entry.Resource)
	}
	if entry.Decision != "deny" {
		t.Errorf("expected decision 'deny', got '%s'", entry.Decision)
	}
	if entry.DenyReason != "DENY_TENANT_BOUNDARY" {
		t.Errorf("expected denyReason 'DENY_TENANT_BOUNDARY', got '%s'", entry.DenyReason)
	}
	if entry.SourceIP != "192.168.100.50" {
		t.Errorf("expected sourceIp '192.168.100.50', got '%s'", entry.SourceIP)
	}
	if entry.UserAgent != "python-requests/2.25.1" {
		t.Errorf("expected userAgent 'python-requests/2.25.1', got '%s'", entry.UserAgent)
	}
	if entry.DurationMs != 25 {
		t.Errorf("expected durationMs 25, got %d", entry.DurationMs)
	}
	if entry.StatusCode != 0 {
		t.Errorf("expected statusCode 0, got %d", entry.StatusCode)
	}
}

func TestNewDenyEntry_BucketOnly(t *testing.T) {
	entry := NewDenyEntry(
		"req-002",
		"client",
		"tenant",
		"s3:ListBucket",
		"mybucket",
		"",
		"10.0.0.1",
		"",
		"DENY_POLICY",
		10*time.Millisecond,
	)

	if entry.Resource != "arn:aws:s3:::mybucket" {
		t.Errorf("expected resource 'arn:aws:s3:::mybucket', got '%s'", entry.Resource)
	}
}

func TestBuildResourceARN(t *testing.T) {
	tests := []struct {
		bucket   string
		key      string
		expected string
	}{
		{"mybucket", "", "arn:aws:s3:::mybucket"},
		{"mybucket", "mykey", "arn:aws:s3:::mybucket/mykey"},
		{"mybucket", "path/to/object", "arn:aws:s3:::mybucket/path/to/object"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := buildResourceARN(tt.bucket, tt.key)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
