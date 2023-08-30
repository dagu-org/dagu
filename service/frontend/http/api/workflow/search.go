package workflow

import (
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/service/frontend/http/api/response"
	"github.com/yohamta/dagu/service/frontend/models"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
)

func Search(params operations.SearchWorkflowsParams) (*models.SearchWorkflowsResponse, *response.CodedError) {
	// TODO: change this to dependency injection
	cfg := config.Get()

	query := params.Q
	if query == "" {
		return nil, response.NewBadRequestError(errInvalidArgs)
	}

	ret, errs, err := controller.GrepDAG(cfg.DAGs, query)
	if err != nil {
		return nil, response.NewInternalError(err)
	}

	return response.ToSearchWorkflowsResponse(ret, errs), nil
}
