package response

import (
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
)

func ToSearchWorkflowsResponse(ret []*controller.GrepResult, errs []string) *models.SearchDagsResponse {
	return &models.SearchDagsResponse{
		Results: lo.Map(ret, func(item *controller.GrepResult, _ int) *models.SearchDagsResultItem {
			return ToSearchWorkflowsResultItem(item)
		}),
		Errors: errs,
	}
}

func ToSearchWorkflowsResultItem(workflow *controller.GrepResult) *models.SearchDagsResultItem {
	return &models.SearchDagsResultItem{
		Name: workflow.Name,
		DAG:  ToWorkflow(workflow.DAG),
		Matches: lo.Map(workflow.Matches, func(item *grep.Match, _ int) *models.SearchDagsMatchItem {
			return ToSearchWorkflowsMatchItem(item)
		}),
	}
}

func ToSearchWorkflowsMatchItem(match *grep.Match) *models.SearchDagsMatchItem {
	return &models.SearchDagsMatchItem{
		Line:       match.Line,
		LineNumber: int64(match.LineNumber),
		StartLine:  int64(match.StartLine),
	}
}
