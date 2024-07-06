package dag

import (
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
			name: "InvalidCond",
			condition: []Condition{
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
