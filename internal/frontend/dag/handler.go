package dag

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/frontend/gen/models"
	"github.com/dagu-dev/dagu/internal/frontend/gen/restapi/operations"
	"github.com/dagu-dev/dagu/internal/frontend/server"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
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

// Handler is a handler for the DAG API.
type Handler struct {
	engine             engine.Engine
	logEncodingCharset string
}

type NewHandlerArgs struct {
	Engine             engine.Engine
	LogEncodingCharset string
}

func NewHandler(args *NewHandlerArgs) server.Handler {
	return &Handler{
		engine:             args.Engine,
		logEncodingCharset: args.LogEncodingCharset,
	}
}

func (h *Handler) Configure(api *operations.DaguAPI) {
	api.ListDagsHandler = operations.ListDagsHandlerFunc(
		func(params operations.ListDagsParams) middleware.Responder {
			resp, err := h.getList(params)
			if err != nil {
				return operations.NewListDagsDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewListDagsOK().WithPayload(resp)
		})

	api.GetDagDetailsHandler = operations.GetDagDetailsHandlerFunc(
		func(params operations.GetDagDetailsParams) middleware.Responder {
			resp, err := h.getDetail(params)
			if err != nil {
				return operations.NewGetDagDetailsDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewGetDagDetailsOK().WithPayload(resp)
		})

	api.PostDagActionHandler = operations.PostDagActionHandlerFunc(
		func(params operations.PostDagActionParams) middleware.Responder {
			resp, err := h.postAction(params)
			if err != nil {
				return operations.NewPostDagActionDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewPostDagActionOK().WithPayload(resp)
		})

	api.CreateDagHandler = operations.CreateDagHandlerFunc(
		func(params operations.CreateDagParams) middleware.Responder {
			resp, err := h.createDAG(params)
			if err != nil {
				return operations.NewCreateDagDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewCreateDagOK().WithPayload(resp)
		})

	api.DeleteDagHandler = operations.DeleteDagHandlerFunc(
		func(params operations.DeleteDagParams) middleware.Responder {
			err := h.deleteDAG(params)
			if err != nil {
				return operations.NewDeleteDagDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewDeleteDagOK()
		})

	api.SearchDagsHandler = operations.SearchDagsHandlerFunc(
		func(params operations.SearchDagsParams) middleware.Responder {
			resp, err := h.searchDAGs(params)
			if err != nil {
				return operations.NewSearchDagsDefault(err.Code).
					WithPayload(err.APIError)
			}
			return operations.NewSearchDagsOK().WithPayload(resp)
		})
}

func (h *Handler) createDAG(
	params operations.CreateDagParams,
) (*models.CreateDagResponse, *codedError) {
	if params.Body.Action == nil || params.Body.Value == nil {
		return nil, newBadRequestError(errInvalidArgs)
	}

	switch *params.Body.Action {
	case "new":
		name := *params.Body.Value
		id, err := h.engine.CreateDAG(name)
		if err != nil {
			return nil, newInternalError(err)
		}
		return &models.CreateDagResponse{DagID: swag.String(id)}, nil
	default:
		return nil, newBadRequestError(errInvalidArgs)
	}
}
func (h *Handler) deleteDAG(
	params operations.DeleteDagParams,
) *codedError {
	dagStatus, err := h.engine.GetStatus(params.DagID)
	if err != nil {
		return newNotFoundError(err)
	}
	if err := h.engine.DeleteDAG(
		params.DagID, dagStatus.DAG.Location,
	); err != nil {
		return newInternalError(err)
	}
	return nil
}

func (h *Handler) getList(
	_ operations.ListDagsParams,
) (*models.ListDagsResponse, *codedError) {
	dags, errs, err := h.engine.GetAllStatus()
	if err != nil {
		return nil, newInternalError(err)
	}

	hasErr := len(errs) > 0
	if !hasErr {
		// Check if any DAG has an error
		for _, d := range dags {
			if d.Error != nil {
				hasErr = true
				break
			}
		}
	}

	resp := &models.ListDagsResponse{
		Errors:   errs,
		HasError: swag.Bool(hasErr),
	}

	for _, dagStatus := range dags {
		s := dagStatus.Status

		status := &models.DagStatus{
			Log:        swag.String(s.Log),
			Name:       swag.String(s.Name),
			Params:     swag.String(s.Params),
			Pid:        swag.Int64(int64(s.Pid)),
			RequestID:  swag.String(s.RequestID),
			StartedAt:  swag.String(s.StartedAt),
			FinishedAt: swag.String(s.FinishedAt),
			Status:     swag.Int64(int64(s.Status)),
			StatusText: swag.String(s.StatusText),
		}

		item := &models.DagListItem{
			Dir:       swag.String(dagStatus.Dir),
			ErrorT:    dagStatus.ErrorT,
			File:      swag.String(dagStatus.File),
			Status:    status,
			Suspended: swag.Bool(dagStatus.Suspended),
			DAG:       convertToDAG(dagStatus.DAG),
		}

		if dagStatus.Error != nil {
			item.Error = swag.String(dagStatus.Error.Error())
		}

		resp.DAGs = append(resp.DAGs, item)
	}

	return resp, nil
}

func (h *Handler) getDetail(
	params operations.GetDagDetailsParams,
) (*models.GetDagDetailsResponse, *codedError) {
	dagID := params.DagID

	tab := dagTabTypeStatus
	if params.Tab != nil {
		tab = *params.Tab
	}

	dagStatus, err := h.engine.GetStatus(dagID)
	if dagStatus == nil {
		return nil, newNotFoundError(err)
	}

	dg := dagStatus.DAG

	var steps []*models.StepObject
	for _, step := range dg.Steps {
		steps = append(steps, convertToStepObject(step))
	}

	hdlrs := dagStatus.DAG.HandlerOn

	handlerOn := &models.HandlerOn{}
	if hdlrs.Failure != nil {
		handlerOn.Failure = convertToStepObject(*hdlrs.Failure)
	}
	if hdlrs.Success != nil {
		handlerOn.Success = convertToStepObject(*hdlrs.Success)
	}
	if hdlrs.Cancel != nil {
		handlerOn.Cancel = convertToStepObject(*hdlrs.Cancel)
	}
	if hdlrs.Exit != nil {
		handlerOn.Exit = convertToStepObject(*hdlrs.Exit)
	}

	var schedules []*models.Schedule
	for _, s := range dg.Schedule {
		schedules = append(schedules, &models.Schedule{
			Expression: swag.String(s.Expression),
		})
	}

	var preconditions []*models.Condition
	for _, p := range dg.Preconditions {
		preconditions = append(preconditions, &models.Condition{
			Condition: p.Condition,
			Expected:  p.Expected,
		})
	}

	dagDetail := &models.DagDetail{
		DefaultParams:     swag.String(dg.DefaultParams),
		Delay:             swag.Int64(int64(dg.Delay)),
		Description:       swag.String(dg.Description),
		Env:               dg.Env,
		Group:             swag.String(dg.Group),
		HandlerOn:         handlerOn,
		HistRetentionDays: swag.Int64(int64(dg.HistRetentionDays)),
		Location:          swag.String(dg.Location),
		LogDir:            swag.String(dg.LogDir),
		MaxActiveRuns:     swag.Int64(int64(dg.MaxActiveRuns)),
		Name:              swag.String(dg.Name),
		Params:            dg.Params,
		Preconditions:     preconditions,
		Schedule:          schedules,
		Steps:             steps,
		Tags:              dg.Tags,
	}

	statusWithDetails := &models.DagStatusWithDetails{
		DAG:       dagDetail,
		Dir:       swag.String(dagStatus.Dir),
		ErrorT:    dagStatus.ErrorT,
		File:      swag.String(dagStatus.File),
		Status:    convertToStatusDetail(dagStatus.Status),
		Suspended: swag.Bool(dagStatus.Suspended),
	}

	if dagStatus.Error != nil {
		statusWithDetails.Error = swag.String(dagStatus.Error.Error())
	}

	resp := &models.GetDagDetailsResponse{
		Title:      swag.String(dagStatus.DAG.Name),
		DAG:        statusWithDetails,
		Tab:        swag.String(tab),
		Definition: swag.String(""),
		LogData:    nil,
		Errors:     []string{},
	}

	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case dagTabTypeStatus:
		return resp, nil

	case dagTabTypeSpec:
		return h.processSpecRequest(dagID, resp)

	case dagTabTypeHistory:
		return h.processLogRequest(resp, dg)

	case dagTabTypeStepLog:
		return h.processStepLogRequest(dg, params, resp)

	case dagTabTypeSchedulerLog:
		return h.processSchedulerLogRequest(dg, params, resp)

	default:
		return nil, newBadRequestError(errInvalidArgs)
	}
}

func (h *Handler) processSchedulerLogRequest(
	dg *dag.DAG,
	params operations.GetDagDetailsParams,
	resp *models.GetDagDetailsResponse,
) (*models.GetDagDetailsResponse, *codedError) {
	var logFile string

	if params.File != nil {
		status, err := jsondb.ParseFile(*params.File)
		if err != nil {
			return nil, newBadRequestError(err)
		}
		logFile = status.Log
	}

	if logFile == "" {
		lastStatus, err := h.engine.GetLatestStatus(dg)
		if err != nil {
			return nil, newInternalError(err)
		}
		logFile = lastStatus.Log
	}

	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, newInternalError(err)
	}

	resp.ScLog = &models.DagSchedulerLogResponse{
		LogFile: swag.String(logFile),
		Content: swag.String(string(content)),
	}

	return resp, nil
}

func (h *Handler) processStepLogRequest(
	dg *dag.DAG,
	params operations.GetDagDetailsParams,
	resp *models.GetDagDetailsResponse,
) (*models.GetDagDetailsResponse, *codedError) {
	var status *model.Status

	if params.Step == nil {
		return nil, newBadRequestError(errInvalidArgs)
	}

	if params.File != nil {
		s, err := jsondb.ParseFile(*params.File)
		if err != nil {
			return nil, newBadRequestError(err)
		}
		status = s
	}

	if status == nil {
		s, err := h.engine.GetLatestStatus(dg)
		if err != nil {
			return nil, newInternalError(err)
		}
		status = s
	}

	// Find the step in the status to get the log file.
	var node *model.Node

	for _, n := range status.Nodes {
		if n.Step.Name == *params.Step {
			node = n
		}
	}

	if node == nil {
		if status.OnSuccess != nil && status.OnSuccess.Name == *params.Step {
			node = status.OnSuccess
		}
		if status.OnFailure != nil && status.OnFailure.Name == *params.Step {
			node = status.OnFailure
		}
		if status.OnCancel != nil && status.OnCancel.Name == *params.Step {
			node = status.OnCancel
		}
		if status.OnExit != nil && status.OnExit.Name == *params.Step {
			node = status.OnExit
		}
	}

	if node == nil {
		return nil, newNotFoundError(ErrStepNotFound)
	}

	var decoder *encoding.Decoder
	if strings.ToLower(h.logEncodingCharset) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}

	logContent, err := readFileContent(node.Log, decoder)
	if err != nil {
		return nil, newInternalError(err)
	}

	stepLog := &models.DagStepLogResponse{
		LogFile: swag.String(node.Log),
		Step:    convertToNode(node),
		Content: swag.String(string(logContent)),
	}

	resp.StepLog = stepLog
	return resp, nil
}

func (h *Handler) processSpecRequest(
	dagID string,
	resp *models.GetDagDetailsResponse,
) (*models.GetDagDetailsResponse, *codedError) {
	dagContent, err := h.engine.GetDAGSpec(dagID)
	if err != nil {
		return nil, newNotFoundError(err)
	}
	resp.Definition = swag.String(dagContent)
	return resp, nil
}

var (
	defaultHistoryLimit = 30
)

func (h *Handler) processLogRequest(
	resp *models.GetDagDetailsResponse,
	dg *dag.DAG,
) (*models.GetDagDetailsResponse, *codedError) {
	logs := h.engine.GetRecentHistory(dg, defaultHistoryLimit)

	nodeNameToStatusList := map[string][]scheduler.NodeStatus{}
	for idx, log := range logs {
		for _, node := range log.Status.Nodes {
			addNodeStatus(nodeNameToStatusList, len(logs), idx, node.Name, node.Status)
		}
	}

	var grid []*models.DagLogGridItem
	for node, statusList := range nodeNameToStatusList {
		var values []int64
		for _, status := range statusList {
			values = append(values, int64(status))
		}
		grid = append(grid, &models.DagLogGridItem{
			Name: swag.String(node),
			Vals: values,
		})
	}

	sort.Slice(grid, func(i, c int) bool {
		a := *grid[i].Name
		b := *grid[c].Name
		return strings.Compare(a, b) <= 0
	})

	handlerToStatusList := map[string][]scheduler.NodeStatus{}
	for idx, log := range logs {
		if n := log.Status.OnSuccess; n != nil {
			addNodeStatus(
				handlerToStatusList, len(logs), idx, n.Name, n.Status,
			)
		}
		if n := log.Status.OnFailure; n != nil {
			addNodeStatus(
				handlerToStatusList, len(logs), idx, n.Name, n.Status,
			)
		}
		if n := log.Status.OnCancel; n != nil {
			n := log.Status.OnCancel
			addNodeStatus(
				handlerToStatusList, len(logs), idx, n.Name, n.Status,
			)
		}
		if n := log.Status.OnExit; n != nil {
			addNodeStatus(
				handlerToStatusList, len(logs), idx, n.Name, n.Status,
			)
		}
	}

	for _, handlerType := range []dag.HandlerType{
		dag.HandlerOnSuccess,
		dag.HandlerOnFailure,
		dag.HandlerOnCancel,
		dag.HandlerOnExit,
	} {
		if statusList, ok := handlerToStatusList[handlerType.String()]; ok {
			var values []int64
			for _, status := range statusList {
				values = append(values, int64(status))
			}
			grid = append(grid, &models.DagLogGridItem{
				Name: swag.String(handlerType.String()),
				Vals: values,
			})
		}
	}

	var logFileStatusList []*models.DagStatusFile
	for _, log := range logs {
		logFileStatusList = append(logFileStatusList, &models.DagStatusFile{
			File:   swag.String(log.File),
			Status: convertToStatusDetail(log.Status),
		})
	}

	resp.LogData = &models.DagLogResponse{
		Logs:     lo.Reverse(logFileStatusList),
		GridData: grid,
	}

	return resp, nil
}

func addNodeStatus(
	data map[string][]scheduler.NodeStatus,
	logLen int,
	logIdx int,
	nodeName string,
	status scheduler.NodeStatus,
) {
	if _, ok := data[nodeName]; !ok {
		data[nodeName] = make([]scheduler.NodeStatus, logLen)
	}
	data[nodeName][logIdx] = status
}

// nolint // cognitive complexity
func (h *Handler) postAction(
	params operations.PostDagActionParams,
) (*models.PostDagActionResponse, *codedError) {
	if params.Body.Action == nil {
		return nil, newBadRequestError(errInvalidArgs)
	}

	var dagStatus *persistence.DAGStatus

	if *params.Body.Action != "save" {
		s, err := h.engine.GetStatus(params.DagID)
		if err != nil {
			return nil, newBadRequestError(err)
		}
		dagStatus = s
	}

	switch *params.Body.Action {
	case "start":
		if dagStatus.Status.Status == scheduler.StatusRunning {
			return nil, newBadRequestError(errInvalidArgs)
		}
		h.engine.StartAsync(dagStatus.DAG, params.Body.Params)
		return &models.PostDagActionResponse{}, nil

	case "suspend":
		_ = h.engine.ToggleSuspend(params.DagID, params.Body.Value == "true")
		return &models.PostDagActionResponse{}, nil

	case "stop":
		if dagStatus.Status.Status != scheduler.StatusRunning {
			return nil, newBadRequestError(
				fmt.Errorf("the DAG is not running: %w", errInvalidArgs),
			)
		}
		if err := h.engine.Stop(dagStatus.DAG); err != nil {
			return nil, newBadRequestError(
				fmt.Errorf("error trying to stop the DAG: %w", err),
			)
		}
		return &models.PostDagActionResponse{}, nil

	case "retry":
		if params.Body.RequestID == "" {
			return nil, newBadRequestError(
				fmt.Errorf("request-id is required: %w", errInvalidArgs),
			)
		}
		if err := h.engine.Retry(dagStatus.DAG, params.Body.RequestID); err != nil {
			return nil, newInternalError(
				fmt.Errorf("error trying to retry the DAG: %w", err),
			)
		}
		return &models.PostDagActionResponse{}, nil

	case "mark-success":
		return h.processUpdateStatus(
			params, dagStatus, scheduler.NodeStatusSuccess,
		)

	case "mark-failed":
		return h.processUpdateStatus(
			params, dagStatus, scheduler.NodeStatusError,
		)

	case "save":
		if err := h.engine.UpdateDAG(params.DagID, params.Body.Value); err != nil {
			return nil, newInternalError(err)
		}
		return &models.PostDagActionResponse{}, nil

	case "rename":
		newName := params.Body.Value
		if newName == "" {
			return nil, newBadRequestError(
				fmt.Errorf("new name is required: %w", errInvalidArgs),
			)
		}
		if err := h.engine.Rename(params.DagID, newName); err != nil {
			return nil, newInternalError(err)
		}
		return &models.PostDagActionResponse{NewDagID: params.Body.Value}, nil

	default:
		return nil, newBadRequestError(
			fmt.Errorf("invalid action: %s", *params.Body.Action),
		)
	}
}

func (h *Handler) processUpdateStatus(
	params operations.PostDagActionParams,
	dagStatus *persistence.DAGStatus, to scheduler.NodeStatus,
) (*models.PostDagActionResponse, *codedError) {
	if params.Body.RequestID == "" {
		return nil, newBadRequestError(fmt.Errorf("request-id is required: %w", errInvalidArgs))
	}

	if params.Body.Step == "" {
		return nil, newBadRequestError(fmt.Errorf("step name is required: %w", errInvalidArgs))
	}

	// Do not allow updating the status if the DAG is still running.
	if dagStatus.Status.Status == scheduler.StatusRunning {
		return nil, newBadRequestError(
			fmt.Errorf("the DAG is still running: %w", errInvalidArgs),
		)
	}

	status, err := h.engine.GetStatusByRequestID(dagStatus.DAG, params.Body.RequestID)
	if err != nil {
		return nil, newInternalError(err)
	}

	var (
		idxToUpdate int
		ok          bool
	)

	for idx, n := range status.Nodes {
		if n.Step.Name == params.Body.Step {
			idxToUpdate = idx
			ok = true
		}
	}
	if !ok {
		return nil, newBadRequestError(fmt.Errorf("step not found: %w", errInvalidArgs))
	}

	status.Nodes[idxToUpdate].Status = to
	status.Nodes[idxToUpdate].StatusText = to.String()

	if err := h.engine.UpdateStatus(dagStatus.DAG, status); err != nil {
		return nil, newInternalError(err)
	}

	return &models.PostDagActionResponse{}, nil
}

func (h *Handler) searchDAGs(
	params operations.SearchDagsParams,
) (*models.SearchDagsResponse, *codedError) {
	query := params.Q
	if query == "" {
		return nil, newBadRequestError(errInvalidArgs)
	}

	ret, errs, err := h.engine.Grep(query)
	if err != nil {
		return nil, newInternalError(err)
	}

	var results []*models.SearchDagsResultItem
	for _, item := range ret {
		var matches []*models.SearchDagsMatchItem
		for _, match := range item.Matches {
			matches = append(matches, &models.SearchDagsMatchItem{
				Line:       match.Line,
				LineNumber: int64(match.LineNumber),
				StartLine:  int64(match.StartLine),
			})
		}

		results = append(results, &models.SearchDagsResultItem{
			Name:    item.Name,
			DAG:     convertToDAG(item.DAG),
			Matches: matches,
		})
	}

	return &models.SearchDagsResponse{
		Results: results,
		Errors:  errs,
	}, nil
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
