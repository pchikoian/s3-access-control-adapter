package auth

import (
	"fmt"
	"sync"

	"github.com/s3-access-control-adapter/internal/config"
)

// Credential represents a client's authentication credential with associated metadata
type Credential struct {
	AccessKey   string
	SecretKey   string
	ClientID    string
	TenantID    string
	Description string
	Policies    []string
	Scopes      []string // Allowed bucket/prefix patterns for tenant boundary check
}

// CredentialStore provides access to client credentials
type CredentialStore interface {
	// GetCredential retrieves a credential by access key
	GetCredential(accessKey string) (*Credential, error)
	// Reload reloads credentials from the configuration file
	Reload() error
}

// InMemoryCredentialStore stores credentials in memory, loaded from a config file
type InMemoryCredentialStore struct {
	mu          sync.RWMutex
	credentials map[string]*Credential
	configPath  string
}

// NewInMemoryCredentialStore creates a new in-memory credential store
func NewInMemoryCredentialStore(configPath string) (*InMemoryCredentialStore, error) {
	store := &InMemoryCredentialStore{
		credentials: make(map[string]*Credential),
		configPath:  configPath,
	}

	if err := store.Reload(); err != nil {
		return nil, err
	}

	return store, nil
}

// GetCredential retrieves a credential by access key
func (s *InMemoryCredentialStore) GetCredential(accessKey string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cred, ok := s.credentials[accessKey]
	if !ok {
		return nil, fmt.Errorf("credential not found for access key: %s", accessKey)
	}

	return cred, nil
}

// Reload reloads credentials from the configuration file
func (s *InMemoryCredentialStore) Reload() error {
	cfg, err := config.LoadCredentials(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	newCreds := make(map[string]*Credential, len(cfg.Credentials))
	for _, c := range cfg.Credentials {
		newCreds[c.AccessKey] = &Credential{
			AccessKey:   c.AccessKey,
			SecretKey:   c.SecretKey,
			ClientID:    c.ClientID,
			TenantID:    c.TenantID,
			Description: c.Description,
			Policies:    c.Policies,
			Scopes:      c.Scopes,
		}
	}

	s.mu.Lock()
	s.credentials = newCreds
	s.mu.Unlock()

	return nil
}
