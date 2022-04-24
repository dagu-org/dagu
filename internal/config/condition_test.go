package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/jobctl/internal/config"
)

func TestCondition(t *testing.T) {
	{
		c := &config.Condition{
			Condition: "`echo 1`",
			Expected:  "1",
		}
		ret, err := c.Eval()
		require.NoError(t, err)
		assert.Equal(t, ret.Condition, c.Condition)
		assert.Equal(t, ret.Expected, c.Expected)
		assert.Equal(t, ret.Actual, c.Expected)
	}
	{
		os.Setenv("TEST_CONDITION", "100")
		c := &config.Condition{
			Condition: "${TEST_CONDITION}",
			Expected:  "100",
		}
		ret, err := c.Eval()
		require.NoError(t, err)
		assert.Equal(t, ret.Condition, c.Condition)
		assert.Equal(t, ret.Expected, c.Expected)
		assert.Equal(t, ret.Actual, c.Expected)
	}
}

func TestEvalConditions(t *testing.T) {
	for scenario, test := range map[string]struct {
		Conditions []*config.Condition
		Want       bool
	}{
		"no error conditions": {
			[]*config.Condition{
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
			[]*config.Condition{
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
			err := config.EvalConditions(test.Conditions)
			if test.Want {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
