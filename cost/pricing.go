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
			"gpt-5.2":       {InputPer1M: 3.00, OutputPer1M: 15.00},
			"gpt-5.4":       {InputPer1M: 3.00, OutputPer1M: 15.00},
			"gpt-5.3-codex": {InputPer1M: 3.00, OutputPer1M: 15.00},
			"gpt-4o":        {InputPer1M: 2.50, OutputPer1M: 10.00},
			"gpt-4o-mini":   {InputPer1M: 0.15, OutputPer1M: 0.60},
		},
	}
}
