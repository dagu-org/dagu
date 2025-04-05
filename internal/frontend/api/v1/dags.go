package api

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence"
)

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	name, err := a.client.CreateDAG(ctx, request.Body.Name)
	if err != nil {
		if errors.Is(err, persistence.ErrDAGAlreadyExists) {
			return nil, newBadRequestError(api.ErrorCodeBadRequest, err)
		}
		return nil, newInternalError(err)
	}
	return &api.CreateDAG201JSONResponse{
		Name: name,
	}, nil
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
	var opts []client.ListStatusOption
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

	result, err := a.client.ListStatus(ctx, opts...)
	if err != nil {
		return nil, newInternalError(err)
	}

	hasErr := len(result.Errors) > 0
	for _, item := range result.Items {
		if item.Error != nil {
			hasErr = true
			break
		}
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:    ptr(result.Errors),
		PageCount: result.TotalPage,
		HasError:  hasErr,
	}

	for _, item := range result.Items {
		status := api.DAGStatus{
			Log:        ptr(item.Status.Log),
			Name:       item.Status.Name,
			Params:     ptr(item.Status.Params),
			Pid:        ptr(int(item.Status.PID)),
			RequestId:  item.Status.RequestID,
			StartedAt:  item.Status.StartedAt,
			FinishedAt: item.Status.FinishedAt,
			Status:     api.RunStatus(item.Status.Status),
			StatusText: api.RunStatusText(item.Status.StatusText),
		}

		dag := api.DAGStatusFile{
			Error:     item.ErrorT,
			File:      item.File,
			Status:    status,
			Suspended: item.Suspended,
			DAG:       convertToDAG(item.DAG),
		}

		if item.Error != nil {
			dag.Error = ptr(item.Error.Error())
		}

		resp.DAGs = append(resp.DAGs, dag)
	}

	return resp, nil
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	tags, errs, err := a.client.GetTagList(ctx)
	if err != nil {
		return nil, newInternalError(err)
	}
	return &api.ListTags200JSONResponse{
		Tags:   tags,
		Errors: errs,
	}, nil
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
