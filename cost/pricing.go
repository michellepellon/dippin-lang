package cost

// DefaultPricing returns a PricingTable with current model prices (USD per 1M tokens).
//
// Sources:
//
//	Anthropic:  https://platform.claude.com/docs/en/docs/about-claude/models
//	Google:     https://ai.google.dev/gemini-api/docs/pricing
//	OpenAI:     https://developers.openai.com/api/docs/pricing
//	DeepSeek:   https://api-docs.deepseek.com/quick_start/pricing
//	xAI (Grok): https://docs.x.ai/developers/models
//	Mistral:    https://mistral.ai/pricing
//	Cohere:     https://cohere.com/pricing
func DefaultPricing() PricingTable {
	gemini := geminiPricing()
	grok := grokPricing()
	return PricingTable{
		"anthropic": {
			"claude-opus-4-6":   {InputPer1M: 5.00, OutputPer1M: 25.00},
			"claude-sonnet-4-6": {InputPer1M: 3.00, OutputPer1M: 15.00},
			"claude-haiku-4-5":  {InputPer1M: 1.00, OutputPer1M: 5.00},
			"claude-sonnet-4-5": {InputPer1M: 3.00, OutputPer1M: 15.00},
			"claude-opus-4-5":   {InputPer1M: 5.00, OutputPer1M: 25.00},
			"claude-opus-4-1":   {InputPer1M: 15.00, OutputPer1M: 75.00},
			"claude-sonnet-4-0": {InputPer1M: 3.00, OutputPer1M: 15.00},
			"claude-opus-4-0":   {InputPer1M: 15.00, OutputPer1M: 75.00},
			// Deprecated — retires 2026-04-19.
			"claude-haiku-3-5": {InputPer1M: 0.25, OutputPer1M: 1.25},
		},
		"openai": {
			"gpt-5.4":       {InputPer1M: 2.50, OutputPer1M: 15.00},
			"gpt-5.4-mini":  {InputPer1M: 0.75, OutputPer1M: 4.50},
			"gpt-5.4-nano":  {InputPer1M: 0.20, OutputPer1M: 1.25},
			"gpt-5.3-codex": {InputPer1M: 1.75, OutputPer1M: 14.00},
			"gpt-5.2":       {InputPer1M: 0.875, OutputPer1M: 7.00},
			"gpt-5.1":       {InputPer1M: 0.625, OutputPer1M: 5.00},
			"gpt-4.1":       {InputPer1M: 2.00, OutputPer1M: 8.00},
			"gpt-4.1-mini":  {InputPer1M: 0.20, OutputPer1M: 0.80},
			"gpt-4.1-nano":  {InputPer1M: 0.05, OutputPer1M: 0.20},
			"gpt-4o":        {InputPer1M: 2.50, OutputPer1M: 10.00},
			"gpt-4o-mini":   {InputPer1M: 0.15, OutputPer1M: 0.60},
			"o3":            {InputPer1M: 2.00, OutputPer1M: 8.00},
			"o3-mini":       {InputPer1M: 1.10, OutputPer1M: 4.40},
			"o3-pro":        {InputPer1M: 20.00, OutputPer1M: 80.00},
			"o4-mini":       {InputPer1M: 1.10, OutputPer1M: 4.40},
		},
		"google": gemini,
		"gemini": gemini,
		"deepseek": {
			"deepseek-chat":     {InputPer1M: 0.28, OutputPer1M: 0.42},
			"deepseek-reasoner": {InputPer1M: 0.28, OutputPer1M: 0.42},
		},
		"xai":  grok,
		"grok": grok,
		"mistral": {
			"mistral-large-3":   {InputPer1M: 0.50, OutputPer1M: 1.50},
			"mistral-medium-3":  {InputPer1M: 0.40, OutputPer1M: 2.00},
			"mistral-small-3.2": {InputPer1M: 0.075, OutputPer1M: 0.20},
			"mistral-small":     {InputPer1M: 0.10, OutputPer1M: 0.30},
			"ministral-8b":      {InputPer1M: 0.10, OutputPer1M: 0.10},
			"codestral":         {InputPer1M: 0.30, OutputPer1M: 0.90},
			"magistral-medium":  {InputPer1M: 2.00, OutputPer1M: 5.00},
			"mistral-nemo":      {InputPer1M: 0.02, OutputPer1M: 0.04},
			"pixtral-large":     {InputPer1M: 2.00, OutputPer1M: 6.00},
		},
		"cohere": {
			"command-r-plus": {InputPer1M: 2.50, OutputPer1M: 10.00},
			"command-r":      {InputPer1M: 0.50, OutputPer1M: 1.50},
			"command-r7b":    {InputPer1M: 0.0375, OutputPer1M: 0.15},
		},
	}
}

// grokPricing returns pricing for xAI Grok models.
func grokPricing() map[string]ModelPrice {
	return map[string]ModelPrice{
		"grok-4.20-0309-reasoning":     {InputPer1M: 2.00, OutputPer1M: 6.00},
		"grok-4.20-0309-non-reasoning": {InputPer1M: 2.00, OutputPer1M: 6.00},
		"grok-4-1-fast-reasoning":      {InputPer1M: 0.20, OutputPer1M: 0.50},
		"grok-4-1-fast-non-reasoning":  {InputPer1M: 0.20, OutputPer1M: 0.50},
		"grok-4.20-multi-agent-0309":   {InputPer1M: 2.00, OutputPer1M: 6.00},
	}
}

// geminiPricing returns pricing for all known Gemini models.
func geminiPricing() map[string]ModelPrice {
	return map[string]ModelPrice{
		"gemini-3.1-pro-preview":        {InputPer1M: 2.00, OutputPer1M: 12.00},
		"gemini-3-flash-preview":        {InputPer1M: 0.50, OutputPer1M: 3.00},
		"gemini-3.1-flash-lite-preview": {InputPer1M: 0.25, OutputPer1M: 1.50},
		"gemini-2.5-pro":                {InputPer1M: 1.25, OutputPer1M: 10.00},
		"gemini-2.5-flash":              {InputPer1M: 0.30, OutputPer1M: 2.50},
		"gemini-2.5-flash-lite":         {InputPer1M: 0.10, OutputPer1M: 0.40},
		"gemini-2.0-flash":              {InputPer1M: 0.10, OutputPer1M: 0.40},
	}
}
