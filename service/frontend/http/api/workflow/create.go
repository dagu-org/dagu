package workflow

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/service/frontend/http/api/response"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"github.com/samber/lo"
	"path"
)

func Create(params operations.CreateDagParams) (*models.CreateDagResponse, *response.CodedError) {
	// TODO: change this to dependency injection
	cfg := config.Get()

	switch lo.FromPtr(params.Body.Action) {
	case "new":
		filename := nameWithExt(path.Join(cfg.DAGs, lo.FromPtr(params.Body.Value)))
		err := controller.CreateDAG(filename)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

		return &models.CreateDagResponse{DagID: params.Body.Value}, nil
	default:
		return nil, response.NewBadRequestError(errInvalidArgs)
	}
}
