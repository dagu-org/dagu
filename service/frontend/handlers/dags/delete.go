package dags

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/controller"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/service/frontend/handlers/response"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"path/filepath"
)

func Delete(params operations.DeleteDagParams) *response.CodedError {
	// TODO: change this to dependency injection
	cfg := config.Get()

	filename := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", params.DagID))
	dr := controller.NewDAGStatusReader(jsondb.New())
	dagStatus, err := dr.ReadStatus(filename, false)
	if err != nil {
		return response.NewNotFoundError(err)
	}

	ctrl := controller.New(dagStatus.DAG, jsondb.New())
	if err := ctrl.DeleteDAG(); err != nil {
		return response.NewInternalError(err)
	}
	return nil
}
