package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/service/frontend/gen/models"
	"github.com/go-openapi/swag"
)

func NewStepObject(step dag.Step) *models.StepObject {
	so := &models.StepObject{
		Args:          step.Args,
		CmdWithArgs:   swag.String(step.CmdWithArgs),
		Command:       swag.String(step.Command),
		Depends:       step.Depends,
		Description:   swag.String(step.Description),
		Dir:           swag.String(step.Dir),
		MailOnError:   swag.Bool(step.MailOnError),
		Name:          swag.String(step.Name),
		Output:        swag.String(step.Output),
		Preconditions: NewConditions(step.Preconditions),
		RepeatPolicy:  NewRepeatPolicy(step.RepeatPolicy),
		Script:        swag.String(step.Script),
		Variables:     step.Variables,
	}
	if step.SubWorkflow != nil {
		so.Run = step.SubWorkflow.Name
		so.Params = step.SubWorkflow.Params
	}
	return so
}

func NewRepeatPolicy(repeatPolicy dag.RepeatPolicy) *models.RepeatPolicy {
	return &models.RepeatPolicy{
		Repeat:   repeatPolicy.Repeat,
		Interval: int64(repeatPolicy.Interval),
	}
}

func NewConditions(conditions []dag.Condition) []*models.Condition {
	var result []*models.Condition
	for _, cond := range conditions {
		result = append(result, NewCondition(cond))
	}
	return result
}

func NewCondition(cond dag.Condition) *models.Condition {
	return &models.Condition{
		Condition: cond.Condition,
		Expected:  cond.Expected,
	}
}
