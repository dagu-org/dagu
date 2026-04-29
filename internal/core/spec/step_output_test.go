// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected *outputConfig
		wantErr  string
	}{
		{
			name:     "NilOutput",
			input:    nil,
			expected: nil,
		},
		{
			name:  "StringOutput",
			input: "RESULT",
			expected: &outputConfig{
				Name: "RESULT",
			},
		},
		{
			name: "StructuredLiteralObject",
			input: map[string]any{
				"meta": map[string]any{
					"version": "v1.2.3",
					"env":     "stg",
				},
			},
			expected: &outputConfig{
				StructuredOutput: map[string]core.StepOutputEntry{
					"meta": {
						HasValue: true,
						Value: map[string]any{
							"version": "v1.2.3",
							"env":     "stg",
						},
					},
				},
			},
		},
		{
			name: "StructuredSourceEntry",
			input: map[string]any{
				"version": map[string]any{
					"from":   "stdout",
					"decode": "json",
					"select": ".version",
				},
			},
			expected: &outputConfig{
				StructuredOutput: map[string]core.StepOutputEntry{
					"version": {
						From:   core.StepOutputSourceStdout,
						Decode: core.StepOutputDecodeJSON,
						Select: ".version",
					},
				},
			},
		},
		{
			name: "StructuredValueEntry",
			input: map[string]any{
				"label": map[string]any{
					"value": "ver - ${build.output.version}",
				},
			},
			expected: &outputConfig{
				StructuredOutput: map[string]core.StepOutputEntry{
					"label": {
						HasValue: true,
						Value:    "ver - ${build.output.version}",
					},
				},
			},
		},
		{
			name:    "InvalidOutputType",
			input:   123,
			wantErr: "output must be a string or object",
		},
		{
			name: "InvalidSourceField",
			input: map[string]any{
				"version": map[string]any{
					"from": "network",
				},
			},
			wantErr: `output.version: from must be one of "stdout", "stderr", or "file"`,
		},
		{
			name: "SelectRequiresStructuredDecode",
			input: map[string]any{
				"version": map[string]any{
					"from":   "stdout",
					"decode": "text",
					"select": ".version",
				},
			},
			wantErr: `output.version: select requires decode to be "json" or "yaml"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := parseOutputConfig(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestBuildStepStructuredOutput(t *testing.T) {
	t.Parallel()

	s := &step{
		Output: map[string]any{
			"meta": map[string]any{
				"from":   "stdout",
				"decode": "json",
			},
			"label": "ver - ${build.output.version}",
		},
	}

	result, err := buildStepStructuredOutput(testStepBuildContext(), s)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, core.StepOutputSourceStdout, result["meta"].From)
	assert.Equal(t, core.StepOutputDecodeJSON, result["meta"].Decode)
	assert.True(t, result["label"].HasValue)
	assert.Equal(t, "ver - ${build.output.version}", result["label"].Value)
}

func TestBuildStepExecutorInfersNoopForStructuredOutput(t *testing.T) {
	t.Parallel()

	s := &step{
		Output: map[string]any{
			"versionLabel": "ver - ${build.output.version}",
		},
	}
	result := &core.Step{
		ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)},
		StructuredOutput: map[string]core.StepOutputEntry{
			"versionLabel": {HasValue: true, Value: "ver - ${build.output.version}"},
		},
	}

	err := buildStepExecutor(testStepBuildContext(), s, result)
	require.NoError(t, err)
	assert.Equal(t, "noop", result.ExecutorConfig.Type)
}
