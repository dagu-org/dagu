package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/service/frontend/gen/models"
	"github.com/go-openapi/swag"
	"github.com/samber/lo"
)

func NewGetDagDetailResponse(
	workflowStatus *persistence.DAGStatus,
	tab string,
) *models.GetDagDetailsResponse {
	return &models.GetDagDetailsResponse{
		Title:      swag.String(workflowStatus.DAG.Name),
		DAG:        NewDagStatusWithDetails(workflowStatus),
		Tab:        swag.String(tab),
		Definition: swag.String(""),
		LogData:    nil,
		Errors:     []string{},
	}
}

func NewDagStatusWithDetails(
	dagStatus *persistence.DAGStatus,
) *models.DagStatusWithDetails {
	return &models.DagStatusWithDetails{
		DAG:       NewDagDetail(dagStatus.DAG),
		Dir:       swag.String(dagStatus.Dir),
		Error:     swag.String(NewErrorText(dagStatus.Error)),
		ErrorT:    dagStatus.ErrorT,
		File:      swag.String(dagStatus.File),
		Status:    NewDagStatusDetail(dagStatus.Status),
		Suspended: swag.Bool(dagStatus.Suspended),
	}
}

func NewDagDetail(dg *dag.DAG) *models.DagDetail {
	return &models.DagDetail{
		DefaultParams:     swag.String(dg.DefaultParams),
		Delay:             swag.Int64(int64(dg.Delay)),
		Description:       swag.String(dg.Description),
		Env:               dg.Env,
		Group:             swag.String(dg.Group),
		HandlerOn:         NewHandlerOn(dg.HandlerOn),
		HistRetentionDays: swag.Int64(int64(dg.HistRetentionDays)),
		Location:          swag.String(dg.Location),
		LogDir:            swag.String(dg.LogDir),
		MaxActiveRuns:     swag.Int64(int64(dg.MaxActiveRuns)),
		Name:              swag.String(dg.Name),
		Params:            dg.Params,
		Preconditions: lo.Map(
			dg.Preconditions, func(
				item dag.Condition, _ int,
			) *models.Condition {
				return NewCondition(item)
			}),
		Schedule: lo.Map(
			dg.Schedule, func(item dag.Schedule, _ int) *models.Schedule {
				return NewSchedule(item)
			}),
		Steps: lo.Map(dg.Steps, func(item dag.Step, _ int) *models.StepObject {
			return NewStepObject(item)
		}),
		Tags: dg.Tags,
	}
}

func NewHandlerOn(handlerOn dag.HandlerOn) *models.HandlerOn {
	ret := &models.HandlerOn{}
	if handlerOn.Failure != nil {
		ret.Failure = NewStepObject(*handlerOn.Failure)
	}
	if handlerOn.Success != nil {
		ret.Success = NewStepObject(*handlerOn.Success)
	}
	if handlerOn.Cancel != nil {
		ret.Cancel = NewStepObject(*handlerOn.Cancel)
	}
	if handlerOn.Exit != nil {
		ret.Exit = NewStepObject(*handlerOn.Exit)
	}
	return ret
}

func NewDagStatusDetail(s *model.Status) *models.DagStatusDetail {
	return &models.DagStatusDetail{
		Log:        swag.String(s.Log),
		Name:       swag.String(s.Name),
		Params:     swag.String(s.Params),
		Pid:        swag.Int64(int64(s.Pid)),
		RequestID:  swag.String(s.RequestID),
		StartedAt:  swag.String(s.StartedAt),
		FinishedAt: swag.String(s.FinishedAt),
		Status:     swag.Int64(int64(s.Status)),
		StatusText: swag.String(s.StatusText),
		Nodes: lo.Map(
			s.Nodes, func(item *model.Node, _ int) *models.StatusNode {
				return NewNode(item)
			}),
	}
}
