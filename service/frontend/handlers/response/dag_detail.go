package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	domain "github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
)

func ToGetDagDetailResponse(
	workflowStatus *engine.DAGStatus,
	tab string,
) *models.GetDagDetailsResponse {
	return &models.GetDagDetailsResponse{
		Title:      lo.ToPtr(workflowStatus.DAG.Name),
		DAG:        ToDagStatusWithDetails(workflowStatus),
		Tab:        lo.ToPtr(tab),
		Definition: lo.ToPtr(""),
		LogData:    nil,
		Errors:     []string{},
	}
}

func ToDagStatusWithDetails(dagStatus *engine.DAGStatus) *models.DagStatusWithDetails {
	return &models.DagStatusWithDetails{
		DAG:       ToDagDetail(dagStatus.DAG),
		Dir:       lo.ToPtr(dagStatus.Dir),
		Error:     lo.ToPtr(toErrorText(dagStatus.Error)),
		ErrorT:    dagStatus.ErrorT,
		File:      lo.ToPtr(dagStatus.File),
		Status:    ToDagStatusDetail(dagStatus.Status),
		Suspended: lo.ToPtr(dagStatus.Suspended),
	}
}

func ToDagDetail(d *dag.DAG) *models.DagDetail {
	return &models.DagDetail{
		DefaultParams:     lo.ToPtr(d.DefaultParams),
		Delay:             lo.ToPtr(int64(d.Delay)),
		Description:       lo.ToPtr(d.Description),
		Env:               d.Env,
		Group:             lo.ToPtr(d.Group),
		HandlerOn:         ToHandlerOn(d.HandlerOn),
		HistRetentionDays: lo.ToPtr(int64(d.HistRetentionDays)),
		Location:          lo.ToPtr(d.Location),
		LogDir:            lo.ToPtr(d.LogDir),
		MaxActiveRuns:     lo.ToPtr(int64(d.MaxActiveRuns)),
		Name:              lo.ToPtr(d.Name),
		Params:            d.Params,
		Preconditions: lo.Map(d.Preconditions, func(item *dag.Condition, _ int) *models.Condition {
			return ToCondition(item)
		}),
		Schedule: lo.Map(d.Schedule, func(item *dag.Schedule, _ int) *models.Schedule {
			return ToSchedule(item)
		}),
		Steps: lo.Map(d.Steps, func(item *dag.Step, _ int) *models.StepObject {
			return ToStepObject(item)
		}),
		Tags: d.Tags,
	}
}

func ToHandlerOn(handlerOn dag.HandlerOn) *models.HandlerOn {
	ret := &models.HandlerOn{}
	if handlerOn.Failure != nil {
		ret.Failure = ToStepObject(handlerOn.Failure)
	}
	if handlerOn.Success != nil {
		ret.Success = ToStepObject(handlerOn.Success)
	}
	if handlerOn.Cancel != nil {
		ret.Cancel = ToStepObject(handlerOn.Cancel)
	}
	if handlerOn.Exit != nil {
		ret.Exit = ToStepObject(handlerOn.Exit)
	}
	return ret
}

func ToDagStatusDetail(s *domain.Status) *models.DagStatusDetail {
	return &models.DagStatusDetail{
		Log:        lo.ToPtr(s.Log),
		Name:       lo.ToPtr(s.Name),
		Params:     lo.ToPtr(s.Params),
		Pid:        lo.ToPtr(int64(s.Pid)),
		RequestID:  lo.ToPtr(s.RequestId),
		StartedAt:  lo.ToPtr(s.StartedAt),
		FinishedAt: lo.ToPtr(s.FinishedAt),
		Status:     lo.ToPtr(int64(s.Status)),
		StatusText: lo.ToPtr(s.StatusText),
		Nodes: lo.Map(s.Nodes, func(item *domain.Node, _ int) *models.StatusNode {
			return ToNode(item)
		}),
	}
}
