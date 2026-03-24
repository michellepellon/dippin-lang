package validator

import (
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

// lintUnboundedRetry checks DIP104: nodes with retry configuration that have
// no max_retries limit and no fallback target. This could cause infinite retry
// loops at runtime.
func lintUnboundedRetry(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if isUnboundedRetry(n.Retry) {
			diags = append(diags, Diagnostic{
				Code:     DIP104,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has retry configuration but no max_retries or fallback_target", n.ID),
				Location: n.Source,
				Help:     "set max_retries to limit retries, or add a fallback_target for graceful degradation",
			})
		}
	}
	return diags
}

// isUnboundedRetry returns true if retry config exists but has no bounds.
func isUnboundedRetry(r ir.RetryConfig) bool {
	hasRetryConfig := r.Policy != "" || r.RetryTarget != ""
	return hasRetryConfig && r.MaxRetries == 0 && r.FallbackTarget == ""
}

// validRetryPolicies is the set of retry policy names recognized by Tracker's engine.
var validRetryPolicies = map[string]bool{
	"standard":   true,
	"aggressive": true,
	"patient":    true,
	"linear":     true,
	"none":       true,
}

// lintRetryPolicy checks DIP113: retry_policy must be one of the known policy names.
func lintRetryPolicy(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	diags = append(diags, checkDefaultRetryPolicy(w)...)
	diags = append(diags, checkNodeRetryPolicies(w)...)
	return diags
}

// checkDefaultRetryPolicy checks the workflow-level default retry policy.
func checkDefaultRetryPolicy(w *ir.Workflow) []Diagnostic {
	if p := w.Defaults.RetryPolicy; p != "" && !validRetryPolicies[p] {
		return []Diagnostic{{
			Code:     DIP113,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("workflow default retry_policy %q is not a recognized policy name", p),
			Help:     "valid policies: standard, aggressive, patient, linear, none",
		}}
	}
	return nil
}

// checkNodeRetryPolicies checks per-node retry policies.
func checkNodeRetryPolicies(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if p := n.Retry.Policy; p != "" && !validRetryPolicies[p] {
			diags = append(diags, Diagnostic{
				Code:     DIP113,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has retry_policy %q which is not a recognized policy name", n.ID, p),
				Location: n.Source,
				Help:     "valid policies: standard, aggressive, patient, linear, none",
			})
		}
	}
	return diags
}

// lintGoalGateFallback checks DIP115: nodes with goal_gate: true should have
// a retry_target or fallback_target so the pipeline has a recovery path when
// the gate fails.
func lintGoalGateFallback(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if needsGoalGateFallback(n) {
			diags = append(diags, Diagnostic{
				Code:     DIP115,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has goal_gate: true but no retry_target or fallback_target", n.ID),
				Location: n.Source,
				Help:     "add retry_target or fallback_target so the pipeline can recover when the gate fails",
			})
		}
	}
	return diags
}

// needsGoalGateFallback returns true if a node has goal_gate but no recovery path.
func needsGoalGateFallback(n *ir.Node) bool {
	cfg, ok := n.Config.(ir.AgentConfig)
	if !ok || !cfg.GoalGate {
		return false
	}
	return n.Retry.RetryTarget == "" && n.Retry.FallbackTarget == ""
}
