package response

import (
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/internal/dag"
	domain "github.com/dagu-dev/dagu/internal/models"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
)

func ToGetWorkflowDetailResponse(
	workflowStatus *controller.DAGStatus,
	tab string,
) *models.GetWorkflowDetailResponse {
	return &models.GetWorkflowDetailResponse{
		Title:      lo.ToPtr(workflowStatus.DAG.Name),
		DAG:        ToWorkflowStatusWithDetail(workflowStatus),
		Tab:        lo.ToPtr(tab),
		Definition: lo.ToPtr(""),
		LogData:    nil,
		Errors:     []string{},
	}
}

func ToWorkflowStatusWithDetail(workflowStatus *controller.DAGStatus) *models.WorkflowStatusWithDetail {
	return &models.WorkflowStatusWithDetail{
		DAG:       ToWorkflowDetail(workflowStatus.DAG),
		Dir:       lo.ToPtr(workflowStatus.Dir),
		Error:     lo.ToPtr(toErrorText(workflowStatus.Error)),
		ErrorT:    workflowStatus.ErrorT,
		File:      lo.ToPtr(workflowStatus.File),
		Status:    ToWorkflowStatusDetail(workflowStatus.Status),
		Suspended: lo.ToPtr(workflowStatus.Suspended),
	}
}

func ToWorkflowDetail(workflow *dag.DAG) *models.WorkflowDetail {
	return &models.WorkflowDetail{
		DefaultParams:     lo.ToPtr(workflow.DefaultParams),
		Delay:             lo.ToPtr(int64(workflow.Delay)),
		Description:       lo.ToPtr(workflow.Description),
		Env:               workflow.Env,
		Group:             lo.ToPtr(workflow.Group),
		HandlerOn:         ToHandlerOn(workflow.HandlerOn),
		HistRetentionDays: lo.ToPtr(int64(workflow.HistRetentionDays)),
		Location:          lo.ToPtr(workflow.Location),
		LogDir:            lo.ToPtr(workflow.LogDir),
		MaxActiveRuns:     lo.ToPtr(int64(workflow.MaxActiveRuns)),
		Name:              lo.ToPtr(workflow.Name),
		Params:            workflow.Params,
		Preconditions: lo.Map(workflow.Preconditions, func(item *dag.Condition, _ int) *models.Condition {
			return ToCondition(item)
		}),
		Schedule: lo.Map(workflow.Schedule, func(item *dag.Schedule, _ int) *models.Schedule {
			return ToSchedule(item)
		}),
		Steps: lo.Map(workflow.Steps, func(item *dag.Step, _ int) *models.StepObject {
			return ToStepObject(item)
		}),
		Tags: workflow.Tags,
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

func ToWorkflowStatusDetail(s *domain.Status) *models.WorkflowStatusDetail {
	return &models.WorkflowStatusDetail{
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
