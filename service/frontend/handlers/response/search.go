package response

import (
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/service/frontend/gen/models"
	"github.com/samber/lo"
)

func NewSearchDAGsResponse(ret []*persistence.GrepResult, errs []string) *models.SearchDagsResponse {
	return &models.SearchDagsResponse{
		Results: lo.Map(ret, func(item *persistence.GrepResult, _ int) *models.SearchDagsResultItem {
			return NewSearchDAGsResultItem(item)
		}),
		Errors: errs,
	}
}

func NewSearchDAGsResultItem(result *persistence.GrepResult) *models.SearchDagsResultItem {
	return &models.SearchDagsResultItem{
		Name: result.Name,
		DAG:  NewDAG(result.DAG),
		Matches: lo.Map(result.Matches, func(item *grep.Match, _ int) *models.SearchDagsMatchItem {
			return NewSearchDAGsMatchItem(item)
		}),
	}
}

func NewSearchDAGsMatchItem(match *grep.Match) *models.SearchDagsMatchItem {
	return &models.SearchDagsMatchItem{
		Line:       match.Line,
		LineNumber: int64(match.LineNumber),
		StartLine:  int64(match.StartLine),
	}
}
