package config

import (
	"fmt"

	"github.com/yohamta/jobctl/internal/utils"
)

type Condition struct {
	Condition string
	Expected  string
}

type ConditionResult struct {
	Condition string
	Expected  string
	Actual    string
}

func (c *Condition) Eval() (*ConditionResult, error) {
	ret, err := utils.ParseVariable(c.Condition)
	if err != nil {
		return nil, err
	}
	return &ConditionResult{
		Condition: c.Condition,
		Expected:  c.Expected,
		Actual:    ret,
	}, nil
}

func EvalCondition(c *Condition) error {
	r, err := c.Eval()
	if err != nil {
		return fmt.Errorf(
			"failed to evaluate condition. Condition=%s Error=%v",
			c.Condition, err)
	}
	if r.Expected != r.Actual {
		return fmt.Errorf(
			"condition was not met. Condition=%s Expected=%s Actual=%s",
			r.Condition, r.Expected, r.Actual)
	}
	return err
}

func EvalConditions(cond []*Condition) error {
	for _, c := range cond {
		err := EvalCondition(c)
		if err != nil {
			return err
		}
	}
	return nil
}
