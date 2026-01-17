package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/s3-access-control-adapter/internal/config"
)

// Entry represents an audit log entry
type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"requestId"`
	ClientID   string    `json:"clientId"`
	TenantID   string    `json:"tenantId"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	Bucket     string    `json:"bucket"`
	Key        string    `json:"key,omitempty"`
	Decision   string    `json:"decision"` // "allow" or "deny"
	DenyReason string    `json:"denyReason,omitempty"`
	SourceIP   string    `json:"sourceIp"`
	UserAgent  string    `json:"userAgent,omitempty"`
	DurationMs int64     `json:"durationMs"`
	StatusCode int       `json:"statusCode,omitempty"`
	ErrorMsg   string    `json:"error,omitempty"`
}

// Logger is the interface for audit logging
type Logger interface {
	Log(entry *Entry) error
	Close() error
}

// JSONLogger writes audit logs in JSON lines format
type JSONLogger struct {
	mu      sync.Mutex
	writers []io.Writer
	file    *os.File
	enabled bool
}

// NewLogger creates a new audit logger based on configuration
func NewLogger(cfg *config.AuditConfig) (*JSONLogger, error) {
	logger := &JSONLogger{
		enabled: cfg.Enabled,
		writers: []io.Writer{},
	}

	if !cfg.Enabled {
		return logger, nil
	}

	switch cfg.Output {
	case "stdout":
		logger.writers = append(logger.writers, os.Stdout)
	case "file":
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		logger.file = file
		logger.writers = append(logger.writers, file)
	case "both":
		logger.writers = append(logger.writers, os.Stdout)
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		logger.file = file
		logger.writers = append(logger.writers, file)
	default:
		logger.writers = append(logger.writers, os.Stdout)
	}

	return logger, nil
}

// Log writes an audit entry
func (l *JSONLogger) Log(entry *Entry) error {
	if !l.enabled || len(l.writers) == 0 {
		return nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, w := range l.writers {
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write audit entry: %w", err)
		}
	}

	return nil
}

// Close closes the audit logger
func (l *JSONLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// NewAllowEntry creates an audit entry for an allowed request
func NewAllowEntry(requestID, clientID, tenantID, action, bucket, key, sourceIP, userAgent string, duration time.Duration, statusCode int) *Entry {
	return &Entry{
		Timestamp:  time.Now().UTC(),
		RequestID:  requestID,
		ClientID:   clientID,
		TenantID:   tenantID,
		Action:     action,
		Resource:   buildResourceARN(bucket, key),
		Bucket:     bucket,
		Key:        key,
		Decision:   "allow",
		SourceIP:   sourceIP,
		UserAgent:  userAgent,
		DurationMs: duration.Milliseconds(),
		StatusCode: statusCode,
	}
}

// NewDenyEntry creates an audit entry for a denied request
func NewDenyEntry(requestID, clientID, tenantID, action, bucket, key, sourceIP, userAgent, denyReason string, duration time.Duration) *Entry {
	return &Entry{
		Timestamp:  time.Now().UTC(),
		RequestID:  requestID,
		ClientID:   clientID,
		TenantID:   tenantID,
		Action:     action,
		Resource:   buildResourceARN(bucket, key),
		Bucket:     bucket,
		Key:        key,
		Decision:   "deny",
		DenyReason: denyReason,
		SourceIP:   sourceIP,
		UserAgent:  userAgent,
		DurationMs: duration.Milliseconds(),
	}
}

func buildResourceARN(bucket, key string) string {
	if key == "" {
		return "arn:aws:s3:::" + bucket
	}
	return "arn:aws:s3:::" + bucket + "/" + key
}
