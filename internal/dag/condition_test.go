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

func TestEvalConditions(t *testing.T) {
	for scenario, test := range map[string]struct {
		Conditions []*Condition
		Want       bool
	}{
		"no error conditions": {
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
			true,
		},
		"fail conditions": {
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
			false,
		},
	} {
		t.Run(scenario, func(t *testing.T) {
			err := EvalConditions(test.Conditions)
			if test.Want {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestConditionError(t *testing.T) {
	c := &Condition{
		Condition: "`invalid`",
		Expected:  "1",
	}
	_, err := c.Eval()
	require.Error(t, err)

	err = EvalCondition(c)
	require.Error(t, err)
}
