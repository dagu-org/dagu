// Package allproviders imports all LLM providers to register them.
// Import this package if you want all providers to be available:
//
//	import _ "github.com/dagu-org/dagu/internal/llm/allproviders"
package allproviders

import (
	_ "github.com/dagu-org/dagu/internal/llm/providers/anthropic"
	_ "github.com/dagu-org/dagu/internal/llm/providers/gemini"
	_ "github.com/dagu-org/dagu/internal/llm/providers/local"
	_ "github.com/dagu-org/dagu/internal/llm/providers/openai"
	_ "github.com/dagu-org/dagu/internal/llm/providers/openrouter"
)
