package workflow

import (
	"github.com/samber/lo"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/service/frontend/http/api/response"
	"github.com/yohamta/dagu/service/frontend/models"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
	"path/filepath"
)

func GetList(_ operations.ListWorkflowsParams) (*models.ListWorkflowsResponse, *response.CodedError) {
	cfg := config.Get()

	// TODO: fix this to use workflow store & history store
	dir := filepath.Join(cfg.DAGs)
	dr := controller.NewDAGStatusReader(jsondb.New())
	dags, errs, err := dr.ReadAllStatus(dir)
	if err != nil {
		return nil, response.NewInternalError(err)
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
