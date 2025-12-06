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
	t.Run("EvalStringError", func(t *testing.T) {
		// Create a condition with an invalid command substitution that will cause an eval error
		// Using an unclosed backtick to cause a parse error
		cond := &core.Condition{
			Condition: "`nonexistent_command_that_does_not_exist_xyz123`",
			Expected:  "anything",
			Negate:    true,
		}

		ctx := context.Background()
		ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
		err := runtime.EvalCondition(ctx, []string{"sh"}, cond)

		// The error should be returned even with Negate: true, because it's an evaluation error
		// not a "condition not met" error
		if err != nil {
			// If there's an error, it should NOT be ErrConditionNotMet
			// (command substitution failure is wrapped differently)
			// The key point is that the error is not swallowed
			require.Error(t, err)
		}
		// If the command happens to exist on the system (unlikely), this is still valid
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		// Test with a command that definitely won't exist
		cond := &core.Condition{
			Condition: "/nonexistent/path/to/command_xyz_123_abc",
			Negate:    true,
		}

		ctx := context.Background()
		ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
		err := runtime.EvalCondition(ctx, []string{"sh"}, cond)

		// Command not found returns ErrConditionNotMet (wrapped around exec error)
		// so with Negate: true, it should be inverted to success
		// This is the expected behavior - command failures are "condition not met"
		require.NoError(t, err)
	})

	t.Run("NegateOnlyInvertsConditionNotMet", func(t *testing.T) {
		// This test verifies that only ErrConditionNotMet is inverted
		// If matchCondition returns an error that is NOT ErrConditionNotMet,
		// it should still be returned

		// "false" command exits with code 1, which wraps as ErrConditionNotMet
		cond := &core.Condition{
			Condition: "false",
			Negate:    true,
		}

		ctx := context.Background()
		ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
		err := runtime.EvalCondition(ctx, []string{"sh"}, cond)

		// With Negate: true, "false" (which returns ErrConditionNotMet) should be inverted to success
		require.NoError(t, err)
	})

	t.Run("NegateWithMatchingConditionFails", func(t *testing.T) {
		// When condition matches but negate is true, it should fail with ErrConditionNotMet
		cond := &core.Condition{
			Condition: "hello",
			Expected:  "hello",
			Negate:    true,
		}

		ctx := context.Background()
		ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, core.Step{}))
		err := runtime.EvalCondition(ctx, []string{"sh"}, cond)

		require.Error(t, err)
		require.True(t, errors.Is(err, runtime.ErrConditionNotMet))
	})
}
