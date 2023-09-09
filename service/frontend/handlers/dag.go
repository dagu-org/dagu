package handlers

import (
	"errors"
	"fmt"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/dagu-dev/dagu/internal/controller"
	domain "github.com/dagu-dev/dagu/internal/models"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
	"github.com/dagu-dev/dagu/service/frontend/handlers/response"
	"github.com/dagu-dev/dagu/service/frontend/http"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"github.com/samber/lo"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// TODO: separate API
	dagTabTypeStatus       = "status"
	dagTabTypeSpec         = "spec"
	dagTabTypeHistory      = "history"
	dagTabTypeStepLog      = "log"
	dagTabTypeSchedulerLog = "scheduler-log"
)

var (
	errInvalidArgs = errors.New("invalid argument")
)

type DAG struct{}

func NewDAG() http.Handler {
	return &DAG{}
}

func (d *DAG) Configure(api *operations.DaguAPI) {
	api.ListDagsHandler = operations.ListDagsHandlerFunc(
		func(params operations.ListDagsParams) middleware.Responder {
			resp, err := d.GetList(params)
			if err != nil {
				return operations.NewListDagsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewListDagsOK().WithPayload(resp)
		})

	api.GetDagDetailsHandler = operations.GetDagDetailsHandlerFunc(
		func(params operations.GetDagDetailsParams) middleware.Responder {
			resp, err := d.GetDetail(params)
			if err != nil {
				return operations.NewGetDagDetailsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewGetDagDetailsOK().WithPayload(resp)
		})

	api.PostDagActionHandler = operations.PostDagActionHandlerFunc(
		func(params operations.PostDagActionParams) middleware.Responder {
			resp, err := d.PostAction(params)
			if err != nil {
				return operations.NewPostDagActionDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewPostDagActionOK().WithPayload(resp)
		})

	api.CreateDagHandler = operations.CreateDagHandlerFunc(
		func(params operations.CreateDagParams) middleware.Responder {
			resp, err := d.Create(params)
			if err != nil {
				return operations.NewCreateDagDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewCreateDagOK().WithPayload(resp)
		})

	api.DeleteDagHandler = operations.DeleteDagHandlerFunc(
		func(params operations.DeleteDagParams) middleware.Responder {
			err := d.Delete(params)
			if err != nil {
				return operations.NewDeleteDagDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewDeleteDagOK()
		})

	api.SearchDagsHandler = operations.SearchDagsHandlerFunc(
		func(params operations.SearchDagsParams) middleware.Responder {
			resp, err := d.Search(params)
			if err != nil {
				return operations.NewSearchDagsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewSearchDagsOK().WithPayload(resp)
		})
}

func (*DAG) Create(params operations.CreateDagParams) (*models.CreateDagResponse, *response.CodedError) {
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
func (*DAG) Delete(params operations.DeleteDagParams) *response.CodedError {
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

func (*DAG) GetList(_ operations.ListDagsParams) (*models.ListDagsResponse, *response.CodedError) {
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

func (*DAG) GetDetail(params operations.GetDagDetailsParams) (*models.GetDagDetailsResponse, *response.CodedError) {
	dagID := params.DagID

	// TODO: separate API
	// optional params
	tab := dagTabTypeStatus
	if params.Tab != nil {
		tab = *params.Tab
	}

	logFile := params.File
	stepName := params.Step

	// TODO: change this to dependency injection
	cfg := config.Get()

	file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dagID))
	dr := controller.NewDAGStatusReader(jsondb.New())
	dagStatus, err := dr.ReadStatus(file, false)
	if dagStatus == nil {
		return nil, response.NewNotFoundError(err)
	}

	ctrl := controller.New(dagStatus.DAG, jsondb.New())
	resp := response.ToGetDagDetailResponse(
		dagStatus,
		tab,
	)

	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case dagTabTypeStatus:
	case dagTabTypeSpec:
		dagContent, err := readFileContent(file, nil)
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.Definition = lo.ToPtr(string(dagContent))

	case dagTabTypeHistory:
		logs := controller.New(dagStatus.DAG, jsondb.New()).GetRecentStatuses(30)
		resp.LogData = response.ToDagLogResponse(logs)

	case dagTabTypeStepLog:
		stepLog, err := getStepLog(ctrl, lo.FromPtr(logFile), lo.FromPtr(stepName))
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.StepLog = stepLog

	case dagTabTypeSchedulerLog:
		schedulerLog, err := readSchedulerLog(ctrl, lo.FromPtr(logFile))
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.ScLog = schedulerLog

	default:
	}

	return resp, nil
}

func getStepLog(c *controller.DAGController, logFile, stepName string) (*models.DagStepLogResponse, error) {
	var stepByName = map[string]*domain.Node{
		constants.OnSuccess: nil,
		constants.OnFailure: nil,
		constants.OnCancel:  nil,
		constants.OnExit:    nil,
	}

	var status *domain.Status
	if logFile == "" {
		s, err := c.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("failed to read status")
		}
		status = s
	} else {
		s, err := jsondb.ParseFile(logFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", logFile, err)
		}
		status = s
	}

	stepByName[constants.OnSuccess] = status.OnSuccess
	stepByName[constants.OnFailure] = status.OnFailure
	stepByName[constants.OnCancel] = status.OnCancel
	stepByName[constants.OnExit] = status.OnExit

	node, ok := lo.Find(status.Nodes, func(item *domain.Node) bool {
		return item.Name == stepName
	})
	if !ok {
		return nil, fmt.Errorf("step name was not found %s", stepName)
	}

	logContent, err := getLogFileContent(node.Log)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Log, err)
	}

	return response.ToDagStepLogResponse(node.Log, logContent, node), nil
}

func getLogFileContent(fileName string) (string, error) {
	// TODO: fix this to change to dependency injection
	enc := config.Get().LogEncodingCharset

	var decoder *encoding.Decoder
	if strings.ToLower(enc) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}
	logContent, err := readFileContent(fileName, decoder)
	return string(logContent), err
}

// TODO: refactor this
func readFileContent(f string, decoder *encoding.Decoder) ([]byte, error) {
	if decoder == nil {
		return os.ReadFile(f)
	}

	r, err := os.Open(f)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", f, err)
	}
	defer func() {
		_ = r.Close()
	}()
	tr := transform.NewReader(r, decoder)
	ret, err := io.ReadAll(tr)
	return ret, err
}

func readSchedulerLog(ctrl *controller.DAGController, statusFile string) (*models.DagSchedulerLogResponse, error) {
	var (
		logFile string
	)
	if statusFile == "" {
		s, err := ctrl.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("error reading the last status")
		}
		logFile = s.Log
	} else {
		s, err := jsondb.ParseFile(statusFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", statusFile, err)
		}
		logFile = s.Log
	}
	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}
	return response.ToDagSchedulerLogResponse(logFile, string(content)), nil
}

func (*DAG) PostAction(params operations.PostDagActionParams) (*models.PostDagActionResponse, *response.CodedError) {
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

func (*DAG) Search(params operations.SearchDagsParams) (*models.SearchDagsResponse, *response.CodedError) {
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
