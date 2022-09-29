package dag

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondition(t *testing.T) {
	{
		c := &Condition{
			Condition: "`echo 1`",
			Expected:  "1",
		}
		ret, err := c.Eval()
		require.NoError(t, err)
		require.Equal(t, ret.Condition, c.Condition)
		require.Equal(t, ret.Expected, c.Expected)
		require.Equal(t, ret.Actual, c.Expected)
	}
	{
		os.Setenv("TEST_CONDITION", "100")
		c := &Condition{
			Condition: "${TEST_CONDITION}",
			Expected:  "100",
		}
		ret, err := c.Eval()
		require.NoError(t, err)
		require.Equal(t, ret.Condition, c.Condition)
		require.Equal(t, ret.Expected, c.Expected)
		require.Equal(t, ret.Actual, c.Expected)
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
