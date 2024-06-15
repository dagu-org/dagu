package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/service/frontend/gen/models"
	"github.com/go-openapi/swag"
	"github.com/samber/lo"
)

func ToListDagResponse(
	dagStatusList []*persistence.DAGStatus,
	errs []string,
	hasError bool,
) *models.ListDagsResponse {
	return &models.ListDagsResponse{
		DAGs: lo.Map(dagStatusList, func(item *persistence.DAGStatus, _ int) *models.DagListItem {
			return ToDagListItem(item)
		}),
		Errors:   errs,
		HasError: swag.Bool(hasError),
	}
}

func ToDagListItem(status *persistence.DAGStatus) *models.DagListItem {
	return &models.DagListItem{
		Dir:       swag.String(status.Dir),
		Error:     swag.String(toErrorText(status.Error)),
		ErrorT:    status.ErrorT,
		File:      swag.String(status.File),
		Status:    ToDagStatus(status.Status),
		Suspended: swag.Bool(status.Suspended),
		DAG:       ToDAG(status.DAG),
	}
}

func ToDAG(dg *dag.DAG) *models.Dag {
	return &models.Dag{
		Name:          swag.String(dg.Name),
		Group:         swag.String(dg.Group),
		Description:   swag.String(dg.Description),
		Params:        dg.Params,
		DefaultParams: swag.String(dg.DefaultParams),
		Tags:          dg.Tags,
		Schedule:      ToSchedules(dg.Schedule),
	}
}

func ToSchedules(schedules []dag.Schedule) []*models.Schedule {
	var result []*models.Schedule
	for _, item := range schedules {
		result = append(result, ToSchedule(item))
	}
	return result
}

func ToSchedule(schedule dag.Schedule) *models.Schedule {
	return &models.Schedule{
		Expression: swag.String(schedule.Expression),
	}
}
