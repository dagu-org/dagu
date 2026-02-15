package command

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func getEvalOptions(t *testing.T, step core.Step) *eval.Options {
	t.Helper()

	ctx := runtime.NewContextForTest(context.Background(), &core.DAG{Name: "test-dag"}, "run-1", "test.log")
	env := runtime.NewEnv(ctx, step)
	ctx = runtime.WithEnv(ctx, env)

	opts := eval.NewOptions()
	for _, opt := range step.EvalOptions(ctx) {
		opt(opts)
	}

	return opts
}

func TestCommandExecutor_GetEvalOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		shell         string
		wantExpandEnv bool
		wantExpandOS  bool
		wantEscape    bool
	}{
		{
			name:          "EmptyShellDefaultsToUnix",
			shell:         "",
			wantExpandEnv: false,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "DirectShellUsesOSExpansion",
			shell:         "direct",
			wantExpandEnv: true,
			wantExpandOS:  true,
			wantEscape:    true,
		},
		{
			name:          "ShDisablesBoltbaseEnvExpansion",
			shell:         "/bin/sh",
			wantExpandEnv: false,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "BashDisablesBoltbaseEnvExpansion",
			shell:         "/bin/bash",
			wantExpandEnv: false,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "ZshDisablesBoltbaseEnvExpansion",
			shell:         "/bin/zsh",
			wantExpandEnv: false,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "FishKeepsBoltbaseEnvExpansion",
			shell:         "fish",
			wantExpandEnv: true,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "NuKeepsBoltbaseEnvExpansion",
			shell:         "nu",
			wantExpandEnv: true,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "PowerShellKeepsBoltbaseEnvExpansion",
			shell:         "powershell",
			wantExpandEnv: true,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "PwshKeepsBoltbaseEnvExpansion",
			shell:         "pwsh",
			wantExpandEnv: true,
			wantExpandOS:  false,
			wantEscape:    false,
		},
		{
			name:          "CmdKeepsBoltbaseEnvExpansion",
			shell:         "cmd.exe",
			wantExpandEnv: true,
			wantExpandOS:  false,
			wantEscape:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := core.Step{
				Shell:          tt.shell,
				ExecutorConfig: core.ExecutorConfig{Type: "command"},
			}
			opts := getEvalOptions(t, step)

			require.Equal(t, tt.wantExpandEnv, opts.ExpandEnv)
			require.Equal(t, tt.wantExpandOS, opts.ExpandOS)
			require.Equal(t, tt.wantEscape, opts.EscapeDollar)
		})
	}
}
