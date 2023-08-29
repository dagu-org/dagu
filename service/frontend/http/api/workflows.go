package api

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/samber/lo"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/service/frontend/http/api/response"
	"github.com/yohamta/dagu/service/frontend/models"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
	"path/filepath"
)

func registerWorkflows(api *operations.DaguAPI) {
	api.ListWorkflowsHandler = operations.ListWorkflowsHandlerFunc(
		func(params operations.ListWorkflowsParams) middleware.Responder {
			resp, err := listWorkflows(params)
			if err != nil {
				return operations.NewListWorkflowsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewListWorkflowsOK().WithPayload(resp)
		})
}

func listWorkflows(params operations.ListWorkflowsParams) (*models.ListWorkflowsResponse, *CodedError) {
	cfg := config.Get()

	// TODO: fix this to use workflow store & history store
	dir := filepath.Join(cfg.DAGs)
	dr := controller.NewDAGStatusReader(jsondb.New())
	dags, errs, err := dr.ReadAllStatus(dir)
	if err != nil {
		return nil, NewInternalError(err)
	}

	// TODO: remove this if it's not needed
	_, _, hasErr := lo.FindIndexOf(dags, func(d *controller.DAGStatus) bool {
		return d.Error != nil
	})

	if len(errs) > 0 {
		hasErr = true
	}

	return response.ToListWorkflowResponse(dags, errs, hasErr), nil
}
