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
//
// Last verified: 2026-04-17
//
// Sources:
//
//	Anthropic:  https://platform.claude.com/docs/en/docs/about-claude/models
//	Google:     https://ai.google.dev/gemini-api/docs/models
//	OpenAI:     https://developers.openai.com/api/docs/pricing
//	DeepSeek:   https://api-docs.deepseek.com/quick_start/pricing
//	xAI (Grok): https://docs.x.ai/developers/models
//	Mistral:    https://mistral.ai/pricing
//	Cohere:     https://cohere.com/pricing
var knownModelProviders = map[string]map[string]bool{
	"anthropic": {
		"claude-opus-4-7":   true,
		"claude-opus-4-6":   true,
		"claude-sonnet-4-6": true,
		"claude-haiku-4-5":  true,
		// Legacy models still available via API.
		"claude-sonnet-4-5": true,
		"claude-opus-4-5":   true,
		"claude-opus-4-1":   true,
		"claude-sonnet-4-0": true,
		"claude-opus-4-0":   true,
		// Deprecated — retired 2026-02-19.
		"claude-haiku-3-5": true,
	},
	"google": geminiModels(),
	"gemini": geminiModels(),
	"openai": {
		"gpt-5.4":       true,
		"gpt-5.4-mini":  true,
		"gpt-5.4-nano":  true,
		"gpt-5.3-codex": true,
		"gpt-5.2":       true,
		"gpt-5.1":       true,
		"gpt-4.1":       true,
		"gpt-4.1-mini":  true,
		"gpt-4.1-nano":  true,
		"gpt-4o":        true,
		"gpt-4o-mini":   true,
		"o3":            true,
		"o3-mini":       true,
		"o3-pro":        true,
		"o4-mini":       true,
	},
	"deepseek": {
		"deepseek-chat":     true,
		"deepseek-reasoner": true,
	},
	"xai":  grokModels(),
	"grok": grokModels(),
	"mistral": {
		"mistral-large-3":    true,
		"mistral-medium-3":   true,
		"mistral-small-2603": true, // Mistral Small 4 (March 2026)
		"mistral-small-3.2":  true,
		"mistral-small":      true,
		"ministral-8b":       true,
		"codestral":          true,
		"magistral-medium":   true,
		"mistral-nemo":       true,
		"pixtral-large":      true,
	},
	"cohere": {
		"command-a-03-2025": true, // Current flagship
		"command-r-plus":    true,
		"command-r":         true,
		"command-r7b":       true,
	},
}

// geminiModels returns the set of known Gemini model IDs.
func geminiModels() map[string]bool {
	return map[string]bool{
		// Gemini 3.x
		"gemini-3.1-pro-preview":             true,
		"gemini-3.1-pro-preview-customtools": true,
		"gemini-3-flash-preview":             true,
		"gemini-3.1-flash-lite-preview":      true,
		// Gemini 3 Pro (shut down 2026-03-09, replaced by 3.1)
		"gemini-3-pro-preview": true,
		// Gemini 2.x
		"gemini-2.5-pro":        true,
		"gemini-2.5-flash":      true,
		"gemini-2.5-flash-lite": true,
		"gemini-2.0-flash":      true,
	}
}

// grokModels returns the set of known xAI Grok model IDs.
func grokModels() map[string]bool {
	return map[string]bool{
		"grok-4.20-0309-reasoning":     true,
		"grok-4.20-0309-non-reasoning": true,
		"grok-4-1-fast-reasoning":      true,
		"grok-4-1-fast-non-reasoning":  true,
		"grok-4.20-multi-agent-0309":   true,
	}
}

// RegisterExtraModels extends the known model catalog with user-provided entries.
// Format: "provider:model1,model2;provider2:model3"
func RegisterExtraModels(spec string) {
	for _, entry := range strings.Split(spec, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		registerOneProvider(entry)
	}
}

// registerOneProvider parses a single "provider:model1,model2" entry and adds
// the models to the known catalog. Silently ignores entries with empty provider
// or no valid model names.
func registerOneProvider(entry string) {
	parts := strings.SplitN(entry, ":", 2)
	if len(parts) != 2 {
		return
	}
	provider := strings.TrimSpace(parts[0])
	if provider == "" {
		return
	}
	models := parseModelNames(parts[1])
	if len(models) == 0 {
		return
	}
	addModelsToProvider(provider, models)
}

// addModelsToProvider registers models under the given provider name.
func addModelsToProvider(provider string, models []string) {
	if knownModelProviders[provider] == nil {
		knownModelProviders[provider] = make(map[string]bool)
	}
	for _, m := range models {
		knownModelProviders[provider][m] = true
	}
}

// parseModelNames splits a comma-separated model list, trimming whitespace
// and discarding empty entries.
func parseModelNames(raw string) []string {
	var models []string
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			models = append(models, m)
		}
	}
	return models
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
