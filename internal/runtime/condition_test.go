package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func newTestContext() context.Context {
	ctx := context.Background()
	return runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
}

func TestEvalConditions(t *testing.T) {
	tests := []struct {
		name                string
		conditions          []*core.Condition
		wantErr             bool
		wantConditionNotMet bool // true if error should be ErrConditionNotMet
		notConditionNotMet  bool // true if error should NOT be ErrConditionNotMet
	}{
		{
			name:       "CommandSubstitution",
			conditions: []*core.Condition{{Condition: "`echo 1`", Expected: "1"}},
		},
		{
			name:       "EnvVar",
			conditions: []*core.Condition{{Condition: "${TEST_CONDITION}", Expected: "100"}},
		},
		{
			name: "MultipleCond",
			conditions: []*core.Condition{
				{Condition: "`echo 1`", Expected: "1"},
				{Condition: "`echo 100`", Expected: "100"},
			},
		},
		{
			name: "MultipleCondOneMet",
			conditions: []*core.Condition{
				{Condition: "`echo 1`", Expected: "1"},
				{Condition: "`echo 100`", Expected: "1"},
			},
			wantErr:             true,
			wantConditionNotMet: true,
		},
		{
			name:       "CommandResultMet",
			conditions: []*core.Condition{{Condition: "true"}},
		},
		{
			name:                "CommandResultNotMet",
			conditions:          []*core.Condition{{Condition: "false"}},
			wantErr:             true,
			wantConditionNotMet: true,
		},
		{
			name:       "ComplexCommand",
			conditions: []*core.Condition{{Condition: "test 1 -eq 1"}},
		},
		{
			name:       "EvenMoreComplexCommand",
			conditions: []*core.Condition{{Condition: "df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'"}},
		},
		{
			name:       "CommandResultTest",
			conditions: []*core.Condition{{Condition: "test 1 -eq 1"}},
		},
		{
			name:       "RegexMatch",
			conditions: []*core.Condition{{Condition: "test", Expected: "re:^test$"}},
		},
		// Negate tests
		{
			name: "NegateMatchingCondition",
			conditions: []*core.Condition{
				{Condition: "`echo success`", Expected: "success", Negate: true},
			},
			wantErr:             true,
			wantConditionNotMet: true,
		},
		{
			name: "NegateNonMatchingCondition",
			conditions: []*core.Condition{
				{Condition: "`echo failure`", Expected: "success", Negate: true},
			},
		},
		{
			name: "NegateCommandSuccess",
			conditions: []*core.Condition{
				{Condition: "true", Negate: true},
			},
			wantErr:             true,
			wantConditionNotMet: true,
		},
		{
			name: "NegateCommandFailure",
			conditions: []*core.Condition{
				{Condition: "false", Negate: true},
			},
		},
		{
			name: "NegateEnvVar",
			conditions: []*core.Condition{
				{Condition: "${TEST_CONDITION}", Expected: "wrong_value", Negate: true},
			},
		},
		{
			name: "NegateEnvVarMatching",
			conditions: []*core.Condition{
				{Condition: "${TEST_CONDITION}", Expected: "100", Negate: true},
			},
			wantErr:             true,
			wantConditionNotMet: true,
		},
		// Error handling tests
		{
			name: "EvalStringErrorNotSwallowed",
			conditions: []*core.Condition{
				{
					Condition: "`/nonexistent_binary_xyz_123_456`",
					Expected:  "anything",
					Negate:    true,
				},
			},
			wantErr:            true,
			notConditionNotMet: true,
		},
		{
			name: "CommandNotFoundInvertedToSuccess",
			conditions: []*core.Condition{
				{
					Condition: "/nonexistent/path/to/command_xyz_123_abc",
					Negate:    true,
				},
			},
		},
		{
			name: "FalseCommandInvertedToSuccess",
			conditions: []*core.Condition{
				{
					Condition: "false",
					Negate:    true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestContext()
			// Add TEST_CONDITION to the env scope (not OS env)
			env := runtime.GetEnv(ctx)
			env.Scope = env.Scope.WithEntry("TEST_CONDITION", "100", eval.EnvSourceDAGEnv)
			ctx = runtime.WithEnv(ctx, env)
			err := runtime.EvalConditions(ctx, []string{"sh"}, tt.conditions)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantConditionNotMet {
					require.True(t, errors.Is(err, runtime.ErrConditionNotMet),
						"expected ErrConditionNotMet but got: %v", err)
				}
				if tt.notConditionNotMet {
					require.False(t, errors.Is(err, runtime.ErrConditionNotMet),
						"evaluation errors should not be wrapped as ErrConditionNotMet")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
