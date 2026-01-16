package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// envVarRegex matches ${VAR_NAME} patterns
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// LoadGatewayConfig loads the main gateway configuration from a YAML file
func LoadGatewayConfig(path string) (*GatewayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	data = substituteEnvVars(data)

	var cfg GatewayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate
	if err := validateGatewayConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadCredentials loads client credentials from a YAML file
func LoadCredentials(path string) (*CredentialsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var cfg CredentialsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	if err := validateCredentials(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadPolicies loads IAM-like policies from a YAML file
func LoadPolicies(path string) (*PoliciesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policies file: %w", err)
	}

	var cfg PoliciesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse policies file: %w", err)
	}

	if err := validatePolicies(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// substituteEnvVars replaces ${VAR_NAME} with environment variable values
func substituteEnvVars(data []byte) []byte {
	return envVarRegex.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := string(envVarRegex.FindSubmatch(match)[1])
		if value := os.Getenv(varName); value != "" {
			return []byte(value)
		}
		return match // Keep original if env var not set
	})
}

func applyDefaults(cfg *GatewayConfig) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 60 * time.Second
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 10 * time.Second
	}
	if cfg.AWS.Region == "" {
		cfg.AWS.Region = "us-east-1"
	}
	if cfg.Audit.Format == "" {
		cfg.Audit.Format = "json"
	}
	if cfg.Audit.Output == "" {
		cfg.Audit.Output = "stdout"
	}
}

func validateGatewayConfig(cfg *GatewayConfig) error {
	if cfg.CredentialsFile == "" {
		return fmt.Errorf("credentialsFile is required")
	}
	if cfg.PoliciesFile == "" {
		return fmt.Errorf("policiesFile is required")
	}
	return nil
}

func validateCredentials(cfg *CredentialsConfig) error {
	seen := make(map[string]bool)
	for i, cred := range cfg.Credentials {
		if cred.AccessKey == "" {
			return fmt.Errorf("credentials[%d]: accessKey is required", i)
		}
		if cred.SecretKey == "" {
			return fmt.Errorf("credentials[%d]: secretKey is required", i)
		}
		if cred.ClientID == "" {
			return fmt.Errorf("credentials[%d]: clientId is required", i)
		}
		if cred.TenantID == "" {
			return fmt.Errorf("credentials[%d]: tenantId is required", i)
		}
		if seen[cred.AccessKey] {
			return fmt.Errorf("credentials[%d]: duplicate accessKey %q", i, cred.AccessKey)
		}
		seen[cred.AccessKey] = true
	}
	return nil
}

func validatePolicies(cfg *PoliciesConfig) error {
	seen := make(map[string]bool)
	for i, policy := range cfg.Policies {
		if policy.Name == "" {
			return fmt.Errorf("policies[%d]: name is required", i)
		}
		if seen[policy.Name] {
			return fmt.Errorf("policies[%d]: duplicate policy name %q", i, policy.Name)
		}
		seen[policy.Name] = true

		for j, stmt := range policy.Statements {
			if stmt.Effect != EffectAllow && stmt.Effect != EffectDeny {
				return fmt.Errorf("policies[%d].statements[%d]: effect must be Allow or Deny", i, j)
			}
			if len(stmt.Actions) == 0 {
				return fmt.Errorf("policies[%d].statements[%d]: actions is required", i, j)
			}
			if len(stmt.Resources) == 0 {
				return fmt.Errorf("policies[%d].statements[%d]: resources is required", i, j)
			}
		}
	}
	return nil
}
