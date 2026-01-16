package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/s3-access-control-adapter/internal/errors"
)

func TestPolicyEngine_DefaultDeny(t *testing.T) {
	// Create a temporary policy file with no policies
	tmpDir := t.TempDir()
	policyFile := filepath.Join(tmpDir, "policies.yaml")
	os.WriteFile(policyFile, []byte("policies: []"), 0644)

	engine, err := NewEngine(policyFile)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	ctx := &EvalContext{
		ClientID: "test-client",
		TenantID: "test-tenant",
		Action:   "s3:GetObject",
		Resource: "arn:aws:s3:::bucket/key",
	}

	decision := engine.Evaluate(ctx, []string{"nonexistent-policy"})

	if decision.Allowed {
		t.Error("Expected default deny, got allow")
	}
	if decision.DenyReason != errors.DenyPolicy {
		t.Errorf("Expected DenyReason=%s, got %s", errors.DenyPolicy, decision.DenyReason)
	}
}

func TestPolicyEngine_AllowStatement(t *testing.T) {
	tmpDir := t.TempDir()
	policyFile := filepath.Join(tmpDir, "policies.yaml")
	policyContent := `
policies:
  - name: test-policy
    version: "2012-10-17"
    statements:
      - sid: AllowGet
        effect: Allow
        actions:
          - s3:GetObject
        resources:
          - arn:aws:s3:::test-bucket/*
`
	os.WriteFile(policyFile, []byte(policyContent), 0644)

	engine, err := NewEngine(policyFile)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	tests := []struct {
		name      string
		action    string
		resource  string
		wantAllow bool
	}{
		{
			name:      "allowed action and resource",
			action:    "s3:GetObject",
			resource:  "arn:aws:s3:::test-bucket/file.txt",
			wantAllow: true,
		},
		{
			name:      "wrong action",
			action:    "s3:PutObject",
			resource:  "arn:aws:s3:::test-bucket/file.txt",
			wantAllow: false,
		},
		{
			name:      "wrong resource",
			action:    "s3:GetObject",
			resource:  "arn:aws:s3:::other-bucket/file.txt",
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvalContext{
				ClientID: "test-client",
				TenantID: "test-tenant",
				Action:   tt.action,
				Resource: tt.resource,
			}

			decision := engine.Evaluate(ctx, []string{"test-policy"})

			if decision.Allowed != tt.wantAllow {
				t.Errorf("Evaluate() allowed = %v, want %v", decision.Allowed, tt.wantAllow)
			}
		})
	}
}

func TestPolicyEngine_ExplicitDenyTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	policyFile := filepath.Join(tmpDir, "policies.yaml")
	policyContent := `
policies:
  - name: allow-policy
    version: "2012-10-17"
    statements:
      - sid: AllowAll
        effect: Allow
        actions:
          - s3:*
        resources:
          - arn:aws:s3:::*
          - arn:aws:s3:::*/*
  - name: deny-policy
    version: "2012-10-17"
    statements:
      - sid: DenyDelete
        effect: Deny
        actions:
          - s3:DeleteObject
        resources:
          - arn:aws:s3:::protected-bucket/*
`
	os.WriteFile(policyFile, []byte(policyContent), 0644)

	engine, err := NewEngine(policyFile)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	tests := []struct {
		name      string
		action    string
		resource  string
		policies  []string
		wantAllow bool
	}{
		{
			name:      "allow without deny policy",
			action:    "s3:DeleteObject",
			resource:  "arn:aws:s3:::protected-bucket/file.txt",
			policies:  []string{"allow-policy"},
			wantAllow: true,
		},
		{
			name:      "deny takes precedence",
			action:    "s3:DeleteObject",
			resource:  "arn:aws:s3:::protected-bucket/file.txt",
			policies:  []string{"allow-policy", "deny-policy"},
			wantAllow: false,
		},
		{
			name:      "deny first still denies",
			action:    "s3:DeleteObject",
			resource:  "arn:aws:s3:::protected-bucket/file.txt",
			policies:  []string{"deny-policy", "allow-policy"},
			wantAllow: false,
		},
		{
			name:      "other action still allowed",
			action:    "s3:GetObject",
			resource:  "arn:aws:s3:::protected-bucket/file.txt",
			policies:  []string{"allow-policy", "deny-policy"},
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvalContext{
				ClientID: "test-client",
				TenantID: "test-tenant",
				Action:   tt.action,
				Resource: tt.resource,
			}

			decision := engine.Evaluate(ctx, tt.policies)

			if decision.Allowed != tt.wantAllow {
				t.Errorf("Evaluate() allowed = %v, want %v", decision.Allowed, tt.wantAllow)
			}
		})
	}
}

func TestPolicyEngine_WildcardActions(t *testing.T) {
	tmpDir := t.TempDir()
	policyFile := filepath.Join(tmpDir, "policies.yaml")
	policyContent := `
policies:
  - name: wildcard-policy
    version: "2012-10-17"
    statements:
      - sid: AllowGetOperations
        effect: Allow
        actions:
          - s3:Get*
        resources:
          - arn:aws:s3:::bucket/*
`
	os.WriteFile(policyFile, []byte(policyContent), 0644)

	engine, err := NewEngine(policyFile)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	tests := []struct {
		action    string
		wantAllow bool
	}{
		{"s3:GetObject", true},
		{"s3:GetObjectAcl", true},
		{"s3:GetBucketAcl", true},
		{"s3:PutObject", false},
		{"s3:DeleteObject", false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			ctx := &EvalContext{
				Action:   tt.action,
				Resource: "arn:aws:s3:::bucket/key",
			}

			decision := engine.Evaluate(ctx, []string{"wildcard-policy"})

			if decision.Allowed != tt.wantAllow {
				t.Errorf("Evaluate(%s) allowed = %v, want %v", tt.action, decision.Allowed, tt.wantAllow)
			}
		})
	}
}
