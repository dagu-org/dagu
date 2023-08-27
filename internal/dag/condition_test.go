package dag

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondition(t *testing.T) {
	{
		c := &Condition{Condition: "`echo 1`", Expected: "1"}
		err := EvalCondition(c)
		require.NoError(t, err)
	}
	{
		_ = os.Setenv("TEST_CONDITION", "100")
		c := &Condition{Condition: "${TEST_CONDITION}", Expected: "100"}
		err := EvalCondition(c)
		require.NoError(t, err)
	}
}

func TestConditionsWithEval(t *testing.T) {
	tests := []struct {
		conditions []*Condition
		isErr      bool
	}{
		{
			[]*Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
				{
					Condition: "`echo 100`",
					Expected:  "100",
				},
			},
			false,
		},
		{
			[]*Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
				{
					Condition: "`echo 100`",
					Expected:  "0",
				},
			},
			true,
		},
		{
			[]*Condition{
				{
					Condition: "`invalid`",
					Expected:  "1",
				},
			},
			true,
		},
	}

	for _, tt := range tests {
		err := EvalConditions(tt.conditions)
		require.Equal(t, tt.isErr, err != nil)
	}
}
