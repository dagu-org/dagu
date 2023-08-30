package workflow

import (
	"errors"
	"fmt"
	"github.com/samber/lo"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
	"github.com/yohamta/dagu/service/frontend/http/api/response"
	"github.com/yohamta/dagu/service/frontend/models"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
	"path"
	"path/filepath"
	"strings"
)

var (
	errInvalidArgs = errors.New("invalid argument")
)

func PostAction(params operations.PostWorkflowActionParams) (*models.PostWorkflowActionResponse, *response.CodedError) {
	// TODO: change this to dependency injection
	cfg := config.Get()

	file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", params.WorkflowID))
	dr := controller.NewDAGStatusReader(jsondb.New())
	d, err := dr.ReadStatus(file, false)

	if err != nil && params.Action != "save" {
		return nil, response.NewBadRequestError(err)
	}

	ctrl := controller.New(d.DAG, jsondb.New())

	switch params.Action {
	case "start":
		if d.Status.Status == scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(errInvalidArgs)
		}
		ctrl.StartAsync(cfg.Command, cfg.WorkDir, lo.FromPtr(params.Params))

	case "suspend":
		sc := suspend.NewSuspendChecker(storage.NewStorage(config.Get().SuspendFlagsDir))
		_ = sc.ToggleSuspend(d.DAG, lo.FromPtr(params.Value) == "true")

	case "stop":
		if d.Status.Status != scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(fmt.Errorf("the DAG is not running: %w", errInvalidArgs))
		}
		err = ctrl.Stop()
		if err != nil {
			return nil, response.NewBadRequestError(fmt.Errorf("error trying to stop the DAG: %w", err))
		}

	case "retry":
		if params.RequestID == nil {
			return nil, response.NewBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
		}
		err = ctrl.Retry(cfg.Command, cfg.WorkDir, lo.FromPtr(params.RequestID))
		if err != nil {
			return nil, response.NewInternalError(fmt.Errorf("error trying to retry the DAG: %w", err))
		}

	case "mark-success":
		if d.Status.Status == scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
		}
		if params.RequestID == nil {
			return nil, response.NewBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
		}
		if params.Step == nil {
			return nil, response.NewBadRequestError(fmt.Errorf("step name is required: %w", errInvalidArgs))
		}

		err = updateStatus(ctrl, lo.FromPtr(params.RequestID), lo.FromPtr(params.Step), scheduler.NodeStatus_Success)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "mark-failed":
		if d.Status.Status == scheduler.SchedulerStatus_Running {
			return nil, response.NewBadRequestError(fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
		}
		if params.RequestID == nil {
			return nil, response.NewBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
		}
		if params.Step == nil {
			return nil, response.NewBadRequestError(fmt.Errorf("step name is required: %w", errInvalidArgs))
		}

		err = updateStatus(ctrl, lo.FromPtr(params.RequestID), lo.FromPtr(params.Step), scheduler.NodeStatus_Error)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "save":
		err := ctrl.UpdateDAGSpec(lo.FromPtr(params.Value))
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "rename":
		newFile := nameWithExt(path.Join(cfg.DAGs, lo.FromPtr(params.Value)))
		c := controller.New(d.DAG, jsondb.New())
		err := c.MoveDAG(file, newFile)
		if err != nil {
			return nil, response.NewInternalError(err)
		}
		return &models.PostWorkflowActionResponse{NewWorkflowID: lo.FromPtr(params.Value)}, nil

	default:
		return nil, response.NewBadRequestError(fmt.Errorf("invalid action: %s", params.Action))
	}

	return &models.PostWorkflowActionResponse{}, nil
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
	found := false
	for i := range status.Nodes {
		if status.Nodes[i].Step.Name == step {
			status.Nodes[i].Status = to
			status.Nodes[i].StatusText = to.String()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("step was not found: %s", step)
	}
	return ctrl.UpdateStatus(status)
}
