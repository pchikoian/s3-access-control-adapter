package config

import "time"

// GatewayConfig holds the main configuration for the gateway
type GatewayConfig struct {
	Server          ServerConfig `yaml:"server"`
	AWS             AWSConfig    `yaml:"aws"`
	CredentialsFile string       `yaml:"credentialsFile"`
	PoliciesFile    string       `yaml:"policiesFile"`
	Audit           AuditConfig  `yaml:"audit"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
}

// AWSConfig holds AWS/S3 connection settings
type AWSConfig struct {
	Region          string `yaml:"region"`
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"accessKeyId"`
	SecretAccessKey string `yaml:"secretAccessKey"`
	UsePathStyle    bool   `yaml:"usePathStyle"`
}

// AuditConfig holds audit logging settings
type AuditConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Output   string `yaml:"output"` // stdout, file, or both
	FilePath string `yaml:"filePath"`
	Format   string `yaml:"format"` // json
}

// CredentialsConfig holds the list of client credentials
type CredentialsConfig struct {
	Credentials []Credential `yaml:"credentials"`
}

// Credential represents a client's authentication credentials
type Credential struct {
	AccessKey   string   `yaml:"accessKey"`
	SecretKey   string   `yaml:"secretKey"`
	ClientID    string   `yaml:"clientId"`
	TenantID    string   `yaml:"tenantId"`
	Description string   `yaml:"description"`
	Policies    []string `yaml:"policies"`
	Scopes      []string `yaml:"scopes"` // Allowed bucket/prefix patterns
}

// PoliciesConfig holds the list of IAM-like policies
type PoliciesConfig struct {
	Policies []Policy `yaml:"policies"`
}

// Policy represents an IAM-like policy
type Policy struct {
	Name       string      `yaml:"name"`
	Version    string      `yaml:"version"`
	Statements []Statement `yaml:"statements"`
}

// Statement represents a policy statement
type Statement struct {
	Sid        string                       `yaml:"sid"`
	Effect     Effect                       `yaml:"effect"`
	Actions    []string                     `yaml:"actions"`
	Resources  []string                     `yaml:"resources"`
	Conditions map[string]map[string]string `yaml:"conditions,omitempty"`
}

// Effect represents Allow or Deny
type Effect string

const (
	EffectAllow Effect = "Allow"
	EffectDeny  Effect = "Deny"
)
