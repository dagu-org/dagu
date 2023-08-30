package response

import (
	"github.com/samber/lo"
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/service/frontend/models"
)

func ToSearchWorkflowsResponse(ret []*controller.GrepResult, errs []string) *models.SearchWorkflowsResponse {
	return &models.SearchWorkflowsResponse{
		Results: lo.Map(ret, func(item *controller.GrepResult, _ int) *models.SearchWorkflowsResultItem {
			return ToSearchWorkflowsResultItem(item)
		}),
		Errors: errs,
	}
}

func ToSearchWorkflowsResultItem(workflow *controller.GrepResult) *models.SearchWorkflowsResultItem {
	return &models.SearchWorkflowsResultItem{
		Name: workflow.Name,
		DAG:  ToWorkflow(workflow.DAG),
		Matches: lo.Map(workflow.Matches, func(item *grep.Match, _ int) *models.SearchWorkflowsMatchItem {
			return ToSearchWorkflowsMatchItem(item)
		}),
	}
}

func ToSearchWorkflowsMatchItem(match *grep.Match) *models.SearchWorkflowsMatchItem {
	return &models.SearchWorkflowsMatchItem{
		Line:       match.Line,
		LineNumber: int64(match.LineNumber),
		StartLine:  int64(match.StartLine),
	}
}
