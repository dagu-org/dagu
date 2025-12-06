package runtime_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func TestCondition_Eval(t *testing.T) {
	tests := []struct {
		name      string
		condition []*core.Condition
		wantErr   bool
	}{
		{
			name:      "CommandSubstitution",
			condition: []*core.Condition{{Condition: "`echo 1`", Expected: "1"}},
		},
		{
			name:      "EnvVar",
			condition: []*core.Condition{{Condition: "${TEST_CONDITION}", Expected: "100"}},
		},
		{
			name: "MultipleCond",
			condition: []*core.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
				{
					Condition: "`echo 100`",
					Expected:  "100",
				},
			},
		},
		{
			name: "MultipleCondOneMet",
			condition: []*core.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
				{
					Condition: "`echo 100`",
					Expected:  "1",
				},
			},
			wantErr: true,
		},
		{
			name: "CommandResultMet",
			condition: []*core.Condition{
				{
					Condition: "true",
				},
			},
		},
		{
			name: "CommandResultNotMet",
			condition: []*core.Condition{
				{
					Condition: "false",
				},
			},
			wantErr: true,
		},
		{
			name: "ComplexCommand",
			condition: []*core.Condition{
				{
					Condition: "test 1 -eq 1",
				},
			},
		},
		{
			name: "EvenMoreComplexCommand",
			condition: []*core.Condition{
				{
					Condition: "df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'",
				},
			},
		},
		{
			name: "CommandResultTest",
			condition: []*core.Condition{
				{
					Condition: "test 1 -eq 1",
				},
			},
		},
		{
			name: "RegexMatch",
			condition: []*core.Condition{
				{
					Condition: "test",
					Expected:  "re:^test$",
				},
			},
		},
		// Negate tests
		{
			name: "NegateMatchingCondition",
			condition: []*core.Condition{
				{
					Condition: "`echo success`",
					Expected:  "success",
					Negate:    true,
				},
			},
			wantErr: true, // condition matches, but negate is true, so it should fail
		},
		{
			name: "NegateNonMatchingCondition",
			condition: []*core.Condition{
				{
					Condition: "`echo failure`",
					Expected:  "success",
					Negate:    true,
				},
			},
			wantErr: false, // condition doesn't match, and negate is true, so it should pass
		},
		{
			name: "NegateCommandSuccess",
			condition: []*core.Condition{
				{
					Condition: "true",
					Negate:    true,
				},
			},
			wantErr: true, // command succeeds, but negate is true, so it should fail
		},
		{
			name: "NegateCommandFailure",
			condition: []*core.Condition{
				{
					Condition: "false",
					Negate:    true,
				},
			},
			wantErr: false, // command fails, and negate is true, so it should pass
		},
		{
			name: "NegateEnvVar",
			condition: []*core.Condition{
				{
					Condition: "${TEST_CONDITION}",
					Expected:  "wrong_value",
					Negate:    true,
				},
			},
			wantErr: false, // env var is 100, not wrong_value, so with negate it should pass
		},
		{
			name: "NegateEnvVarMatching",
			condition: []*core.Condition{
				{
					Condition: "${TEST_CONDITION}",
					Expected:  "100",
					Negate:    true,
				},
			},
			wantErr: true, // env var is 100, matches expected, so with negate it should fail
		},
	}

	// Set environment variable for testing
	_ = os.Setenv("TEST_CONDITION", "100")
	t.Cleanup(func() {
		_ = os.Unsetenv("TEST_CONDITION")
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
			err := runtime.EvalConditions(ctx, []string{"sh"}, tt.condition)
			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
			} else {
				require.NoError(t, err, "expected no error but got %v", err)
			}
			if err != nil {
				require.ErrorIs(t, err, runtime.ErrConditionNotMet)
				require.NotEmpty(t, tt.condition[0].GetErrorMessage())
			}
		})
	}
}

// TestNegateDoesNotSwallowEvaluationErrors verifies that when Negate is true,
// evaluation/runtime errors are NOT swallowed - only ErrConditionNotMet is inverted.
func TestNegateDoesNotSwallowEvaluationErrors(t *testing.T) {
	tests := []struct {
		name                 string
		condition            *core.Condition
		wantErr              bool
		wantConditionNotMet  bool // true if error should be ErrConditionNotMet
		notConditionNotMet   bool // true if error should NOT be ErrConditionNotMet
	}{
		{
			name: "EvalStringErrorNotSwallowed",
			condition: &core.Condition{
				// Command substitution with non-existent binary fails during evaluation
				// This error is NOT ErrConditionNotMet, so it should NOT be inverted
				Condition: "`/nonexistent_binary_xyz_123_456`",
				Expected:  "anything",
				Negate:    true,
			},
			wantErr:            true,
			notConditionNotMet: true, // evaluation errors should not be ErrConditionNotMet
		},
		{
			name: "CommandNotFoundInvertedToSuccess",
			condition: &core.Condition{
				// Command not found returns ErrConditionNotMet (wrapped around exec error)
				// With Negate: true, ErrConditionNotMet is inverted to success
				Condition: "/nonexistent/path/to/command_xyz_123_abc",
				Negate:    true,
			},
			wantErr: false, // ErrConditionNotMet is inverted to success
		},
		{
			name: "FalseCommandInvertedToSuccess",
			condition: &core.Condition{
				// "false" command exits with code 1, which wraps as ErrConditionNotMet
				// With Negate: true, it should be inverted to success
				Condition: "false",
				Negate:    true,
			},
			wantErr: false, // ErrConditionNotMet is inverted to success
		},
		{
			name: "MatchingConditionWithNegateFailsAsConditionNotMet",
			condition: &core.Condition{
				// When condition matches but negate is true, it should fail with ErrConditionNotMet
				Condition: "hello",
				Expected:  "hello",
				Negate:    true,
			},
			wantErr:             true,
			wantConditionNotMet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
			err := runtime.EvalCondition(ctx, []string{"sh"}, tt.condition)

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
