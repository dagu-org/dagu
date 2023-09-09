package dags

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/service/frontend/handlers/response"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
)

func Search(params operations.SearchDagsParams) (*models.SearchDagsResponse, *response.CodedError) {
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

	return response.ToSearchDAGsResponse(ret, errs), nil
}
