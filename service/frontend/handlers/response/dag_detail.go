package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
)

func ToGetDagDetailResponse(
	workflowStatus *persistence.DAGStatus,
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

func ToDagStatusWithDetails(dagStatus *persistence.DAGStatus) *models.DagStatusWithDetails {
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

func ToDagDetail(dg *dag.DAG) *models.DagDetail {
	return &models.DagDetail{
		DefaultParams:     lo.ToPtr(dg.DefaultParams),
		Delay:             lo.ToPtr(int64(dg.Delay)),
		Description:       lo.ToPtr(dg.Description),
		Env:               dg.Env,
		Group:             lo.ToPtr(dg.Group),
		HandlerOn:         ToHandlerOn(dg.HandlerOn),
		HistRetentionDays: lo.ToPtr(int64(dg.HistRetentionDays)),
		Location:          lo.ToPtr(dg.Location),
		LogDir:            lo.ToPtr(dg.LogDir),
		MaxActiveRuns:     lo.ToPtr(int64(dg.MaxActiveRuns)),
		Name:              lo.ToPtr(dg.Name),
		Params:            dg.Params,
		Preconditions: lo.Map(dg.Preconditions, func(item *dag.Condition, _ int) *models.Condition {
			return ToCondition(item)
		}),
		Schedule: lo.Map(dg.Schedule, func(item *dag.Schedule, _ int) *models.Schedule {
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
		Log:        lo.ToPtr(s.Log),
		Name:       lo.ToPtr(s.Name),
		Params:     lo.ToPtr(s.Params),
		Pid:        lo.ToPtr(int64(s.Pid)),
		RequestID:  lo.ToPtr(s.RequestId),
		StartedAt:  lo.ToPtr(s.StartedAt),
		FinishedAt: lo.ToPtr(s.FinishedAt),
		Status:     lo.ToPtr(int64(s.Status)),
		StatusText: lo.ToPtr(s.StatusText),
		Nodes: lo.Map(s.Nodes, func(item *model.Node, _ int) *models.StatusNode {
			return ToNode(item)
		}),
	}
}
