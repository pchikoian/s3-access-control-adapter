package policy

import "github.com/s3-access-control-adapter/internal/errors"

// Effect represents Allow or Deny
type Effect string

const (
	EffectAllow Effect = "Allow"
	EffectDeny  Effect = "Deny"
)

// Policy represents an IAM-like policy
type Policy struct {
	Name       string
	Version    string
	Statements []Statement
}

// Statement represents a policy statement
type Statement struct {
	Sid        string
	Effect     Effect
	Actions    []string
	Resources  []string
	Conditions map[string]map[string]string
}

// EvalContext contains the context for policy evaluation
type EvalContext struct {
	ClientID   string
	TenantID   string
	Action     string            // e.g., "s3:GetObject"
	Resource   string            // e.g., "arn:aws:s3:::bucket/key"
	Bucket     string            // Bucket name for convenience
	Key        string            // Object key for convenience
	Conditions map[string]string // Runtime conditions (source IP, etc.)
}

// Decision represents the result of policy evaluation
type Decision struct {
	Allowed          bool
	DenyReason       errors.DenyReason
	MatchedPolicy    string
	MatchedStatement string
}

// NewAllowDecision creates an allow decision
func NewAllowDecision(policyName, statementSid string) *Decision {
	return &Decision{
		Allowed:          true,
		MatchedPolicy:    policyName,
		MatchedStatement: statementSid,
	}
}

// NewDenyDecision creates a deny decision
func NewDenyDecision(reason errors.DenyReason, policyName, statementSid string) *Decision {
	return &Decision{
		Allowed:          false,
		DenyReason:       reason,
		MatchedPolicy:    policyName,
		MatchedStatement: statementSid,
	}
}

// DefaultDenyDecision returns the default deny decision (no matching policy)
func DefaultDenyDecision() *Decision {
	return &Decision{
		Allowed:    false,
		DenyReason: errors.DenyPolicy,
	}
}
