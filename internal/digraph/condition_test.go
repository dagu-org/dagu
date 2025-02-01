package digraph

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondition_Eval(t *testing.T) {
	tests := []struct {
		name      string
		condition []Condition
		wantErr   bool
	}{
		{
			name:      "CommandSubstitution",
			condition: []Condition{{Condition: "`echo 1`", Expected: "1"}},
		},
		{
			name:      "EnvVar",
			condition: []Condition{{Condition: "${TEST_CONDITION}", Expected: "100"}},
		},
		{
			name: "MultipleCond",
			condition: []Condition{
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
			condition: []Condition{
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
			condition: []Condition{
				{
					Command: "true",
				},
			},
		},
		{
			name: "CommandResultNotMet",
			condition: []Condition{
				{
					Command: "false",
				},
			},
			wantErr: true,
		},
		{
			name: "ComplexCommand",
			condition: []Condition{
				{
					Command: "test 1 -eq 1",
				},
			},
		},
		{
			name: "EvenMoreComplexCommand",
			condition: []Condition{
				{
					Command: "df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'",
				},
			},
		},
		{
			name: "CommandResultTest",
			condition: []Condition{
				{
					Command: "test 1 -eq 1",
				},
			},
		},
		{
			name: "RegexMatch",
			condition: []Condition{
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
			err := EvalConditions(context.Background(), tt.condition)
			require.Equal(t, tt.wantErr, err != nil)
			if err != nil {
				require.ErrorIs(t, err, ErrConditionNotMet)
			}
		})
	}
}
