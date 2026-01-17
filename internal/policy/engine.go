package policy

import (
	"fmt"
	"sync"

	"github.com/s3-access-control-adapter/internal/config"
	"github.com/s3-access-control-adapter/internal/errors"
)

// Engine evaluates IAM-like policies
type Engine interface {
	// Evaluate evaluates policies for a request
	Evaluate(ctx *EvalContext, policyNames []string) *Decision
	// Reload reloads policies from the configuration file
	Reload() error
	// GetPolicy retrieves a policy by name
	GetPolicy(name string) (*Policy, bool)
}

// DefaultEngine implements the policy evaluation engine
type DefaultEngine struct {
	mu         sync.RWMutex
	policies   map[string]*Policy
	configPath string
}

// NewEngine creates a new policy engine
func NewEngine(configPath string) (*DefaultEngine, error) {
	engine := &DefaultEngine{
		policies:   make(map[string]*Policy),
		configPath: configPath,
	}

	if err := engine.Reload(); err != nil {
		return nil, err
	}

	return engine, nil
}

// Reload reloads policies from the configuration file
func (e *DefaultEngine) Reload() error {
	cfg, err := config.LoadPolicies(e.configPath)
	if err != nil {
		return fmt.Errorf("failed to load policies: %w", err)
	}

	newPolicies := make(map[string]*Policy, len(cfg.Policies))
	for _, p := range cfg.Policies {
		policy := &Policy{
			Name:       p.Name,
			Version:    p.Version,
			Statements: make([]Statement, len(p.Statements)),
		}

		for i, s := range p.Statements {
			policy.Statements[i] = Statement{
				Sid:        s.Sid,
				Effect:     Effect(s.Effect),
				Actions:    s.Actions,
				Resources:  s.Resources,
				Conditions: s.Conditions,
			}
		}

		newPolicies[p.Name] = policy
	}

	e.mu.Lock()
	e.policies = newPolicies
	e.mu.Unlock()

	return nil
}

// GetPolicy retrieves a policy by name
func (e *DefaultEngine) GetPolicy(name string) (*Policy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, ok := e.policies[name]
	return policy, ok
}

// Evaluate evaluates policies for a request
// It implements AWS IAM evaluation logic:
// 1. Default deny
// 2. Explicit deny takes precedence over any allow
// 3. If there's an explicit allow and no explicit deny, allow
func (e *DefaultEngine) Evaluate(ctx *EvalContext, policyNames []string) *Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var allowDecision *Decision

	// Evaluate each policy
	for _, policyName := range policyNames {
		policy, ok := e.policies[policyName]
		if !ok {
			continue // Policy not found, skip
		}

		decision := e.evaluatePolicy(ctx, policy)

		// Explicit deny takes immediate precedence
		if decision != nil && !decision.Allowed {
			return decision
		}

		// Track the first allow
		if decision != nil && decision.Allowed && allowDecision == nil {
			allowDecision = decision
		}
	}

	// If we found an allow and no explicit deny, return allow
	if allowDecision != nil {
		return allowDecision
	}

	// Default deny
	return DefaultDenyDecision()
}

// evaluatePolicy evaluates a single policy
func (e *DefaultEngine) evaluatePolicy(ctx *EvalContext, policy *Policy) *Decision {
	var allowDecision *Decision

	for _, stmt := range policy.Statements {
		if !e.statementMatches(ctx, &stmt) {
			continue
		}

		if stmt.Effect == EffectDeny {
			// Explicit deny
			return NewDenyDecision(errors.DenyPolicy, policy.Name, stmt.Sid)
		}

		if stmt.Effect == EffectAllow && allowDecision == nil {
			allowDecision = NewAllowDecision(policy.Name, stmt.Sid)
		}
	}

	return allowDecision
}

// statementMatches checks if a statement matches the request context
func (e *DefaultEngine) statementMatches(ctx *EvalContext, stmt *Statement) bool {
	// Check if action matches
	if !MatchAction(ctx.Action, stmt.Actions) {
		return false
	}

	// Check if resource matches
	if !MatchResource(ctx.Resource, stmt.Resources) {
		return false
	}

	// Check conditions if present
	if len(stmt.Conditions) > 0 {
		if !e.evaluateConditions(ctx, stmt.Conditions) {
			return false
		}
	}

	return true
}

// evaluateConditions evaluates condition blocks
func (e *DefaultEngine) evaluateConditions(ctx *EvalContext, conditions map[string]map[string]string) bool {
	for operator, conditionBlock := range conditions {
		for key, expectedValue := range conditionBlock {
			actualValue, ok := ctx.Conditions[key]
			if !ok {
				return false
			}

			if !evaluateCondition(operator, actualValue, expectedValue) {
				return false
			}
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func evaluateCondition(operator, actual, expected string) bool {
	switch operator {
	case "StringEquals":
		return actual == expected
	case "StringNotEquals":
		return actual != expected
	case "StringLike":
		return MatchAction(actual, []string{expected})
	case "StringNotLike":
		return !MatchAction(actual, []string{expected})
	default:
		// Unsupported operator, fail closed
		return false
	}
}
