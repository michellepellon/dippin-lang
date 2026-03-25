package validator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// knownModelProviders lists known valid model/provider combinations.
// This is a best-effort catalog — unknown combinations produce a warning,
// not an error, since new models may be added at any time.
var knownModelProviders = map[string]map[string]bool{
	"anthropic": {
		"claude-opus-4-6":        true,
		"claude-sonnet-4-6":      true,
		"claude-haiku-4-5":       true,
		"claude-haiku-3-5":       true,
		"claude-opus-4-20250116": true,
	},
	"google": {
		"gemini-3-pro":                       true,
		"gemini-3-flash-preview":             true,
		"gemini-3.1-pro-preview-customtools": true,
		"gemini-2.5-pro":                     true,
		"gemini-2.5-flash":                   true,
		"gemini-2.0-flash":                   true,
	},
	"gemini": {
		"gemini-3-pro":                       true,
		"gemini-3-flash-preview":             true,
		"gemini-3.1-pro-preview-customtools": true,
		"gemini-2.5-pro":                     true,
		"gemini-2.5-flash":                   true,
		"gemini-2.0-flash":                   true,
	},
	"openai": {
		"gpt-5.4":       true,
		"gpt-5.3-codex": true,
		"gpt-5.2":       true,
		"gpt-4.1":       true,
		"gpt-4.1-mini":  true,
		"gpt-4.1-nano":  true,
		"gpt-4o":        true,
		"gpt-4o-mini":   true,
		"o3":            true,
		"o3-mini":       true,
		"o4-mini":       true,
		"o3-pro":        true,
	},
}

// lintModelProvider checks DIP108: model/provider combinations should be
// in the known catalog. Unknown combinations may indicate typos.
func lintModelProvider(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		diags = append(diags, checkNodeModelProvider(w, n)...)
	}
	return diags
}

// checkNodeModelProvider validates the model/provider for a single node.
func checkNodeModelProvider(w *ir.Workflow, n *ir.Node) []Diagnostic {
	cfg, ok := n.Config.(ir.AgentConfig)
	if !ok {
		return nil
	}
	model, provider := resolveModelProvider(cfg, w)
	if model == "" || provider == "" {
		return nil
	}
	return validateModelProvider(n, model, provider)
}

// resolveModelProvider resolves model and provider using node config and workflow defaults.
func resolveModelProvider(cfg ir.AgentConfig, w *ir.Workflow) (model, provider string) {
	model = cfg.Model
	provider = cfg.Provider
	if model == "" {
		model = w.Defaults.Model
	}
	if provider == "" {
		provider = w.Defaults.Provider
	}
	return
}

// validateModelProvider checks if a model/provider combination is known.
func validateModelProvider(n *ir.Node, model, provider string) []Diagnostic {
	providerModels, providerKnown := knownModelProviders[provider]
	if !providerKnown {
		return []Diagnostic{{
			Code:     DIP108,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("node %q uses unknown provider %q", n.ID, provider),
			Location: n.Source,
			Help:     fmt.Sprintf("known providers: %s", knownProviderList()),
		}}
	}
	if !providerModels[model] {
		return []Diagnostic{{
			Code:     DIP108,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("node %q uses unknown model %q for provider %q", n.ID, model, provider),
			Location: n.Source,
			Help:     fmt.Sprintf("known models for %s: %s", provider, knownModelList(provider)),
		}}
	}
	return nil
}

// knownProviderList returns a sorted comma-separated list of known providers.
func knownProviderList() string {
	providers := make([]string, 0, len(knownModelProviders))
	for p := range knownModelProviders {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return strings.Join(providers, ", ")
}

// knownModelList returns a sorted comma-separated list of known models for a provider.
func knownModelList(provider string) string {
	models := knownModelProviders[provider]
	list := make([]string, 0, len(models))
	for m := range models {
		list = append(list, m)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}

// validReasoningEfforts is the set of reasoning effort levels recognized by LLM providers.
var validReasoningEfforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

// lintReasoningEffort checks DIP119: reasoning_effort must be a recognized level.
func lintReasoningEffort(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if r := cfg.ReasoningEffort; r != "" && !validReasoningEfforts[r] {
			diags = append(diags, Diagnostic{
				Code:     DIP119,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has reasoning_effort %q which is not a recognized level", n.ID, r),
				Location: n.Source,
				Help:     "valid levels: low, medium, high",
			})
		}
	}
	return diags
}
