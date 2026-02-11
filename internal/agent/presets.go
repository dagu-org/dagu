package agent

// modelPresets contains hardcoded model presets with metadata.
// These are shown in the frontend as "quick add" options.
// No API key or base URL is included — admin fills those in.
//
// Sources (verified 2026-02-11):
//   Anthropic: https://platform.claude.com/docs/en/docs/about-claude/models
//   OpenAI:    https://platform.openai.com/docs/models
//   Gemini:    https://ai.google.dev/gemini-api/docs/models
var modelPresets = []ModelConfig{
	// --- Anthropic ---
	// https://platform.claude.com/docs/en/docs/about-claude/models
	// https://platform.claude.com/docs/en/docs/about-claude/pricing
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
	// https://platform.openai.com/docs/models/gpt-5.2
	// https://platform.openai.com/docs/models/gpt-5-mini
	// https://platform.openai.com/docs/models/gpt-5-nano
	// https://platform.openai.com/docs/models/o3
	{Name: "GPT-5.2", Provider: "openai", Model: "gpt-5.2",
		ContextWindow: 400_000, MaxOutputTokens: 128_000,
		InputCostPer1M: 1.75, OutputCostPer1M: 14, SupportsThinking: true,
		Description: "Latest flagship GPT with reasoning. 400K context."},
	{Name: "GPT-5 mini", Provider: "openai", Model: "gpt-5-mini",
		ContextWindow: 400_000, MaxOutputTokens: 128_000,
		InputCostPer1M: 0.25, OutputCostPer1M: 2, SupportsThinking: true,
		Description: "Affordable GPT-5 with reasoning. 400K context."},
	{Name: "GPT-5 nano", Provider: "openai", Model: "gpt-5-nano",
		ContextWindow: 400_000, MaxOutputTokens: 128_000,
		InputCostPer1M: 0.05, OutputCostPer1M: 0.40, SupportsThinking: true,
		Description: "Cheapest GPT-5. 400K context."},
	{Name: "o3", Provider: "openai", Model: "o3",
		ContextWindow: 200_000, MaxOutputTokens: 100_000,
		InputCostPer1M: 2, OutputCostPer1M: 8, SupportsThinking: true,
		Description: "Reasoning specialist. 200K context."},
	// --- Google Gemini ---
	// https://ai.google.dev/gemini-api/docs/models
	// https://ai.google.dev/gemini-api/docs/pricing
	{Name: "Gemini 3 Pro", Provider: "gemini", Model: "gemini-3-pro-preview",
		ContextWindow: 1_048_576, MaxOutputTokens: 65_536,
		InputCostPer1M: 2, OutputCostPer1M: 12, SupportsThinking: true,
		Description: "Google's latest flagship. 1M context."},
	{Name: "Gemini 3 Flash", Provider: "gemini", Model: "gemini-3-flash-preview",
		ContextWindow: 1_048_576, MaxOutputTokens: 65_536,
		InputCostPer1M: 0.50, OutputCostPer1M: 3, SupportsThinking: true,
		Description: "Latest Gemini Flash. Fast and capable. 1M context."},
	{Name: "Gemini 2.5 Flash", Provider: "gemini", Model: "gemini-2.5-flash",
		ContextWindow: 1_048_576, MaxOutputTokens: 65_536,
		InputCostPer1M: 0.30, OutputCostPer1M: 2.50, SupportsThinking: true,
		Description: "Stable and fast with thinking. 1M context."},
}

// GetModelPresets returns a copy of the built-in model presets.
func GetModelPresets() []ModelConfig {
	result := make([]ModelConfig, len(modelPresets))
	copy(result, modelPresets)
	return result
}
