package workflow

import (
	"fmt"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/service/frontend/http/api/response"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
	"path/filepath"
)

func Delete(params operations.DeleteWorkflowParams) *response.CodedError {
	// TODO: change this to dependency injection
	cfg := config.Get()

	filename := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", params.WorkflowID))
	dr := controller.NewDAGStatusReader(jsondb.New())
	workflow, err := dr.ReadStatus(filename, false)
	if err != nil {
		return response.NewNotFoundError(err)
	}

	ctrl := controller.New(workflow.DAG, jsondb.New())
	if err := ctrl.DeleteDAG(); err != nil {
		return response.NewInternalError(err)
	}
	return nil
}
