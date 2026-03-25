package cost

// DefaultPricing returns a PricingTable with current model prices.
func DefaultPricing() PricingTable {
	return PricingTable{
		"anthropic": {
			"claude-opus-4-6":   {InputPer1M: 15.00, OutputPer1M: 75.00},
			"claude-sonnet-4-6": {InputPer1M: 3.00, OutputPer1M: 15.00},
			"claude-haiku-4-5":  {InputPer1M: 0.80, OutputPer1M: 4.00},
			"claude-haiku-3-5":  {InputPer1M: 0.25, OutputPer1M: 1.25},
		},
		"openai": {
			"gpt-5.4":       {InputPer1M: 3.00, OutputPer1M: 15.00},
			"gpt-5.3-codex": {InputPer1M: 3.00, OutputPer1M: 15.00},
			"gpt-5.2":       {InputPer1M: 3.00, OutputPer1M: 15.00},
			"gpt-4.1":       {InputPer1M: 2.00, OutputPer1M: 8.00},
			"gpt-4.1-mini":  {InputPer1M: 0.40, OutputPer1M: 1.60},
			"gpt-4.1-nano":  {InputPer1M: 0.10, OutputPer1M: 0.40},
			"gpt-4o":        {InputPer1M: 2.50, OutputPer1M: 10.00},
			"gpt-4o-mini":   {InputPer1M: 0.15, OutputPer1M: 0.60},
			"o3":            {InputPer1M: 10.00, OutputPer1M: 40.00},
			"o3-mini":       {InputPer1M: 1.10, OutputPer1M: 4.40},
			"o3-pro":        {InputPer1M: 20.00, OutputPer1M: 80.00},
			"o4-mini":       {InputPer1M: 1.10, OutputPer1M: 4.40},
		},
		"google": {
			"gemini-3-pro":                       {InputPer1M: 1.25, OutputPer1M: 10.00},
			"gemini-3-flash-preview":             {InputPer1M: 0.15, OutputPer1M: 0.60},
			"gemini-3.1-pro-preview-customtools": {InputPer1M: 1.25, OutputPer1M: 10.00},
			"gemini-2.5-pro":                     {InputPer1M: 1.25, OutputPer1M: 10.00},
			"gemini-2.5-flash":                   {InputPer1M: 0.15, OutputPer1M: 0.60},
			"gemini-2.0-flash":                   {InputPer1M: 0.10, OutputPer1M: 0.40},
		},
		"gemini": {
			"gemini-3-pro":                       {InputPer1M: 1.25, OutputPer1M: 10.00},
			"gemini-3-flash-preview":             {InputPer1M: 0.15, OutputPer1M: 0.60},
			"gemini-3.1-pro-preview-customtools": {InputPer1M: 1.25, OutputPer1M: 10.00},
			"gemini-2.5-pro":                     {InputPer1M: 1.25, OutputPer1M: 10.00},
			"gemini-2.5-flash":                   {InputPer1M: 0.15, OutputPer1M: 0.60},
			"gemini-2.0-flash":                   {InputPer1M: 0.10, OutputPer1M: 0.40},
		},
	}
}
