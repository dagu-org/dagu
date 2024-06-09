package dag

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondition_Eval(t *testing.T) {
	tests := []struct {
		name      string
		condition []*Condition
		wantErr   bool
	}{
		{
			name:      "Condition with a command substitution",
			condition: []*Condition{{Condition: "`echo 1`", Expected: "1"}},
		},
		{
			name:      "Condition with environment variable",
			condition: []*Condition{{Condition: "${TEST_CONDITION}", Expected: "100"}},
		},
		{
			name: "Multiple conditions and all are met",
			condition: []*Condition{
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
			name: "Multiple conditions and one is not met",
			condition: []*Condition{
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
			name: "Invalid condition",
			condition: []*Condition{
				{
					Condition: "`invalid`",
				},
			},
			wantErr: true,
		},
	}

	// Set environment variable for testing
	_ = os.Setenv("TEST_CONDITION", "100")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvalConditions(tt.condition)
			require.Equal(t, tt.wantErr, err != nil)
		})
	}
}
