package jq

import (
	"bytes"
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJQExecutor_RawOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		script         string
		raw            bool
		expectedOutput string
	}{
		{
			name:           "SingleStringWithRawTrue",
			query:          ".foo",
			script:         `{"foo": "bar"}`,
			raw:            true,
			expectedOutput: "bar\n",
		},
		{
			name:           "SingleStringWithRawFalse",
			query:          ".foo",
			script:         `{"foo": "bar"}`,
			raw:            false,
			expectedOutput: "\"bar\"\n",
		},
		{
			name:           "SingleNumberWithRawTrue",
			query:          ".number",
			script:         `{"number": 42}`,
			raw:            true,
			expectedOutput: "42\n",
		},
		{
			name:           "SingleNumberWithRawFalse",
			query:          ".number",
			script:         `{"number": 42}`,
			raw:            false,
			expectedOutput: "42\n",
		},
		{
			name:           "SingleBooleanTrueWithRawTrue",
			query:          ".flag",
			script:         `{"flag": true}`,
			raw:            true,
			expectedOutput: "true\n",
		},
		{
			name:           "SingleBooleanFalseWithRawTrue",
			query:          ".flag",
			script:         `{"flag": false}`,
			raw:            true,
			expectedOutput: "false\n",
		},
		{
			name:           "NullValueWithRawTrue",
			query:          ".nullValue",
			script:         `{"nullValue": null}`,
			raw:            true,
			expectedOutput: "\n",
		},
		{
			name:           "ObjectWithRawTrue",
			query:          ".",
			script:         `{"foo": "bar", "baz": 123}`,
			raw:            true,
			expectedOutput: "", // Key order is not guaranteed, will check separately
		},
		{
			name:           "ObjectWithRawFalse",
			query:          ".",
			script:         `{"foo": "bar", "baz": 123}`,
			raw:            false,
			expectedOutput: "", // Key order is not guaranteed, will check separately
		},
		{
			name:           "ArrayFromObjectWithRawTrue",
			query:          ".items",
			script:         `{"items": [1, 2, 3]}`,
			raw:            true,
			expectedOutput: "[1,2,3]\n",
		},
		{
			name:           "ArrayFromObjectWithRawFalse",
			query:          ".items",
			script:         `{"items": [1, 2, 3]}`,
			raw:            false,
			expectedOutput: "[\n    1,\n    2,\n    3\n]\n",
		},
		{
			name:           "StringWithSpecialCharsWithRawTrue",
			query:          ".message",
			script:         `{"message": "hello\nworld"}`,
			raw:            true,
			expectedOutput: "hello\nworld\n",
		},
		{
			name:           "StringWithTabsWithRawTrue",
			query:          ".data",
			script:         `{"data": "a\tb\tc"}`,
			raw:            true,
			expectedOutput: "a\tb\tc\n",
		},
		{
			name:           "FloatNumberWithRawTrue",
			query:          ".pi",
			script:         `{"pi": 3.14159}`,
			raw:            true,
			expectedOutput: "3.14159\n",
		},
		{
			name:           "NegativeNumberWithRawTrue",
			query:          ".temp",
			script:         `{"temp": -10}`,
			raw:            true,
			expectedOutput: "-10\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			step := core.Step{
				CmdWithArgs: tt.query,
				Script:      tt.script,
				ExecutorConfig: core.ExecutorConfig{
					Type: "jq",
					Config: map[string]any{
						"raw": tt.raw,
					},
				},
			}

			ctx := context.Background()
			executor, err := newJQ(ctx, step)
			require.NoError(t, err)

			executor.SetStdout(&stdout)
			executor.SetStderr(&stderr)

			err = executor.Run(ctx)
			require.NoError(t, err)

			// Special handling for object outputs where key order is not guaranteed
			if tt.name == "ObjectWithRawTrue" {
				output := stdout.String()
				assert.Contains(t, output, `"foo":"bar"`)
				assert.Contains(t, output, `"baz":123`)
				assert.Contains(t, output, "{")
				assert.Contains(t, output, "}")
			} else if tt.name == "ObjectWithRawFalse" {
				output := stdout.String()
				assert.Contains(t, output, `"foo": "bar"`)
				assert.Contains(t, output, `"baz": 123`)
				assert.Contains(t, output, "{")
				assert.Contains(t, output, "}")
			} else {
				assert.Equal(t, tt.expectedOutput, stdout.String())
			}
		})
	}
}

func TestJQExecutor_MultipleOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		script         string
		raw            bool
		expectedOutput string
	}{
		{
			name:           "MultipleStringsWithRawTrue",
			query:          ".items[]",
			script:         `{"items": ["foo", "bar", "baz"]}`,
			raw:            true,
			expectedOutput: "foo\nbar\nbaz\n",
		},
		{
			name:           "MultipleStringsWithRawFalse",
			query:          ".items[]",
			script:         `{"items": ["foo", "bar", "baz"]}`,
			raw:            false,
			expectedOutput: "\"foo\"\n\"bar\"\n\"baz\"\n",
		},
		{
			name:           "MultipleNumbersWithRawTrue",
			query:          ".numbers[]",
			script:         `{"numbers": [1, 2, 3]}`,
			raw:            true,
			expectedOutput: "1\n2\n3\n",
		},
		{
			name:           "MixedTypesWithRawTrue",
			query:          ".values[]",
			script:         `{"values": ["text", 42, true, null]}`,
			raw:            true,
			expectedOutput: "text\n42\ntrue\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			step := core.Step{
				CmdWithArgs: tt.query,
				Script:      tt.script,
				ExecutorConfig: core.ExecutorConfig{
					Type: "jq",
					Config: map[string]any{
						"raw": tt.raw,
					},
				},
			}

			ctx := context.Background()
			executor, err := newJQ(ctx, step)
			require.NoError(t, err)

			executor.SetStdout(&stdout)
			executor.SetStderr(&stderr)

			err = executor.Run(ctx)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedOutput, stdout.String())
		})
	}
}

func TestJQExecutor_InvalidQuery(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	step := core.Step{
		CmdWithArgs: "invalid jq syntax {",
		Script:      `{"foo": "bar"}`,
		ExecutorConfig: core.ExecutorConfig{
			Type: "jq",
			Config: map[string]any{
				"raw": true,
			},
		},
	}

	ctx := context.Background()
	executor, err := newJQ(ctx, step)
	require.NoError(t, err)

	executor.SetStdout(&stdout)
	executor.SetStderr(&stderr)

	err = executor.Run(ctx)
	assert.Error(t, err)
}

func TestJQExecutor_InvalidJSON(t *testing.T) {
	t.Parallel()

	step := core.Step{
		CmdWithArgs: ".foo",
		Script:      `invalid json`,
		ExecutorConfig: core.ExecutorConfig{
			Type: "jq",
			Config: map[string]any{
				"raw": true,
			},
		},
	}

	ctx := context.Background()
	_, err := newJQ(ctx, step)
	assert.Error(t, err)
}

func TestJQExecutor_NoConfig(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	step := core.Step{
		CmdWithArgs: ".foo",
		Script:      `{"foo": "bar"}`,
		ExecutorConfig: core.ExecutorConfig{
			Type:   "jq",
			Config: nil, // No config, should default to raw: false
		},
	}

	ctx := context.Background()
	executor, err := newJQ(ctx, step)
	require.NoError(t, err)

	executor.SetStdout(&stdout)

	err = executor.Run(ctx)
	require.NoError(t, err)

	// Without raw config, should output JSON formatted
	assert.Contains(t, stdout.String(), "\"bar\"")
}

func TestJQExecutor_Kill(t *testing.T) {
	t.Parallel()

	step := core.Step{
		CmdWithArgs: ".foo",
		Script:      `{"foo": "bar"}`,
		ExecutorConfig: core.ExecutorConfig{
			Type: "jq",
		},
	}

	ctx := context.Background()
	executor, err := newJQ(ctx, step)
	require.NoError(t, err)

	// Kill should return nil (jq executor doesn't need cleanup)
	err = executor.Kill(nil)
	assert.NoError(t, err)
}

func TestDecodeJqConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   map[string]any
		expected jqConfig
	}{
		{
			name: "RawTrue",
			config: map[string]any{
				"raw": true,
			},
			expected: jqConfig{
				Raw: true,
			},
		},
		{
			name: "RawFalse",
			config: map[string]any{
				"raw": false,
			},
			expected: jqConfig{
				Raw: false,
			},
		},
		{
			name: "RawAsString",
			config: map[string]any{
				"raw": "true",
			},
			expected: jqConfig{
				Raw: true,
			},
		},
		{
			name:     "EmptyConfig",
			config:   map[string]any{},
			expected: jqConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg jqConfig
			err := decodeJqConfig(tt.config, &cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
