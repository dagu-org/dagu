package agent

// ModelPresets contains hardcoded model presets with metadata.
// These are shown in the frontend as "quick add" options.
// No API key or base URL is included — admin fills those in.
var ModelPresets = []ModelConfig{
	// --- Anthropic ---
	{Name: "Claude Opus 4.6", Provider: "anthropic", Model: "claude-opus-4-6",
		ContextWindow: 200_000, MaxOutputTokens: 128_000,
		InputCostPer1M: 5, OutputCostPer1M: 25, SupportsThinking: true,
		Description: "Most intelligent. Best for complex agents and coding."},
	{Name: "Claude Sonnet 4.5", Provider: "anthropic", Model: "claude-sonnet-4-5",
		ContextWindow: 200_000, MaxOutputTokens: 64_000,
		InputCostPer1M: 3, OutputCostPer1M: 15, SupportsThinking: true,
		Description: "Best balance of speed and intelligence."},
	{Name: "Claude Haiku 4.5", Provider: "anthropic", Model: "claude-haiku-4-5",
		ContextWindow: 200_000, MaxOutputTokens: 64_000,
		InputCostPer1M: 1, OutputCostPer1M: 5, SupportsThinking: true,
		Description: "Fastest with near-frontier intelligence."},
	// --- OpenAI ---
	{Name: "GPT-4.1", Provider: "openai", Model: "gpt-4.1",
		ContextWindow: 1_000_000, MaxOutputTokens: 32_768,
		InputCostPer1M: 2, OutputCostPer1M: 8, SupportsThinking: false,
		Description: "Flagship GPT model. 1M context."},
	{Name: "GPT-4.1 mini", Provider: "openai", Model: "gpt-4.1-mini",
		ContextWindow: 1_000_000, MaxOutputTokens: 32_768,
		InputCostPer1M: 0.4, OutputCostPer1M: 1.6, SupportsThinking: false,
		Description: "Fast and affordable. 1M context."},
	{Name: "GPT-4.1 nano", Provider: "openai", Model: "gpt-4.1-nano",
		ContextWindow: 1_000_000, MaxOutputTokens: 32_768,
		InputCostPer1M: 0.1, OutputCostPer1M: 0.4, SupportsThinking: false,
		Description: "Cheapest GPT. 1M context."},
	{Name: "o3", Provider: "openai", Model: "o3",
		ContextWindow: 200_000, MaxOutputTokens: 100_000,
		InputCostPer1M: 2, OutputCostPer1M: 8, SupportsThinking: true,
		Description: "Advanced reasoning model."},
	{Name: "o4-mini", Provider: "openai", Model: "o4-mini",
		ContextWindow: 200_000, MaxOutputTokens: 100_000,
		InputCostPer1M: 1.1, OutputCostPer1M: 4.4, SupportsThinking: true,
		Description: "Fast reasoning model."},
	// --- Google Gemini ---
	{Name: "Gemini 2.5 Pro", Provider: "gemini", Model: "gemini-2.5-pro",
		ContextWindow: 1_000_000, MaxOutputTokens: 65_536,
		InputCostPer1M: 1.25, OutputCostPer1M: 10, SupportsThinking: true,
		Description: "Google's most capable model. 1M context."},
	{Name: "Gemini 2.5 Flash", Provider: "gemini", Model: "gemini-2.5-flash",
		ContextWindow: 1_000_000, MaxOutputTokens: 65_536,
		InputCostPer1M: 0.3, OutputCostPer1M: 2.5, SupportsThinking: true,
		Description: "Fast and cheap with thinking. 1M context."},
}
