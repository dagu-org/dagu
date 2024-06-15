package response

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/service/frontend/gen/models"
	"github.com/go-openapi/swag"
	"github.com/samber/lo"
)

func NewListDagResponse(
	dagStatusList []*persistence.DAGStatus,
	errs []string,
	hasError bool,
) *models.ListDagsResponse {
	return &models.ListDagsResponse{
		DAGs: lo.Map(dagStatusList, func(item *persistence.DAGStatus, _ int) *models.DagListItem {
			return NewDagListItem(item)
		}),
		Errors:   errs,
		HasError: swag.Bool(hasError),
	}
}

func NewDagListItem(status *persistence.DAGStatus) *models.DagListItem {
	return &models.DagListItem{
		Dir:       swag.String(status.Dir),
		Error:     swag.String(NewErrorText(status.Error)),
		ErrorT:    status.ErrorT,
		File:      swag.String(status.File),
		Status:    NewDagStatus(status.Status),
		Suspended: swag.Bool(status.Suspended),
		DAG:       NewDAG(status.DAG),
	}
}

func NewDAG(dg *dag.DAG) *models.Dag {
	return &models.Dag{
		Name:          swag.String(dg.Name),
		Group:         swag.String(dg.Group),
		Description:   swag.String(dg.Description),
		Params:        dg.Params,
		DefaultParams: swag.String(dg.DefaultParams),
		Tags:          dg.Tags,
		Schedule:      NewSchedules(dg.Schedule),
	}
}

func NewSchedules(schedules []dag.Schedule) []*models.Schedule {
	var result []*models.Schedule
	for _, item := range schedules {
		result = append(result, NewSchedule(item))
	}
	return result
}

func NewSchedule(schedule dag.Schedule) *models.Schedule {
	return &models.Schedule{
		Expression: swag.String(schedule.Expression),
	}
}
