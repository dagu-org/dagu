// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dags

import (
	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/frontend/gen/models"
	"github.com/daguflow/dagu/internal/persistence/model"
	"github.com/go-openapi/swag"
)

func convertToDAG(workflow *dag.DAG) *models.Dag {
	var schedules []*models.Schedule
	for _, s := range workflow.Schedule {
		schedules = append(schedules, &models.Schedule{
			Expression: swag.String(s.Expression),
		})
	}

	return &models.Dag{
		Name:          swag.String(workflow.Name),
		Group:         swag.String(workflow.Group),
		Description:   swag.String(workflow.Description),
		Params:        workflow.Params,
		DefaultParams: swag.String(workflow.DefaultParams),
		Tags:          workflow.Tags,
		Schedule:      schedules,
	}
}

func convertToStatusDetail(s *model.Status) *models.DagStatusDetail {
	status := &models.DagStatusDetail{
		Log:        swag.String(s.Log),
		Name:       swag.String(s.Name),
		Params:     swag.String(s.Params),
		Pid:        swag.Int64(int64(s.PID)),
		RequestID:  swag.String(s.RequestID),
		StartedAt:  swag.String(s.StartedAt),
		FinishedAt: swag.String(s.FinishedAt),
		Status:     swag.Int64(int64(s.Status)),
		StatusText: swag.String(s.StatusText),
	}
	for _, n := range s.Nodes {
		status.Nodes = append(status.Nodes, convertToNode(n))
	}
	if s.OnSuccess != nil {
		status.OnSuccess = convertToNode(s.OnSuccess)
	}
	if s.OnFailure != nil {
		status.OnFailure = convertToNode(s.OnFailure)
	}
	if s.OnCancel != nil {
		status.OnCancel = convertToNode(s.OnCancel)
	}
	if s.OnExit != nil {
		status.OnExit = convertToNode(s.OnExit)
	}
	return status
}

func convertToNode(node *model.Node) *models.StatusNode {
	return &models.StatusNode{
		DoneCount:  swag.Int64(int64(node.DoneCount)),
		Error:      swag.String(node.Error),
		FinishedAt: swag.String(node.FinishedAt),
		Log:        swag.String(node.Log),
		RetryCount: swag.Int64(int64(node.RetryCount)),
		StartedAt:  swag.String(node.StartedAt),
		Status:     swag.Int64(int64(node.Status)),
		StatusText: swag.String(node.StatusText),
		Step:       convertToStepObject(node.Step),
	}
}

func convertToStepObject(step dag.Step) *models.StepObject {
	var conditions []*models.Condition
	for _, cond := range step.Preconditions {
		conditions = append(conditions, &models.Condition{
			Condition: cond.Condition,
			Expected:  cond.Expected,
		})
	}

	repeatPolicy := &models.RepeatPolicy{
		Repeat:   step.RepeatPolicy.Repeat,
		Interval: int64(step.RepeatPolicy.Interval),
	}

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
		Preconditions: conditions,
		RepeatPolicy:  repeatPolicy,
		Script:        swag.String(step.Script),
		Variables:     step.Variables,
	}
	if step.SubWorkflow != nil {
		so.Run = step.SubWorkflow.Name
		so.Params = step.SubWorkflow.Params
	}
	return so
}
