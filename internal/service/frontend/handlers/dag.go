package handlers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/service/frontend/gen/models"
	"github.com/dagu-dev/dagu/internal/service/frontend/gen/restapi/operations"
	"github.com/dagu-dev/dagu/internal/service/frontend/handlers/response"
	"github.com/dagu-dev/dagu/internal/service/frontend/server"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/swag"
	"github.com/samber/lo"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const (
	dagTabTypeStatus       = "status"
	dagTabTypeSpec         = "spec"
	dagTabTypeHistory      = "history"
	dagTabTypeStepLog      = "log"
	dagTabTypeSchedulerLog = "scheduler-log"
)

var (
	errInvalidArgs        = errors.New("invalid argument")
	ErrFailedToReadStatus = errors.New("failed to read status")
	ErrStepNotFound       = errors.New("step was not found")
	ErrReadingLastStatus  = errors.New("error reading the last status")
)

type DAGHandler struct {
	engine engine.Engine
}

func NewDAGHandler(eng engine.Engine) server.New {
	return &DAGHandler{
		engine: eng,
	}
}

func (h *DAGHandler) Configure(api *operations.DaguAPI) {
	api.ListDagsHandler = operations.ListDagsHandlerFunc(
		func(params operations.ListDagsParams) middleware.Responder {
			resp, err := h.GetList(params)
			if err != nil {
				return operations.NewListDagsDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewListDagsOK().WithPayload(resp)
		})

	api.GetDagDetailsHandler = operations.GetDagDetailsHandlerFunc(
		func(params operations.GetDagDetailsParams) middleware.Responder {
			resp, err := h.GetDetail(params)
			if err != nil {
				return operations.NewGetDagDetailsDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewGetDagDetailsOK().WithPayload(resp)
		})

	api.PostDagActionHandler = operations.PostDagActionHandlerFunc(
		func(params operations.PostDagActionParams) middleware.Responder {
			resp, err := h.PostAction(params)
			if err != nil {
				return operations.NewPostDagActionDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewPostDagActionOK().WithPayload(resp)
		})

	api.CreateDagHandler = operations.CreateDagHandlerFunc(
		func(params operations.CreateDagParams) middleware.Responder {
			resp, err := h.Create(params)
			if err != nil {
				return operations.NewCreateDagDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewCreateDagOK().WithPayload(resp)
		})

	api.DeleteDagHandler = operations.DeleteDagHandlerFunc(
		func(params operations.DeleteDagParams) middleware.Responder {
			err := h.Delete(params)
			if err != nil {
				return operations.NewDeleteDagDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewDeleteDagOK()
		})

	api.SearchDagsHandler = operations.SearchDagsHandlerFunc(
		func(params operations.SearchDagsParams) middleware.Responder {
			resp, err := h.Search(params)
			if err != nil {
				return operations.NewSearchDagsDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewSearchDagsOK().WithPayload(resp)
		})
}

func (h *DAGHandler) Create(
	params operations.CreateDagParams,
) (*models.CreateDagResponse, *response.CodedError) {
	switch lo.FromPtr(params.Body.Action) {
	case "new":
		name := *params.Body.Value
		id, err := h.engine.CreateDAG(name)
		if err != nil {
			return nil, response.NewInternalError(err)
		}
		return &models.CreateDagResponse{DagID: swag.String(id)}, nil
	default:
		return nil, response.NewBadRequestError(errInvalidArgs)
	}
}
func (h *DAGHandler) Delete(
	params operations.DeleteDagParams,
) *response.CodedError {
	dagStatus, err := h.engine.GetStatus(params.DagID)
	if err != nil {
		return response.NewNotFoundError(err)
	}
	if err := h.engine.DeleteDAG(
		params.DagID, dagStatus.DAG.Location,
	); err != nil {
		return response.NewInternalError(err)
	}
	return nil
}

func (h *DAGHandler) GetList(
	_ operations.ListDagsParams,
) (*models.ListDagsResponse, *response.CodedError) {
	dags, errs, err := h.engine.GetAllStatus()
	if err != nil {
		return nil, response.NewInternalError(err)
	}

	_, _, hasErr := lo.FindIndexOf(dags, func(d *persistence.DAGStatus) bool {
		return d.Error != nil
	})

	if len(errs) > 0 {
		hasErr = true
	}

	return response.NewListDagResponse(dags, errs, hasErr), nil
}

func (h *DAGHandler) GetDetail(
	params operations.GetDagDetailsParams,
) (*models.GetDagDetailsResponse, *response.CodedError) {
	dagID := params.DagID

	tab := dagTabTypeStatus
	if params.Tab != nil {
		tab = *params.Tab
	}

	logFile := params.File
	stepName := params.Step

	dagStatus, err := h.engine.GetStatus(dagID)
	if dagStatus == nil {
		return nil, response.NewNotFoundError(err)
	}

	resp := response.NewGetDagDetailResponse(
		dagStatus,
		tab,
	)

	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case dagTabTypeStatus:
	case dagTabTypeSpec:
		dagContent, err := h.engine.GetDAGSpec(dagID)
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.Definition = swag.String(dagContent)

	case dagTabTypeHistory:
		logs := h.engine.GetRecentHistory(dagStatus.DAG, 30)
		resp.LogData = response.NewDagLogResponse(logs)

	case dagTabTypeStepLog:
		stepLog, err := h.getStepLog(
			dagStatus.DAG, lo.FromPtr(logFile), lo.FromPtr(stepName),
		)
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.StepLog = stepLog

	case dagTabTypeSchedulerLog:
		schedulerLog, err := h.readSchedulerLog(
			dagStatus.DAG, lo.FromPtr(logFile),
		)
		if err != nil {
			return nil, response.NewNotFoundError(err)
		}
		resp.ScLog = schedulerLog

	default:
	}

	return resp, nil
}

func (h *DAGHandler) getStepLog(
	dg *dag.DAG,
	logFile, stepName string,
) (*models.DagStepLogResponse, error) {
	var stepByName = map[dag.HandlerType]*model.Node{
		dag.HandlerOnSuccess: nil,
		dag.HandlerOnFailure: nil,
		dag.HandlerOnCancel:  nil,
		dag.HandlerOnExit:    nil,
	}

	var status *model.Status

	if logFile == "" {
		latestStatus, err := h.engine.GetLatestStatus(dg)
		if err != nil {
			return nil, ErrFailedToReadStatus
		}
		status = latestStatus
	} else {
		unmarshalledStatus, err := jsondb.ParseFile(logFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", logFile, err)
		}
		status = unmarshalledStatus
	}

	stepByName[dag.HandlerOnSuccess] = status.OnSuccess
	stepByName[dag.HandlerOnFailure] = status.OnFailure
	stepByName[dag.HandlerOnCancel] = status.OnCancel
	stepByName[dag.HandlerOnExit] = status.OnExit

	node, ok := lo.Find(status.Nodes, func(item *model.Node) bool {
		return item.Name == stepName
	})
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrStepNotFound, stepName)
	}

	logContent, err := getLogFileContent(
		node.Log, h.engine.Config().LogEncodingCharset,
	)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Log, err)
	}

	return response.NewDagStepLogResponse(node.Log, logContent, node), nil
}

func getLogFileContent(fileName, enc string) (string, error) {
	var decoder *encoding.Decoder
	if strings.ToLower(enc) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}
	logContent, err := readFileContent(fileName, decoder)
	return string(logContent), err
}

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

func (h *DAGHandler) readSchedulerLog(
	dg *dag.DAG,
	statusFile string,
) (*models.DagSchedulerLogResponse, error) {

	var logFile string
	if statusFile == "" {
		lastStatus, err := h.engine.GetLatestStatus(dg)
		if err != nil {
			return nil, ErrReadingLastStatus
		}
		logFile = lastStatus.Log
	} else {
		status, err := jsondb.ParseFile(statusFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", statusFile, err)
		}
		logFile = status.Log
	}

	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}

	return response.NewDagSchedulerLogResponse(logFile, string(content)), nil
}

// nolint // cognitive complexity
func (h *DAGHandler) PostAction(
	params operations.PostDagActionParams,
) (*models.PostDagActionResponse, *response.CodedError) {
	dagStatus, err := h.engine.GetStatus(params.DagID)

	if err != nil && *params.Body.Action != "save" {
		return nil, response.NewBadRequestError(err)
	}

	switch *params.Body.Action {
	case "start":
		if dagStatus.Status.Status == scheduler.StatusRunning {
			return nil, response.NewBadRequestError(errInvalidArgs)
		}
		h.engine.StartAsync(dagStatus.DAG, params.Body.Params)

	case "suspend":
		_ = h.engine.ToggleSuspend(params.DagID, params.Body.Value == "true")

	case "stop":
		if dagStatus.Status.Status != scheduler.StatusRunning {
			return nil, response.NewBadRequestError(
				fmt.Errorf("the DAG is not running: %w", errInvalidArgs),
			)
		}
		if err := h.engine.Stop(dagStatus.DAG); err != nil {
			return nil, response.NewBadRequestError(
				fmt.Errorf("error trying to stop the DAG: %w", err),
			)
		}

	case "retry":
		if params.Body.RequestID == "" {
			return nil, response.NewBadRequestError(
				fmt.Errorf("request-id is required: %w", errInvalidArgs),
			)
		}
		if err := h.engine.Retry(dagStatus.DAG, params.Body.RequestID); err != nil {
			return nil, response.NewInternalError(
				fmt.Errorf("error trying to retry the DAG: %w", err),
			)
		}

	case "mark-success":
		if dagStatus.Status.Status == scheduler.StatusRunning {
			return nil, response.NewBadRequestError(
				fmt.Errorf("the DAG is still running: %w", errInvalidArgs),
			)
		}
		if params.Body.RequestID == "" {
			return nil, response.NewBadRequestError(
				fmt.Errorf("request-id is required: %w", errInvalidArgs),
			)
		}
		if params.Body.Step == "" {
			return nil, response.NewBadRequestError(
				fmt.Errorf("step name is required: %w", errInvalidArgs),
			)
		}

		err = h.updateStatus(
			dagStatus.DAG,
			params.Body.RequestID,
			params.Body.Step,
			scheduler.NodeStatusSuccess,
		)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "mark-failed":
		if dagStatus.Status.Status == scheduler.StatusRunning {
			return nil, response.NewBadRequestError(
				fmt.Errorf("the DAG is still running: %w", errInvalidArgs),
			)
		}
		if params.Body.RequestID == "" {
			return nil, response.NewBadRequestError(
				fmt.Errorf("request-id is required: %w", errInvalidArgs),
			)
		}
		if params.Body.Step == "" {
			return nil, response.NewBadRequestError(
				fmt.Errorf("step name is required: %w", errInvalidArgs),
			)
		}

		err = h.updateStatus(
			dagStatus.DAG,
			params.Body.RequestID,
			params.Body.Step,
			scheduler.NodeStatusError,
		)
		if err != nil {
			return nil, response.NewInternalError(err)
		}

	case "save":
		if err := h.engine.UpdateDAG(params.DagID, params.Body.Value); err != nil {
			return nil, response.NewInternalError(err)
		}

	case "rename":
		newName := params.Body.Value
		if newName == "" {
			return nil, response.NewBadRequestError(
				fmt.Errorf("new name is required: %w", errInvalidArgs),
			)
		}
		if err := h.engine.Rename(params.DagID, newName); err != nil {
			return nil, response.NewInternalError(err)
		}
		return &models.PostDagActionResponse{NewDagID: params.Body.Value}, nil

	default:
		return nil, response.NewBadRequestError(
			fmt.Errorf("invalid action: %s", *params.Body.Action),
		)
	}

	return &models.PostDagActionResponse{}, nil
}

func (h *DAGHandler) updateStatus(
	dg *dag.DAG, reqID, step string, to scheduler.NodeStatus,
) error {
	status, err := h.engine.GetStatusByRequestID(dg, reqID)
	if err != nil {
		return err
	}

	_, idx, ok := lo.FindIndexOf(status.Nodes, func(item *model.Node) bool {
		return item.Step.Name == step
	})
	if !ok {
		return fmt.Errorf("%w: %s", ErrStepNotFound, step)
	}

	status.Nodes[idx].Status = to
	status.Nodes[idx].StatusText = to.String()

	return h.engine.UpdateStatus(dg, status)
}

func (h *DAGHandler) Search(
	params operations.SearchDagsParams,
) (*models.SearchDagsResponse, *response.CodedError) {
	query := params.Q
	if query == "" {
		return nil, response.NewBadRequestError(errInvalidArgs)
	}

	ret, errs, err := h.engine.Grep(query)
	if err != nil {
		return nil, response.NewInternalError(err)
	}

	return response.NewSearchDAGsResponse(ret, errs), nil
}
