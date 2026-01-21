package spec

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLLM_Validation(t *testing.T) {
	ctx := BuildContext{
		opts: BuildOpts{},
	}

	tests := []struct {
		name    string
		llm     *llmConfig
		wantErr string
	}{
		{
			name: "valid single model string",
			llm: &llmConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
			wantErr: "",
		},
		{
			name: "valid model array",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider": "openai",
						"name":     "gpt-4",
					},
					map[string]any{
						"provider": "anthropic",
						"name":     "claude-sonnet-4-20250514",
					},
				},
			},
			wantErr: "",
		},
		{
			name: "nil model is valid at DAG level",
			llm: &llmConfig{
				Provider: "openai",
			},
			wantErr: "",
		},
		{
			name: "empty model array",
			llm: &llmConfig{
				Model: []any{},
			},
			wantErr: "model array must have at least one entry",
		},
		{
			name: "unsupported model type",
			llm: &llmConfig{
				Model: 123,
			},
			wantErr: "model must be a string or array of model entries",
		},
		{
			name: "invalid temperature in model entry",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider":    "openai",
						"name":        "gpt-4",
						"temperature": 2.5,
					},
				},
			},
			wantErr: "temperature must be between 0.0 and 2.0",
		},
		{
			name: "int64 temperature validation",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider":    "openai",
						"name":        "gpt-4",
						"temperature": int64(3),
					},
				},
			},
			wantErr: "temperature must be between 0.0 and 2.0",
		},
		{
			name: "invalid topP in model entry",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider": "openai",
						"name":     "gpt-4",
						"topP":     1.5,
					},
				},
			},
			wantErr: "topP must be between 0.0 and 1.0",
		},
		{
			name: "invalid maxTokens in model entry",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider":  "openai",
						"name":      "gpt-4",
						"maxTokens": 0,
					},
				},
			},
			wantErr: "maxTokens must be at least 1",
		},
		{
			name: "uint64 maxTokens overflow",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider":  "openai",
						"name":      "gpt-4",
						"maxTokens": uint64(math.MaxInt64) + 1,
					},
				},
			},
			wantErr: "maxTokens must be an integer",
		},
		{
			name: "missing provider in model entry",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"name": "gpt-4",
					},
				},
			},
			wantErr: "provider is required for each model entry",
		},
		{
			name: "missing name in model entry",
			llm: &llmConfig{
				Model: []any{
					map[string]any{
						"provider": "openai",
					},
				},
			},
			wantErr: "name is required for each model entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildLLM(ctx, &dag{LLM: tt.llm})
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
