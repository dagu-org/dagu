package api

import (
	"context"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
)

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	panic("unimplemented")
}

// DeleteDAG implements api.StrictServerInterface.
func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	panic("unimplemented")
}

// GetDAGDetails implements api.StrictServerInterface.
func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	panic("unimplemented")
}

// ListDAGs implements api.StrictServerInterface.
func (a *API) ListDAGs(ctx context.Context, request api.ListDAGsRequestObject) (api.ListDAGsResponseObject, error) {
	var opts []client.GetAllStatusOption
	if request.Params.Limit != nil {
		opts = append(opts, client.WithLimit(*request.Params.Limit))
	}
	if request.Params.Page != nil {
		opts = append(opts, client.WithPage(*request.Params.Page))
	}
	if request.Params.Name != nil {
		opts = append(opts, client.WithName(*request.Params.Name))
	}
	if request.Params.Tag != nil {
		opts = append(opts, client.WithTag(*request.Params.Tag))
	}

	dgs, result, err := a.client.GetAllStatus(ctx, opts...)
	if err != nil {
		return nil, newInternalError(err)
	}

	hasErr := len(result.ErrorList) > 0
	for _, d := range dgs {
		if d.Error != nil {
			hasErr = true
			break
		}
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:    ptr(result.ErrorList),
		PageCount: result.PageCount,
		HasError:  hasErr,
	}

	for _, d := range dgs {
		status := api.DAGStatus{
			Log:        ptr(d.Status.Log),
			Name:       d.Status.Name,
			Params:     ptr(d.Status.Params),
			Pid:        ptr(int(d.Status.PID)),
			RequestId:  d.Status.RequestID,
			StartedAt:  d.Status.StartedAt,
			FinishedAt: d.Status.FinishedAt,
			Status:     api.RunStatus(d.Status.Status),
			StatusText: api.RunStatusText(d.Status.StatusText),
		}

		item := api.DAGStatusFile{
			Error:     d.ErrorT,
			File:      d.File,
			Status:    status,
			Suspended: d.Suspended,
			DAG:       convertToDAG(d.DAG),
		}

		if d.Error != nil {
			item.Error = ptr(d.Error.Error())
		}

		resp.DAGs = append(resp.DAGs, item)
	}

	return resp, nil
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, request api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	panic("unimplemented")
}

// PostDAGAction implements api.StrictServerInterface.
func (a *API) PostDAGAction(ctx context.Context, request api.PostDAGActionRequestObject) (api.PostDAGActionResponseObject, error) {
	panic("unimplemented")
}

// SearchDAGs implements api.StrictServerInterface.
func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	panic("unimplemented")
}

func convertToDAG(dag *digraph.DAG) api.DAG {
	var schedules []api.Schedule
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{Expression: s.Expression})
	}

	return api.DAG{
		Name:          dag.Name,
		Group:         ptr(dag.Group),
		Description:   dag.Description,
		Params:        ptr(dag.Params),
		DefaultParams: ptr(dag.DefaultParams),
		Tags:          ptr(dag.Tags),
		Schedule:      ptr(schedules),
	}
}
