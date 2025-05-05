package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/dagstore"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/runstore"
	"github.com/dagu-org/dagu/internal/runstore/filestore"
	"github.com/samber/lo"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	name, err := a.dagClient.Create(ctx, request.Body.Value)
	if err != nil {
		if errors.Is(err, dagstore.ErrDAGAlreadyExists) {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
			}
		}
		return nil, fmt.Errorf("error creating DAG: %w", err)
	}
	return &api.CreateDAG201JSONResponse{
		DagID: name,
	}, nil
}

// DeleteDAG implements api.StrictServerInterface.
func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	_, err := a.dagClient.Status(ctx, request.Name)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.Name),
		}
	}
	if err := a.dagClient.Delete(ctx, request.Name); err != nil {
		return nil, fmt.Errorf("error deleting DAG: %w", err)
	}
	return &api.DeleteDAG204Response{}, nil
}

// GetDAGDetails implements api.StrictServerInterface.
func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	name := request.Name

	tab := api.DAGDetailTabStatus
	if request.Params.Tab != nil {
		tab = *request.Params.Tab
	}

	status, err := a.dagClient.Status(ctx, name)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", name),
		}
	}

	var steps []api.Step
	for _, step := range status.DAG.Steps {
		steps = append(steps, toStep(step))
	}

	handlers := status.DAG.HandlerOn

	handlerOn := api.HandlerOn{}
	if handlers.Failure != nil {
		handlerOn.Failure = ptrOf(toStep(*handlers.Failure))
	}
	if handlers.Success != nil {
		handlerOn.Success = ptrOf(toStep(*handlers.Success))
	}
	if handlers.Cancel != nil {
		handlerOn.Cancel = ptrOf(toStep(*handlers.Cancel))
	}
	if handlers.Exit != nil {
		handlerOn.Exit = ptrOf(toStep(*handlers.Exit))
	}

	var schedules []api.Schedule
	for _, s := range status.DAG.Schedule {
		schedules = append(schedules, api.Schedule{
			Expression: s.Expression,
		})
	}

	var preconditions []api.Precondition
	for _, p := range status.DAG.Preconditions {
		preconditions = append(preconditions, toPrecondition(p))
	}

	dag := status.DAG
	details := api.DAGDetails{
		Name:              dag.Name,
		Description:       ptrOf(dag.Description),
		DefaultParams:     ptrOf(dag.DefaultParams),
		Delay:             ptrOf(int(dag.Delay.Seconds())),
		Env:               ptrOf(dag.Env),
		Group:             ptrOf(dag.Group),
		HandlerOn:         ptrOf(handlerOn),
		HistRetentionDays: ptrOf(dag.HistRetentionDays),
		Location:          ptrOf(dag.Location),
		LogDir:            ptrOf(dag.LogDir),
		MaxActiveRuns:     ptrOf(dag.MaxActiveRuns),
		Params:            ptrOf(dag.Params),
		Preconditions:     ptrOf(preconditions),
		Schedule:          ptrOf(schedules),
		Steps:             ptrOf(steps),
		Tags:              ptrOf(dag.Tags),
	}

	statusDetails := api.DAGStatusFileDetails{
		DAG:       details,
		Error:     ptrOf(status.ErrorAsString()),
		File:      status.File,
		Status:    toStatus(status.Status),
		Suspended: status.Suspended,
	}

	if status.Error != nil {
		statusDetails.Error = ptrOf(status.Error.Error())
	}

	resp := &api.GetDAGDetails200JSONResponse{
		Title: status.DAG.Name,
		DAG:   statusDetails,
	}

	if err := status.DAG.Validate(); err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case api.DAGDetailTabStatus:
		return resp, nil

	case api.DAGDetailTabSpec:
		spec, err := a.dagClient.GetSpec(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("error getting DAG spec: %w", err)
		}
		resp.Definition = ptrOf(spec)

	case api.DAGDetailTabHistory:
		historyData := a.readHistoryData(ctx, status.DAG)
		resp.LogData = &historyData

	case api.DAGDetailTabLog:
		if request.Params.Step == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "Step name is required",
			}
		}

		l, err := a.readStepLog(ctx, dag, *request.Params.Step, valueOf(request.Params.File))
		if err != nil {
			return nil, err
		}
		resp.StepLog = l

	case api.DAGDetailTabSchedulerLog:
		l, err := a.readLog(ctx, dag, valueOf(request.Params.File))
		if err != nil {
			return nil, err
		}
		resp.ScLog = l

	default:
		// Unreachable
	}

	return resp, nil
}

func (a *API) readHistoryData(
	ctx context.Context,
	dag *digraph.DAG,
) api.DAGHistoryData {
	defaultHistoryLimit := 30
	logs := a.runClient.GetRecentHistory(ctx, dag.Name, defaultHistoryLimit)

	data := map[string][]scheduler.NodeStatus{}

	addStatusFn := func(
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

	for idx, log := range logs {
		for _, node := range log.Status.Nodes {
			addStatusFn(data, len(logs), idx, node.Step.Name, node.Status)
		}
	}

	var grid []api.DAGLogGridItem
	for node, statusList := range data {
		var runstore []api.NodeStatus
		for _, s := range statusList {
			runstore = append(runstore, api.NodeStatus(s))
		}
		grid = append(grid, api.DAGLogGridItem{
			Name: node,
			Vals: runstore,
		})
	}

	sort.Slice(grid, func(i, j int) bool {
		return strings.Compare(grid[i].Name, grid[j].Name) <= 0
	})

	handlers := map[string][]scheduler.NodeStatus{}
	for idx, log := range logs {
		if n := log.Status.OnSuccess; n != nil {
			addStatusFn(handlers, len(logs), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnFailure; n != nil {
			addStatusFn(handlers, len(logs), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnCancel; n != nil {
			n := log.Status.OnCancel
			addStatusFn(handlers, len(logs), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnExit; n != nil {
			addStatusFn(handlers, len(logs), idx, n.Step.Name, n.Status)
		}
	}

	for _, handlerType := range []digraph.HandlerType{
		digraph.HandlerOnSuccess,
		digraph.HandlerOnFailure,
		digraph.HandlerOnCancel,
		digraph.HandlerOnExit,
	} {
		if statusList, ok := handlers[handlerType.String()]; ok {
			var runstore []api.NodeStatus
			for _, status := range statusList {
				runstore = append(runstore, api.NodeStatus(status))
			}
			grid = append(grid, api.DAGLogGridItem{
				Name: handlerType.String(),
				Vals: runstore,
			})
		}
	}

	var statusList []api.DAGLogStatusFile
	for _, log := range logs {
		statusFile := api.DAGLogStatusFile{
			File:   log.File,
			Status: toStatus(log.Status),
		}
		statusList = append(statusList, statusFile)
	}

	return api.DAGHistoryData{
		GridData: grid,
		Logs:     lo.Reverse(statusList),
	}
}

func (a *API) readLog(
	ctx context.Context,
	dag *digraph.DAG,
	statusFile string,
) (*api.SchedulerLog, error) {
	var logFile string

	if statusFile != "" {
		status, err := filestore.ParseStatusFile(statusFile)
		if err != nil {
			return nil, err
		}
		logFile = status.Log
	}

	if logFile == "" {
		lastStatus, err := a.runClient.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		logFile = lastStatus.Log
	}

	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading log file %s: %w", logFile, err)
	}

	return &api.SchedulerLog{
		LogFile: logFile,
		Content: string(content),
	}, nil
}

func (a *API) readStepLog(
	ctx context.Context,
	dag *digraph.DAG,
	stepName string,
	statusFile string,
) (*api.StepLog, error) {
	var status *runstore.Status

	if statusFile != "" {
		parsedStatus, err := filestore.ParseStatusFile(statusFile)
		if err != nil {
			return nil, err
		}
		status = parsedStatus
	}

	if status == nil {
		latestStatus, err := a.runClient.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		status = &latestStatus
	}

	// Find the step in the status to get the log file.
	var node *runstore.Node
	for _, n := range status.Nodes {
		if n.Step.Name == stepName {
			node = n
		}
	}

	if node == nil {
		if status.OnSuccess != nil && status.OnSuccess.Step.Name == stepName {
			node = status.OnSuccess
		}
		if status.OnFailure != nil && status.OnFailure.Step.Name == stepName {
			node = status.OnFailure
		}
		if status.OnCancel != nil && status.OnCancel.Step.Name == stepName {
			node = status.OnCancel
		}
		if status.OnExit != nil && status.OnExit.Step.Name == stepName {
			node = status.OnExit
		}
	}

	if node == nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("step %s not found", stepName),
		}
	}

	var decoder *encoding.Decoder
	if strings.ToLower(a.logEncodingCharset) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}

	logContent, err := readFileContent(node.Log, decoder)
	if err != nil {
		return nil, fmt.Errorf("error reading log file %s: %w", node.Log, err)
	}

	return &api.StepLog{
		LogFile: node.Log,
		Step:    toNode(node),
		Content: string(logContent),
	}, nil
}

func readFileContent(f string, decoder *encoding.Decoder) ([]byte, error) {
	if decoder == nil {
		return os.ReadFile(f) //nolint:gosec
	}

	r, err := os.Open(f) //nolint:gosec
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

// ListDAGs implements api.StrictServerInterface.
func (a *API) ListDAGs(ctx context.Context, request api.ListDAGsRequestObject) (api.ListDAGsResponseObject, error) {
	var opts []dagstore.ListDAGOption
	if request.Params.Limit != nil {
		opts = append(opts, dagstore.WithLimit(*request.Params.Limit))
	}
	if request.Params.Page != nil {
		opts = append(opts, dagstore.WithPage(*request.Params.Page))
	}
	if request.Params.SearchName != nil {
		opts = append(opts, dagstore.WithName(*request.Params.SearchName))
	}
	if request.Params.SearchTag != nil {
		opts = append(opts, dagstore.WithTag(*request.Params.SearchTag))
	}

	result, errList, err := a.dagClient.List(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	hasErr := len(errList) > 0
	for _, item := range result.Items {
		if item.Error != nil {
			hasErr = true
			break
		}
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:    ptrOf(errList),
		PageCount: result.TotalPages,
		HasError:  hasErr,
	}

	for _, item := range result.Items {
		status := api.DAGStatus{
			Log:        ptrOf(item.Status.Log),
			Name:       item.Status.Name,
			Params:     ptrOf(item.Status.Params),
			Pid:        ptrOf(int(item.Status.PID)),
			RequestId:  item.Status.RequestID,
			StartedAt:  item.Status.StartedAt,
			FinishedAt: item.Status.FinishedAt,
			Status:     api.RunStatus(item.Status.Status),
			StatusText: api.RunStatusText(item.Status.Status.String()),
		}

		dag := api.DAGStatusFile{
			Error:     ptrOf(item.ErrorAsString()),
			File:      item.File,
			Status:    status,
			Suspended: item.Suspended,
			DAG:       toDAG(item.DAG),
		}

		if item.Error != nil {
			dag.Error = ptrOf(item.Error.Error())
		}

		resp.DAGs = append(resp.DAGs, dag)
	}

	return resp, nil
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	tags, errs, err := a.dagClient.TagList(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting tags: %w", err)
	}
	return &api.ListTags200JSONResponse{
		Tags:   tags,
		Errors: errs,
	}, nil
}

// PostDAGAction implements api.StrictServerInterface.
func (a *API) PostDAGAction(ctx context.Context, request api.PostDAGActionRequestObject) (api.PostDAGActionResponseObject, error) {
	action := request.Body.Action

	var status dagstore.Status
	if action != api.DAGActionSave {
		s, err := a.dagClient.Status(ctx, request.Name)
		if err != nil {
			return nil, err
		}
		status = s
	}

	switch request.Body.Action {
	case api.DAGActionStart:
		if status.Status.Status == scheduler.StatusRunning {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeAlreadyRunning,
				Message:    "DAG is already running",
			}
		}
		if err := a.runClient.StartDAG(ctx, status.DAG, runstore.StartOptions{
			Params: valueOf(request.Body.Params),
		}); err != nil {
			return nil, fmt.Errorf("error starting DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionSuspend:
		b, err := strconv.ParseBool(valueOf(request.Body.Value))
		if err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "invalid value for suspend, must be true or false",
			}
		}
		if err := a.dagClient.ToggleSuspend(ctx, request.Name, b); err != nil {
			return nil, fmt.Errorf("error toggling suspend: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionStop:
		if status.Status.Status != scheduler.StatusRunning {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeNotRunning,
				Message:    "DAG is not running",
			}
		}
		if err := a.runClient.StopDAG(ctx, status.DAG, ""); err != nil {
			return nil, fmt.Errorf("error stopping DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionRetry:
		if request.Body.RequestId == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "requestId is required for retry action",
			}
		}
		if err := a.runClient.RetryDAG(ctx, status.DAG, *request.Body.RequestId); err != nil {
			return nil, fmt.Errorf("error retrying DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionMarkSuccess:
		fallthrough

	case api.DAGActionMarkFailed:
		if status.Status.Status == scheduler.StatusRunning {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "cannot change status of running DAG",
			}
		}
		if request.Body.RequestId == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "requestId is required for mark-success action",
			}
		}
		if request.Body.Step == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "step is required for mark-success action",
			}
		}
		toStatus := scheduler.NodeStatusSuccess
		if action == api.DAGActionMarkFailed {
			toStatus = scheduler.NodeStatusError
		}

		if err := a.updateStatus(ctx, *request.Body.RequestId, *request.Body.Step, status, toStatus); err != nil {
			return nil, fmt.Errorf("error marking DAG as success: %w", err)
		}

		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionSave:
		if request.Body.Value == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "value is required for save action (DAG spec)",
			}
		}

		if err := a.dagClient.Update(ctx, request.Name, *request.Body.Value); err != nil {
			return nil, err
		}

		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionRename:
		if request.Body.Value == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "value is required for rename action (new name)",
			}
		}

		newName := *request.Body.Value
		if err := a.dagClient.Move(ctx, request.Name, newName); err != nil {
			return nil, fmt.Errorf("error renaming DAG: %w", err)
		}

		return api.PostDAGAction200JSONResponse{
			NewName: ptrOf(newName),
		}, nil

	default:
		// Unreachable
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (a *API) updateStatus(
	ctx context.Context,
	reqID string,
	step string,
	dagStatus dagstore.Status,
	to scheduler.NodeStatus,
) error {
	status, err := a.runClient.GetStatusByRequestID(ctx, dagStatus.DAG, reqID)
	if err != nil {
		return fmt.Errorf("error getting status: %w", err)
	}

	idxToUpdate := -1

	for idx, n := range status.Nodes {
		if n.Step.Name == step {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("step %s not found", step),
		}
	}

	status.Nodes[idxToUpdate].Status = to

	if err := a.runClient.UpdateStatus(ctx, dagStatus.DAG.Name, *status); err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	return nil
}

// SearchDAGs implements api.StrictServerInterface.
func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	query := request.Params.Q
	if query == "" {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "query is required",
		}
	}

	ret, errs, err := a.dagClient.Grep(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error searching DAGs: %w", err)
	}

	var results []api.SearchDAGsResultItem
	for _, item := range ret {
		var matches []api.SearchDAGsMatchItem
		for _, match := range item.Matches {
			matches = append(matches, api.SearchDAGsMatchItem{
				Line:       match.Line,
				LineNumber: match.LineNumber,
				StartLine:  match.StartLine,
			})
		}

		results = append(results, api.SearchDAGsResultItem{
			Name:    item.Name,
			DAG:     toDAG(item.DAG),
			Matches: matches,
		})
	}

	return &api.SearchDAGs200JSONResponse{
		Results: results,
		Errors:  errs,
	}, nil
}

func toDAG(dag *digraph.DAG) api.DAG {
	var schedules []api.Schedule
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{Expression: s.Expression})
	}

	return api.DAG{
		Name:          dag.Name,
		Group:         ptrOf(dag.Group),
		Description:   ptrOf(dag.Description),
		Params:        ptrOf(dag.Params),
		DefaultParams: ptrOf(dag.DefaultParams),
		Tags:          ptrOf(dag.Tags),
		Schedule:      ptrOf(schedules),
	}
}

func toStep(obj digraph.Step) api.Step {
	var conditions []api.Precondition
	for _, cond := range obj.Preconditions {
		conditions = append(conditions, toPrecondition(cond))
	}

	repeatPolicy := api.RepeatPolicy{
		Repeat:   ptrOf(obj.RepeatPolicy.Repeat),
		Interval: ptrOf(int(obj.RepeatPolicy.Interval.Seconds())),
	}

	step := api.Step{
		Name:          obj.Name,
		Description:   ptrOf(obj.Description),
		Args:          ptrOf(obj.Args),
		CmdWithArgs:   ptrOf(obj.CmdWithArgs),
		Command:       ptrOf(obj.Command),
		Depends:       ptrOf(obj.Depends),
		Dir:           ptrOf(obj.Dir),
		MailOnError:   ptrOf(obj.MailOnError),
		Output:        ptrOf(obj.Output),
		Preconditions: ptrOf(conditions),
		RepeatPolicy:  ptrOf(repeatPolicy),
		Script:        ptrOf(obj.Script),
	}

	if obj.SubDAG != nil {
		step.Run = ptrOf(obj.SubDAG.Name)
		step.Params = ptrOf(obj.SubDAG.Params)
	}
	return step
}

func toPrecondition(obj digraph.Condition) api.Precondition {
	return api.Precondition{
		Condition: ptrOf(obj.Condition),
		Expected:  ptrOf(obj.Expected),
	}
}

func toStatus(s runstore.Status) api.DAGStatusDetails {
	status := api.DAGStatusDetails{
		Log:        s.Log,
		Name:       s.Name,
		Params:     ptrOf(s.Params),
		Pid:        int(s.PID),
		RequestId:  s.RequestID,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Status:     api.RunStatus(s.Status),
		StatusText: api.RunStatusText(s.Status.String()),
	}
	for _, n := range s.Nodes {
		status.Nodes = append(status.Nodes, toNode(n))
	}
	if s.OnSuccess != nil {
		status.OnSuccess = ptrOf(toNode(s.OnSuccess))
	}
	if s.OnFailure != nil {
		status.OnFailure = ptrOf(toNode(s.OnFailure))
	}
	if s.OnCancel != nil {
		status.OnCancel = ptrOf(toNode(s.OnCancel))
	}
	if s.OnExit != nil {
		status.OnExit = ptrOf(toNode(s.OnExit))
	}
	return status
}

func toNode(node *runstore.Node) api.Node {
	return api.Node{
		DoneCount:  node.DoneCount,
		FinishedAt: node.FinishedAt,
		Log:        node.Log,
		RetryCount: node.RetryCount,
		StartedAt:  node.StartedAt,
		Status:     api.NodeStatus(node.Status),
		StatusText: api.NodeStatusText(node.Status.String()),
		Step:       toStep(node.Step),
		Error:      ptrOf(node.Error),
	}
}
