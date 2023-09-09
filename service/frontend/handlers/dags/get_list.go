package dags

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/service/frontend/handlers/response"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"github.com/samber/lo"
	"path/filepath"
)

func GetList(_ operations.ListDagsParams) (*models.ListDagsResponse, *response.CodedError) {
	cfg := config.Get()

	// TODO: fix this to use dags store & history store
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

	return response.ToListDagResponse(dags, errs, hasErr), nil
}
