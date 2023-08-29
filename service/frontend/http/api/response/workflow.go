package response

import (
	"github.com/samber/lo"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	pkgmodels "github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/service/frontend/models"
)

func ToListWorkflowResponse(
	workflows []*controller.DAGStatus,
	errs []string,
	hasError bool,
) *models.ListWorkflowsResponse {
	return &models.ListWorkflowsResponse{
		Title: "DAGList",
		DAGs: lo.Map(workflows, func(item *controller.DAGStatus, _ int) *models.WorkflowListItem {
			return ToWorkflowListItem(item)
		}),
		Errors:   errs,
		HasError: hasError,
	}
}

func ToWorkflowListItem(s *controller.DAGStatus) *models.WorkflowListItem {
	return &models.WorkflowListItem{
		Dir:       s.Dir,
		Error:     toErrorText(s.Error),
		ErrorT:    lo.FromPtr(s.ErrorT),
		File:      s.File,
		Status:    ToWorkflowStatus(s.Status),
		Suspended: s.Suspended,
		DAG:       ToDAG(s.DAG),
	}
}

func ToDAG(d *dag.DAG) *models.WorkflowDef {
	return &models.WorkflowDef{
		Name:          d.Name,
		Group:         d.Group,
		Params:        d.Params,
		DefaultParams: d.DefaultParams,
		Schedule: lo.Map(d.Schedule, func(item *dag.Schedule, _ int) *models.Schedule {
			return ToSchedule(item)
		}),
	}
}

func ToSchedule(s *dag.Schedule) *models.Schedule {
	return &models.Schedule{
		Expression: s.Expression,
	}
}

func ToWorkflowStatus(s *pkgmodels.Status) *models.WorkflowStatus {
	return &models.WorkflowStatus{
		FinishedAt: s.FinishedAt,
		Log:        s.Log,
		Name:       s.Name,
		Params:     s.Params,
		Pid:        int64(s.Pid),
		RequestID:  s.RequestId,
		StartedAt:  s.StartedAt,
		Status:     int64(s.Status),
		StatusText: s.StatusText,
	}
}
