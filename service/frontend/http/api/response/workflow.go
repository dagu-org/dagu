package response

import (
	"github.com/samber/lo"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	domain "github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/service/frontend/models"
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

func ToNode(node *domain.Node) *models.StatusNode {
	return &models.StatusNode{
		DoneCount:  lo.ToPtr(int64(node.DoneCount)),
		Error:      lo.ToPtr(node.Error),
		FinishedAt: lo.ToPtr(node.FinishedAt),
		Log:        lo.ToPtr(node.Log),
		RetryCount: lo.ToPtr(int64(node.RetryCount)),
		StartedAt:  lo.ToPtr(node.StartedAt),
		Status:     lo.ToPtr(int64(node.Status)),
		StatusText: lo.ToPtr(node.StatusText),
		Step:       ToStepObject(node.Step),
	}
}

func ToStepObject(step *dag.Step) *models.StepObject {
	return &models.StepObject{
		Args:        step.Args,
		CmdWithArgs: lo.ToPtr(step.CmdWithArgs),
		Command:     lo.ToPtr(step.Command),
		Depends:     step.Depends,
		Description: lo.ToPtr(step.Description),
		Dir:         lo.ToPtr(step.Dir),
		MailOnError: lo.ToPtr(step.MailOnError),
		Name:        lo.ToPtr(step.Name),
		Output:      lo.ToPtr(step.Output),
		Preconditions: lo.Map(step.Preconditions, func(item *dag.Condition, _ int) *models.Condition {
			return ToCondition(item)
		}),
		RepeatPolicy: ToRepeatPolicy(step.RepeatPolicy),
		Script:       lo.ToPtr(step.Script),
		Variables:    step.Variables,
	}
}

func ToCondition(cond *dag.Condition) *models.Condition {
	return &models.Condition{
		Condition: cond.Condition,
		Expected:  cond.Expected,
	}
}

func ToRepeatPolicy(repeatPolicy dag.RepeatPolicy) *models.RepeatPolicy {
	return &models.RepeatPolicy{
		Repeat:   repeatPolicy.Repeat,
		Interval: int64(repeatPolicy.Interval),
	}
}
