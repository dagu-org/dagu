package dags

import (
	"errors"
	"fmt"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/controller"
	domain "github.com/dagu-dev/dagu/internal/models"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
	"github.com/dagu-dev/dagu/service/frontend/handlers/response"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"github.com/samber/lo"
	"path"
	"path/filepath"
	"strings"
)

var (
	errInvalidArgs = errors.New("invalid argument")
)

func PostAction(params operations.PostDagActionParams) (*models.PostDagActionResponse, *response.CodedError) {
	// TODO: change this to dependency injection
	cfg := config.Get()

	file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", params.DagID))
	dr := controller.NewDAGStatusReader(jsondb.New())
	d, err := dr.ReadStatus(file, false)

	if err != nil && params.Body.Action != "save" {
		return nil, response.NewBadRequestError(err)
	}

	ctrl := controller.New(d.DAG, jsondb.New())

	switch params.Body.Action {
	case "start":
		if d.Status.Status == scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(errInvalidArgs)
		}
		ctrl.StartAsync(cfg.Command, cfg.WorkDir, params.Body.Params)

	case "suspend":
		sc := suspend.NewSuspendChecker(storage.NewStorage(config.Get().SuspendFlagsDir))
		_ = sc.ToggleSuspend(d.DAG, params.Body.Value == "true")

	case "stop":
		if d.Status.Status != scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(fmt.Errorf("the DAG is not running: %w", errInvalidArgs))
		}
		err = ctrl.Stop()
		if err != nil {
			return nil, response.NewBadRequestError(fmt.Errorf("error trying to stop the DAG: %w", err))
		}

	case "retry":
		if params.Body.RequestID == "" {
			return nil, response.NewBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
		}
		err = ctrl.Retry(cfg.Command, cfg.WorkDir, params.Body.RequestID)
		if err != nil {
			return nil, response.NewInternalError(fmt.Errorf("error trying to retry the DAG: %w", err))
		}

	case "mark-success":
		if d.Status.Status == scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
		}
		if params.Body.RequestID == "" {
			return nil, response.NewBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
		}
		if params.Body.Step == "" {
			return nil, response.NewBadRequestError(fmt.Errorf("step name is required: %w", errInvalidArgs))
		}

		err = updateStatus(ctrl, params.Body.RequestID, params.Body.Step, scheduler.NodeStatus_Success)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "mark-failed":
		if d.Status.Status == scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
		}
		if params.Body.RequestID == "" {
			return nil, response.NewBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
		}
		if params.Body.Step == "" {
			return nil, response.NewBadRequestError(fmt.Errorf("step name is required: %w", errInvalidArgs))
		}

		err = updateStatus(ctrl, params.Body.RequestID, params.Body.Step, scheduler.NodeStatus_Error)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "save":
		err := ctrl.UpdateDAGSpec(params.Body.Value)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "rename":
		newFile := nameWithExt(path.Join(cfg.DAGs, params.Body.Value))
		c := controller.New(d.DAG, jsondb.New())
		err := c.MoveDAG(file, newFile)
		if err != nil {
			return nil, response.NewInternalError(err)
		}
		return &models.PostDagActionResponse{NewDagID: params.Body.Value}, nil

	default:
		return nil, response.NewBadRequestError(fmt.Errorf("invalid action: %s", params.Body.Action))
	}

	return &models.PostDagActionResponse{}, nil
}

func nameWithExt(name string) string {
	s := strings.TrimSuffix(name, ".yaml")
	return fmt.Sprintf("%s.yaml", s)
}

func updateStatus(ctrl *controller.DAGController, reqId, step string, to scheduler.NodeStatus) error {
	status, err := ctrl.GetStatusByRequestId(reqId)
	if err != nil {
		return err
	}

	_, idx, ok := lo.FindIndexOf(status.Nodes, func(item *domain.Node) bool {
		return item.Step.Name == step
	})
	if !ok {
		return fmt.Errorf("step was not found: %s", step)
	}

	status.Nodes[idx].Status = to
	status.Nodes[idx].StatusText = to.String()

	return ctrl.UpdateStatus(status)
}
