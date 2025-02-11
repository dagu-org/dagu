package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/frontend/gen/models"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/frontend/server"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/go-openapi/runtime"
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

var _ server.Handler = (*DAG)(nil)

// DAG is a handler for the DAG API.
type DAG struct {
	client             client.Client
	logEncodingCharset string
	remoteNodes        map[string]config.RemoteNode
	apiBasePath        string
}

func NewDAG(
	cli client.Client,
	logEncodingCharset string,
	remoteNodeConfigs []config.RemoteNode,
	apiBasePath string,
) server.Handler {
	remoteNodes := make(map[string]config.RemoteNode)
	for _, node := range remoteNodeConfigs {
		remoteNodes[node.Name] = node
	}
	return &DAG{
		client:             cli,
		logEncodingCharset: logEncodingCharset,
		remoteNodes:        remoteNodes,
		apiBasePath:        apiBasePath,
	}
}

func (h *DAG) Configure(api *operations.DaguAPI) {
	api.DagsListDagsHandler = dags.ListDagsHandlerFunc(
		func(params dags.ListDagsParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(nil, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			resp, err := h.getList(ctx, params)
			if err != nil {
				return dags.NewListDagsDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewListDagsOK().WithPayload(resp)
		})

	api.DagsGetDagDetailsHandler = dags.GetDagDetailsHandlerFunc(
		func(params dags.GetDagDetailsParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(nil, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			resp, err := h.getDetail(ctx, params)
			if err != nil {
				return dags.NewGetDagDetailsDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewGetDagDetailsOK().WithPayload(resp)
		})

	api.DagsPostDagActionHandler = dags.PostDagActionHandlerFunc(
		func(params dags.PostDagActionParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(params.Body, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			resp, err := h.postAction(ctx, params)
			if err != nil {
				return dags.NewPostDagActionDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewPostDagActionOK().WithPayload(resp)
		})

	api.DagsCreateDagHandler = dags.CreateDagHandlerFunc(
		func(params dags.CreateDagParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(params.Body, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			resp, err := h.createDAG(ctx, params)
			if err != nil {
				return dags.NewCreateDagDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewCreateDagOK().WithPayload(resp)
		})

	api.DagsDeleteDagHandler = dags.DeleteDagHandlerFunc(
		func(params dags.DeleteDagParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(nil, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			err := h.deleteDAG(ctx, params)
			if err != nil {
				return dags.NewDeleteDagDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewDeleteDagOK()
		})

	api.DagsSearchDagsHandler = dags.SearchDagsHandlerFunc(
		func(params dags.SearchDagsParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(nil, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			resp, err := h.searchDAGs(ctx, params)
			if err != nil {
				return dags.NewSearchDagsDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewSearchDagsOK().WithPayload(resp)
		})

	api.DagsListTagsHandler = dags.ListTagsHandlerFunc(
		func(params dags.ListTagsParams) middleware.Responder {
			if resp := h.handleRemoteNodeProxy(nil, params.HTTPRequest); resp != nil {
				return resp
			}
			ctx := params.HTTPRequest.Context()
			tags, err := h.getTagList(ctx, params)
			if err != nil {
				return dags.NewListTagsDefault(err.HTTPCode).
					WithPayload(err.APIError)
			}
			return dags.NewListTagsOK().WithPayload(tags)
		})
}

// handleRemoteNodeProxy checks if 'remoteNode' is present in the query parameters.
// If yes, it proxies the request to the remote node and returns the remote response.
// If not, it returns nil, indicating to proceed locally.
func (h *DAG) handleRemoteNodeProxy(body any, r *http.Request) middleware.Responder {
	if r == nil {
		return nil
	}

	remoteNodeName := r.URL.Query().Get("remoteNode")
	if remoteNodeName == "" || remoteNodeName == "local" {
		return nil // No remote node specified, handle locally
	}

	node, ok := h.remoteNodes[remoteNodeName]
	if !ok {
		// remote node not found, return bad request
		return dags.NewListDagsDefault(400)
	}

	// forward the request to the remote node
	return h.doRemoteProxy(body, r, node)
}

// doRemoteProxy performs the actual proxying of the request to the remote node.
func (h *DAG) doRemoteProxy(body any, originalReq *http.Request, node config.RemoteNode) middleware.Responder {
	// Copy original query parameters except remoteNode
	q := originalReq.URL.Query()
	q.Del("remoteNode")

	// Build the new remote URL
	urlComponents := strings.Split(originalReq.URL.Path, h.apiBasePath)
	if len(urlComponents) < 2 {
		return h.responderWithCodedError(newBadRequestError(fmt.Errorf("invalid API path")))
	}
	remoteURL := fmt.Sprintf("%s%s?%s", strings.TrimSuffix(node.APIBaseURL, "/"), urlComponents[1], q.Encode())

	method := originalReq.Method
	var bodyJSON io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return h.responderWithCodedError(newError(
				http.StatusInternalServerError,
				models.ErrorCodeInternalError,
				swag.String(fmt.Sprintf("failed to marshal request body: %v", err)),
			))
		}
		bodyJSON = strings.NewReader(string(data))
	}

	req, err := http.NewRequest(method, remoteURL, bodyJSON)
	if err != nil {
		return h.responderWithCodedError(newError(
			http.StatusInternalServerError,
			models.ErrorCodeInternalError,
			swag.String(fmt.Sprintf("failed to create request to remote node: %v", err)),
		))
	}

	// Copy headers from the original request if needed
	// But we need to override authorization headers
	if node.IsBasicAuth {
		req.SetBasicAuth(node.BasicAuthUsername, node.BasicAuthPassword)
	} else if node.IsAuthToken {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", node.AuthToken))
	}
	for k, v := range originalReq.Header {
		if k == "Authorization" {
			continue
		}
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	// Create a custom transport that skips certificate verification
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			// Allow insecure TLS connections if the remote node is configured to skip verification
			// This may be necessary for some enterprise setups
			InsecureSkipVerify: node.SkipTLSVerify, // nolint:gosec
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Add a reasonable timeout
	}

	resp, err := client.Do(req)
	if err != nil {
		return h.responderWithCodedError(newError(
			http.StatusBadGateway,
			models.ErrorCodeBadGateway,
			swag.String(fmt.Sprintf("failed to send request to remote node: %v", err)),
		))
	}

	if resp == nil {
		return h.responderWithCodedError(newError(
			http.StatusBadGateway,
			models.ErrorCodeBadGateway,
			swag.String("received nil response from remote node"),
		))
	}

	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return h.responderWithCodedError(newError(
			http.StatusBadGateway,
			models.ErrorCodeBadGateway,
			swag.String(fmt.Sprintf("failed to read response from remote node: %v", err)),
		))
	}

	// If not status 200, try to parse the error response
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Only try to decode JSON if we actually got some response data
		if len(respData) > 0 {
			var remoteErr models.Error
			if err := json.Unmarshal(respData, &remoteErr); err == nil && remoteErr.Message != nil {
				return h.responderWithCodedError(newError(
					resp.StatusCode, fromPtr(remoteErr.Code),
					remoteErr.Message,
				))
			}
		}
		// If we can't decode a proper error or have no data, return a generic one
		payload := &models.Error{
			Message: swag.String(fmt.Sprintf("remote node responded with status %d", resp.StatusCode)),
		}
		return h.responderWithCodedError(newError(
			resp.StatusCode,
			models.ErrorCodeBadGateway,
			payload.Message,
		))
	}

	return middleware.ResponderFunc(func(w http.ResponseWriter, _ runtime.Producer) {
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respData)
	})
}

func (h *DAG) responderWithCodedError(err *codedError) middleware.Responder {
	return dags.NewListDagsDefault(err.HTTPCode).WithPayload(err.APIError)
}

func (h *DAG) createDAG(ctx context.Context, params dags.CreateDagParams) (
	*models.CreateDagResponse, *codedError,
) {
	if params.Body.Action == nil || params.Body.Value == nil {
		return nil, newBadRequestError(fmt.Errorf("missing required parameters: action and value"))
	}

	switch *params.Body.Action {
	case "new":
		name := *params.Body.Value
		id, err := h.client.CreateDAG(ctx, name)
		if err != nil {
			return nil, newInternalError(err)
		}
		return &models.CreateDagResponse{DagID: swag.String(id)}, nil
	default:
		return nil, newBadRequestError(fmt.Errorf("invalid action: %s", *params.Body.Action))
	}
}
func (h *DAG) deleteDAG(ctx context.Context, params dags.DeleteDagParams) *codedError {
	dagStatus, err := h.client.GetStatus(ctx, params.DagID)
	if err != nil {
		return newNotFoundError(err)
	}
	if err := h.client.DeleteDAG(ctx, params.DagID, dagStatus.DAG.Location); err != nil {
		return newInternalError(err)
	}
	return nil
}

func (h *DAG) getList(ctx context.Context, params dags.ListDagsParams) (*models.ListDagsResponse, *codedError) {
	dgs, result, err := h.client.GetAllStatusPagination(ctx, params)
	if err != nil {
		return nil, newInternalError(err)
	}

	hasErr := len(result.ErrorList) > 0
	if !hasErr {
		// Check if any DAG has an error
		for _, d := range dgs {
			if d.Error != nil {
				hasErr = true
				break
			}
		}
	}

	resp := &models.ListDagsResponse{
		Errors:    result.ErrorList,
		PageCount: swag.Int64(int64(result.PageCount)),
		HasError:  swag.Bool(hasErr),
	}

	for _, dagStatus := range dgs {
		s := dagStatus.Status

		status := &models.DAGStatus{
			Log:        swag.String(s.Log),
			Name:       swag.String(s.Name),
			Params:     swag.String(s.Params),
			Pid:        swag.Int64(int64(s.PID)),
			RequestID:  swag.String(s.RequestID),
			StartedAt:  swag.String(s.StartedAt),
			FinishedAt: swag.String(s.FinishedAt),
			Status:     swag.Int64(int64(s.Status)),
			StatusText: swag.String(s.StatusText),
		}

		item := &models.DAGStatusFile{
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

func (h *DAG) getDetail(
	ctx context.Context, params dags.GetDagDetailsParams,
) (*models.GetDagDetailsResponse, *codedError) {
	dagID := params.DagID

	tab := dagTabTypeStatus
	if params.Tab != nil {
		tab = *params.Tab
	}

	dagStatus, err := h.client.GetStatus(ctx, dagID)

	var steps []*models.Step
	for _, step := range dagStatus.DAG.Steps {
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
	for _, s := range dagStatus.DAG.Schedule {
		schedules = append(schedules, &models.Schedule{
			Expression: swag.String(s.Expression),
		})
	}

	var preconditions []*models.Condition
	for _, p := range dagStatus.DAG.Preconditions {
		preconditions = append(preconditions, &models.Condition{
			Condition: p.Condition,
			Expected:  p.Expected,
		})
	}

	dag := dagStatus.DAG
	dagDetail := &models.DAGDetails{
		DefaultParams:     swag.String(dag.DefaultParams),
		Delay:             swag.Int64(int64(dag.Delay)),
		Description:       swag.String(dag.Description),
		Env:               dag.Env,
		Group:             swag.String(dag.Group),
		HandlerOn:         handlerOn,
		HistRetentionDays: swag.Int64(int64(dag.HistRetentionDays)),
		Location:          swag.String(dag.Location),
		LogDir:            swag.String(dag.LogDir),
		MaxActiveRuns:     swag.Int64(int64(dag.MaxActiveRuns)),
		Name:              swag.String(dag.Name),
		Params:            dag.Params,
		Preconditions:     preconditions,
		Schedule:          schedules,
		Steps:             steps,
		Tags:              dag.Tags,
	}

	statusWithDetails := &models.DAGStatusFileDetails{
		DAG:       dagDetail,
		Dir:       swag.String(dagStatus.Dir),
		ErrorT:    dagStatus.ErrorT,
		File:      swag.String(dagStatus.File),
		Status:    convertToStatusDetails(dagStatus.Status),
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
		return h.processSpecRequest(ctx, dagID, resp)

	case dagTabTypeHistory:
		return h.processLogRequest(ctx, resp, dag)

	case dagTabTypeStepLog:
		return h.processStepLogRequest(ctx, dag, params, resp)

	case dagTabTypeSchedulerLog:
		return h.processSchedulerLogRequest(ctx, dag, params, resp)

	default:
		return nil, newBadRequestError(fmt.Errorf("invalid tab type: %s", tab))
	}
}

func (h *DAG) processSchedulerLogRequest(
	ctx context.Context,
	dag *digraph.DAG,
	params dags.GetDagDetailsParams,
	resp *models.GetDagDetailsResponse,
) (*models.GetDagDetailsResponse, *codedError) {
	var logFile string

	if params.File != nil {
		status, err := jsondb.ParseStatusFile(*params.File)
		if err != nil {
			return nil, newBadRequestError(err)
		}
		logFile = status.Log
	}

	if logFile == "" {
		lastStatus, err := h.client.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, newInternalError(err)
		}
		logFile = lastStatus.Log
	}

	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, newInternalError(err)
	}

	resp.ScLog = &models.SchedulerLog{
		LogFile: swag.String(logFile),
		Content: swag.String(string(content)),
	}

	return resp, nil
}

func (h *DAG) processStepLogRequest(
	ctx context.Context,
	dag *digraph.DAG,
	params dags.GetDagDetailsParams,
	resp *models.GetDagDetailsResponse,
) (*models.GetDagDetailsResponse, *codedError) {
	var status *model.Status

	if params.Step == nil {
		return nil, newBadRequestError(fmt.Errorf("missing required parameter: step"))
	}

	if params.File != nil {
		parsedStatus, err := jsondb.ParseStatusFile(*params.File)
		if err != nil {
			return nil, newBadRequestError(err)
		}
		status = parsedStatus
	}

	if status == nil {
		latestStatus, err := h.client.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, newInternalError(err)
		}
		status = &latestStatus
	}

	// Find the step in the status to get the log file.
	var node *model.Node

	for _, n := range status.Nodes {
		if n.Step.Name == *params.Step {
			node = n
		}
	}

	if node == nil {
		if status.OnSuccess != nil && status.OnSuccess.Step.Name == *params.Step {
			node = status.OnSuccess
		}
		if status.OnFailure != nil && status.OnFailure.Step.Name == *params.Step {
			node = status.OnFailure
		}
		if status.OnCancel != nil && status.OnCancel.Step.Name == *params.Step {
			node = status.OnCancel
		}
		if status.OnExit != nil && status.OnExit.Step.Name == *params.Step {
			node = status.OnExit
		}
	}

	if node == nil {
		return nil, newNotFoundError(fmt.Errorf("step not found: %s", *params.Step))
	}

	var decoder *encoding.Decoder
	if strings.ToLower(h.logEncodingCharset) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}

	logContent, err := readFileContent(node.Log, decoder)
	if err != nil {
		return nil, newInternalError(err)
	}

	stepLog := &models.StepLog{
		LogFile: swag.String(node.Log),
		Step:    convertToNode(node),
		Content: swag.String(string(logContent)),
	}

	resp.StepLog = stepLog
	return resp, nil
}

func (h *DAG) processSpecRequest(
	ctx context.Context,
	dagID string,
	resp *models.GetDagDetailsResponse,
) (*models.GetDagDetailsResponse, *codedError) {
	dagContent, err := h.client.GetDAGSpec(ctx, dagID)
	if err != nil {
		return nil, newNotFoundError(err)
	}
	resp.Definition = swag.String(dagContent)
	return resp, nil
}

var (
	defaultHistoryLimit = 30
)

func (h *DAG) processLogRequest(
	ctx context.Context,
	resp *models.GetDagDetailsResponse,
	dag *digraph.DAG,
) (*models.GetDagDetailsResponse, *codedError) {
	logs := h.client.GetRecentHistory(ctx, dag, defaultHistoryLimit)

	nodeNameToStatusList := map[string][]scheduler.NodeStatus{}
	for idx, log := range logs {
		for _, node := range log.Status.Nodes {
			addNodeStatus(ctx, nodeNameToStatusList, len(logs), idx, node.Step.Name, node.Status)
		}
	}

	var grid []*models.DAGLogGridItem
	for node, statusList := range nodeNameToStatusList {
		var values []int64
		for _, status := range statusList {
			values = append(values, int64(status))
		}
		grid = append(grid, &models.DAGLogGridItem{
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
			addNodeStatus(ctx, handlerToStatusList, len(logs), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnFailure; n != nil {
			addNodeStatus(ctx, handlerToStatusList, len(logs), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnCancel; n != nil {
			n := log.Status.OnCancel
			addNodeStatus(ctx, handlerToStatusList, len(logs), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnExit; n != nil {
			addNodeStatus(ctx, handlerToStatusList, len(logs), idx, n.Step.Name, n.Status)
		}
	}

	for _, handlerType := range []digraph.HandlerType{
		digraph.HandlerOnSuccess,
		digraph.HandlerOnFailure,
		digraph.HandlerOnCancel,
		digraph.HandlerOnExit,
	} {
		if statusList, ok := handlerToStatusList[handlerType.String()]; ok {
			var values []int64
			for _, status := range statusList {
				values = append(values, int64(status))
			}
			grid = append(grid, &models.DAGLogGridItem{
				Name: swag.String(handlerType.String()),
				Vals: values,
			})
		}
	}

	var logStatusFiles []*models.DAGLogStatusFile
	for _, log := range logs {
		logStatusFiles = append(logStatusFiles, &models.DAGLogStatusFile{
			File:   swag.String(log.File),
			Status: convertToStatusDetails(log.Status),
		})
	}

	resp.LogData = &models.DAGLogData{
		Logs:     lo.Reverse(logStatusFiles),
		GridData: grid,
	}

	return resp, nil
}

func addNodeStatus(
	_ context.Context,
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

func (h *DAG) postAction(
	ctx context.Context,
	params dags.PostDagActionParams,
) (*models.PostDagActionResponse, *codedError) {
	if params.Body.Action == nil {
		return nil, newBadRequestError(fmt.Errorf("missing required parameter: action"))
	}

	var dagStatus client.DAGStatus

	if *params.Body.Action != "save" {
		s, err := h.client.GetStatus(ctx, params.DagID)
		if err != nil {
			return nil, newBadRequestError(err)
		}
		dagStatus = s
	}

	switch *params.Body.Action {
	case "start":
		if dagStatus.Status.Status == scheduler.StatusRunning {
			return nil, newBadRequestError(fmt.Errorf("the DAG %q is already running", params.DagID))
		}
		h.client.StartAsync(ctx, dagStatus.DAG, client.StartOptions{
			Params: params.Body.Params,
		})
		return &models.PostDagActionResponse{}, nil

	case "suspend":
		_ = h.client.ToggleSuspend(ctx, params.DagID, params.Body.Value == "true")
		return &models.PostDagActionResponse{}, nil

	case "stop":
		if dagStatus.Status.Status != scheduler.StatusRunning {
			return nil, newBadRequestError(
				fmt.Errorf("the DAG %q is not running", params.DagID),
			)
		}
		if err := h.client.Stop(ctx, dagStatus.DAG); err != nil {
			return nil, newBadRequestError(
				fmt.Errorf("error trying to stop the DAG: %w", err),
			)
		}
		return &models.PostDagActionResponse{}, nil

	case "retry":
		if params.Body.RequestID == "" {
			return nil, newBadRequestError(
				fmt.Errorf("request-id is required"),
			)
		}
		if err := h.client.Retry(ctx, dagStatus.DAG, params.Body.RequestID); err != nil {
			return nil, newInternalError(
				fmt.Errorf("error trying to retry the DAG: %w", err),
			)
		}
		return &models.PostDagActionResponse{}, nil

	case "mark-success":
		return h.processUpdateStatus(ctx, params, dagStatus, scheduler.NodeStatusSuccess)

	case "mark-failed":
		return h.processUpdateStatus(ctx, params, dagStatus, scheduler.NodeStatusError)

	case "save":
		if err := h.client.UpdateDAG(ctx, params.DagID, params.Body.Value); err != nil {
			return nil, newInternalError(err)
		}
		return &models.PostDagActionResponse{}, nil

	case "rename":
		newName := params.Body.Value
		if newName == "" {
			return nil, newBadRequestError(
				fmt.Errorf("the value (new name) is required"),
			)
		}
		if err := h.client.Rename(ctx, params.DagID, newName); err != nil {
			return nil, newInternalError(err)
		}
		return &models.PostDagActionResponse{NewDagID: params.Body.Value}, nil

	default:
		return nil, newBadRequestError(
			fmt.Errorf("invalid action: %s", *params.Body.Action),
		)
	}
}

func (h *DAG) processUpdateStatus(
	ctx context.Context,
	params dags.PostDagActionParams,
	dagStatus client.DAGStatus, to scheduler.NodeStatus,
) (*models.PostDagActionResponse, *codedError) {
	if params.Body.RequestID == "" {
		return nil, newBadRequestError(fmt.Errorf("request-id is required"))
	}

	if params.Body.Step == "" {
		return nil, newBadRequestError(fmt.Errorf("step name is required"))
	}

	// Do not allow updating the status if the DAG is still running.
	if dagStatus.Status.Status == scheduler.StatusRunning {
		return nil, newBadRequestError(
			fmt.Errorf("the DAG %q is still running", dagStatus.DAG.Name),
		)
	}

	status, err := h.client.GetStatusByRequestID(ctx, dagStatus.DAG, params.Body.RequestID)
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
		return nil, newBadRequestError(fmt.Errorf("step %q not found", params.Body.Step))
	}

	status.Nodes[idxToUpdate].Status = to
	status.Nodes[idxToUpdate].StatusText = to.String()

	if err := h.client.UpdateStatus(ctx, dagStatus.DAG, *status); err != nil {
		return nil, newInternalError(err)
	}

	return &models.PostDagActionResponse{}, nil
}

func (h *DAG) searchDAGs(ctx context.Context, params dags.SearchDagsParams) (
	*models.SearchDAGsResponse, *codedError,
) {
	query := params.Q
	if query == "" {
		return nil, newBadRequestError(fmt.Errorf("missing required parameter: q"))
	}

	ret, errs, err := h.client.Grep(ctx, query)
	if err != nil {
		return nil, newInternalError(err)
	}

	var results []*models.SearchDAGsResultItem
	for _, item := range ret {
		var matches []*models.SearchDAGsMatchItem
		for _, match := range item.Matches {
			matches = append(matches, &models.SearchDAGsMatchItem{
				Line:       match.Line,
				LineNumber: int64(match.LineNumber),
				StartLine:  int64(match.StartLine),
			})
		}

		results = append(results, &models.SearchDAGsResultItem{
			Name:    item.Name,
			DAG:     convertToDAG(item.DAG),
			Matches: matches,
		})
	}

	return &models.SearchDAGsResponse{
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

func (h *DAG) getTagList(ctx context.Context, _ dags.ListTagsParams) (*models.ListTagResponse, *codedError) {
	tags, errs, err := h.client.GetTagList(ctx)
	if err != nil {
		return nil, newInternalError(err)
	}
	return &models.ListTagResponse{
		Errors: errs,
		Tags:   tags,
	}, nil
}

func fromPtr[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}
