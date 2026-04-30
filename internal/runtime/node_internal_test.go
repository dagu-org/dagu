// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/internal/cmn/eval"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

// TestEvalExecutorConfig_TemplateTreatsOmittedOptionalParamsAsEmpty verifies
// that optional named params are coerced to empty strings for template config
// evaluation instead of being left as unresolved placeholders.
func TestEvalExecutorConfig_TemplateTreatsOmittedOptionalParamsAsEmpty(t *testing.T) {
	t.Parallel()

	ctx := exec.NewContext(
		context.Background(),
		&core.DAG{
			Name: "test-dag",
			ParamDefs: []core.ParamDef{
				{Name: "name", Type: core.ParamDefTypeString, Required: true},
				{Name: "favorite_color", Type: core.ParamDefTypeString},
			},
		},
		"",
		"",
		exec.WithParams([]string{"name=tom"}),
	)
	env := NewEnv(ctx, core.Step{Name: "render"})
	ctx = WithEnv(ctx, env)

	result, err := evalExecutorConfig(ctx, core.Step{
		ExecutorConfig: core.ExecutorConfig{
			Type: "template",
			Config: map[string]any{
				"data": map[string]any{
					"name":           "${name}",
					"favorite_color": "${favorite_color}",
				},
			},
		},
	})
	require.NoError(t, err)

	data, ok := result["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tom", data["name"])
	require.Equal(t, "", data["favorite_color"])
}

// TestEvalExecutorConfig_TemplatePreservesLiteralCodeFencesInData verifies that
// template config data can carry fenced content without backtick substitution
// executing it during evaluator setup.
func TestEvalExecutorConfig_TemplatePreservesLiteralCodeFencesInData(t *testing.T) {
	t.Parallel()

	ctx := exec.NewContext(
		context.Background(),
		&core.DAG{Name: "test-dag"},
		"",
		"",
	)
	env := NewEnv(ctx, core.Step{Name: "render"})
	env.Scope = env.Scope.WithEntries(map[string]string{
		"ISSUE_TEXT": "```yaml\nenv:\n  TEST_FILE: ~/dagu-test.txt\n\nsteps:\n  - command: touch $TEST_FILE\n```",
	}, eval.EnvSourceStepEnv)
	ctx = WithEnv(ctx, env)

	result, err := evalExecutorConfig(ctx, core.Step{
		ExecutorConfig: core.ExecutorConfig{
			Type: "template",
			Config: map[string]any{
				"data": map[string]any{
					"issue_text": "${ISSUE_TEXT}",
				},
			},
		},
	})
	require.NoError(t, err)

	data, ok := result["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "```yaml\nenv:\n  TEST_FILE: ~/dagu-test.txt\n\nsteps:\n  - command: touch $TEST_FILE\n```", data["issue_text"])
}
