package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/go-openapi/swag"
	"github.com/samber/lo"
)

func ToGetDagDetailResponse(
	workflowStatus *persistence.DAGStatus,
	tab string,
) *models.GetDagDetailsResponse {
	return &models.GetDagDetailsResponse{
		Title:      swag.String(workflowStatus.DAG.Name),
		DAG:        ToDagStatusWithDetails(workflowStatus),
		Tab:        swag.String(tab),
		Definition: swag.String(""),
		LogData:    nil,
		Errors:     []string{},
	}
}

func ToDagStatusWithDetails(dagStatus *persistence.DAGStatus) *models.DagStatusWithDetails {
	return &models.DagStatusWithDetails{
		DAG:       ToDagDetail(dagStatus.DAG),
		Dir:       swag.String(dagStatus.Dir),
		Error:     swag.String(toErrorText(dagStatus.Error)),
		ErrorT:    dagStatus.ErrorT,
		File:      swag.String(dagStatus.File),
		Status:    ToDagStatusDetail(dagStatus.Status),
		Suspended: swag.Bool(dagStatus.Suspended),
	}
}

func ToDagDetail(dg *dag.DAG) *models.DagDetail {
	return &models.DagDetail{
		DefaultParams:     swag.String(dg.DefaultParams),
		Delay:             swag.Int64(int64(dg.Delay)),
		Description:       swag.String(dg.Description),
		Env:               dg.Env,
		Group:             swag.String(dg.Group),
		HandlerOn:         ToHandlerOn(dg.HandlerOn),
		HistRetentionDays: swag.Int64(int64(dg.HistRetentionDays)),
		Location:          swag.String(dg.Location),
		LogDir:            swag.String(dg.LogDir),
		MaxActiveRuns:     swag.Int64(int64(dg.MaxActiveRuns)),
		Name:              swag.String(dg.Name),
		Params:            dg.Params,
		Preconditions: lo.Map(dg.Preconditions, func(item dag.Condition, _ int) *models.Condition {
			return ToCondition(item)
		}),
		Schedule: lo.Map(dg.Schedule, func(item dag.Schedule, _ int) *models.Schedule {
			return ToSchedule(item)
		}),
		Steps: lo.Map(dg.Steps, func(item dag.Step, _ int) *models.StepObject {
			return ToStepObject(item)
		}),
		Tags: dg.Tags,
	}
}

func ToHandlerOn(handlerOn dag.HandlerOn) *models.HandlerOn {
	ret := &models.HandlerOn{}
	if handlerOn.Failure != nil {
		ret.Failure = ToStepObject(*handlerOn.Failure)
	}
	if handlerOn.Success != nil {
		ret.Success = ToStepObject(*handlerOn.Success)
	}
	if handlerOn.Cancel != nil {
		ret.Cancel = ToStepObject(*handlerOn.Cancel)
	}
	if handlerOn.Exit != nil {
		ret.Exit = ToStepObject(*handlerOn.Exit)
	}
	return ret
}

func ToDagStatusDetail(s *model.Status) *models.DagStatusDetail {
	return &models.DagStatusDetail{
		Log:        swag.String(s.Log),
		Name:       swag.String(s.Name),
		Params:     swag.String(s.Params),
		Pid:        swag.Int64(int64(s.Pid)),
		RequestID:  swag.String(s.RequestId),
		StartedAt:  swag.String(s.StartedAt),
		FinishedAt: swag.String(s.FinishedAt),
		Status:     swag.Int64(int64(s.Status)),
		StatusText: swag.String(s.StatusText),
		Nodes: lo.Map(s.Nodes, func(item *model.Node, _ int) *models.StatusNode {
			return ToNode(item)
		}),
	}
}
