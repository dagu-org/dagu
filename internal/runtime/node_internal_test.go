// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

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
