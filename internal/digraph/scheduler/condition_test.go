package scheduler_test

import (
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestCondition_Eval(t *testing.T) {
	tests := []struct {
		name      string
		condition []*digraph.Condition
		wantErr   bool
	}{
		{
			name:      "CommandSubstitution",
			condition: []*digraph.Condition{{Condition: "`echo 1`", Expected: "1"}},
		},
		{
			name:      "EnvVar",
			condition: []*digraph.Condition{{Condition: "${TEST_CONDITION}", Expected: "100"}},
		},
		{
			name: "MultipleCond",
			condition: []*digraph.Condition{
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
			condition: []*digraph.Condition{
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
			condition: []*digraph.Condition{
				{
					Condition: "true",
				},
			},
		},
		{
			name: "CommandResultNotMet",
			condition: []*digraph.Condition{
				{
					Condition: "false",
				},
			},
			wantErr: true,
		},
		{
			name: "ComplexCommand",
			condition: []*digraph.Condition{
				{
					Condition: "test 1 -eq 1",
				},
			},
		},
		{
			name: "EvenMoreComplexCommand",
			condition: []*digraph.Condition{
				{
					Condition: "df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'",
				},
			},
		},
		{
			name: "CommandResultTest",
			condition: []*digraph.Condition{
				{
					Condition: "test 1 -eq 1",
				},
			},
		},
		{
			name: "RegexMatch",
			condition: []*digraph.Condition{
				{
					Condition: "test",
					Expected:  "re:^test$",
				},
			},
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
			ctx = digraph.WithEnv(ctx, digraph.NewEnv(ctx, digraph.Step{}))
			err := scheduler.EvalConditions(ctx, "sh", tt.condition)
			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
			} else {
				require.NoError(t, err, "expected no error but got %v", err)
			}
			if err != nil {
				require.ErrorIs(t, err, scheduler.ErrConditionNotMet)
				require.NotEmpty(t, tt.condition[0].GetErrorMessage())
			}
		})
	}
}
