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

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/samber/lo"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	name, err := a.client.CreateDAG(ctx, request.Body.Name)
	if err != nil {
		if errors.Is(err, persistence.ErrDAGAlreadyExists) {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
			}
		}
		return nil, fmt.Errorf("error creating DAG: %w", err)
	}
	return &api.CreateDAG201JSONResponse{
		Name: name,
	}, nil
}

// DeleteDAG implements api.StrictServerInterface.
func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	_, err := a.client.GetStatus(ctx, request.Name)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.Name),
		}
	}
	if err := a.client.DeleteDAG(ctx, request.Name); err != nil {
		return nil, fmt.Errorf("error deleting DAG: %w", err)
	}
	return &api.DeleteDAG204Response{}, nil
}

// GetDAGSpec implements api.StrictServerInterface.
func (a *API) GetDAGSpec(ctx context.Context, request api.GetDAGSpecRequestObject) (api.GetDAGSpecResponseObject, error) {
	spec, err := a.client.GetDAGSpec(ctx, request.Name)
	if err != nil {
		return nil, err
	}

	// Validate the spec
	_, err = a.client.LoadYAML(ctx, []byte(spec), digraph.WithName(request.Name))
	var errs []string

	var loadErrs digraph.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else {
		return nil, err
	}

	return &api.GetDAGSpec200JSONResponse{
		Spec:   spec,
		Errors: errs,
	}, nil
}

// UpdateDAGSpec implements api.StrictServerInterface.
func (a *API) UpdateDAGSpec(ctx context.Context, request api.UpdateDAGSpecRequestObject) (api.UpdateDAGSpecResponseObject, error) {
	// Check the DAG exists
	_, err := a.client.GetStatus(ctx, request.Name)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.Name),
		}
	}

	err = a.client.UpdateDAG(ctx, request.Name, request.Body.Spec)
	var errs []string

	var loadErrs digraph.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else {
		return nil, err
	}

	if len(errs) > 0 {
		return &api.UpdateDAGSpec400JSONResponse{
			Errors: errs,
		}, nil
	}

	return api.UpdateDAGSpec200Response{}, nil
}

// GetDAGRuns implements api.StrictServerInterface.
func (a *API) GetDAGRunHistory(ctx context.Context, request api.GetDAGRunHistoryRequestObject) (api.GetDAGRunHistoryResponseObject, error) {
	historyData := a.readHistoryData(ctx, request.Name)
	return api.GetDAGRunHistory200JSONResponse{
		Runs:     historyData.Runs,
		GridData: historyData.GridData,
	}, nil
}

// GetDAGDetails implements api.StrictServerInterface.
func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	name := request.Name

	tab := api.DAGDetailTabStatus
	if request.Params.Tab != nil {
		tab = *request.Params.Tab
	}

	status, err := a.client.GetStatus(ctx, name)
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
		handlerOn.Failure = ptr(toStep(*handlers.Failure))
	}
	if handlers.Success != nil {
		handlerOn.Success = ptr(toStep(*handlers.Success))
	}
	if handlers.Cancel != nil {
		handlerOn.Cancel = ptr(toStep(*handlers.Cancel))
	}
	if handlers.Exit != nil {
		handlerOn.Exit = ptr(toStep(*handlers.Exit))
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
		Description:       ptr(dag.Description),
		DefaultParams:     ptr(dag.DefaultParams),
		Delay:             ptr(int(dag.Delay.Seconds())),
		Env:               ptr(dag.Env),
		Group:             ptr(dag.Group),
		HandlerOn:         ptr(handlerOn),
		HistRetentionDays: ptr(dag.HistRetentionDays),
		Location:          ptr(dag.Location),
		LogDir:            ptr(dag.LogDir),
		MaxActiveRuns:     ptr(dag.MaxActiveRuns),
		Params:            ptr(dag.Params),
		Preconditions:     ptr(preconditions),
		Schedule:          ptr(schedules),
		Steps:             ptr(steps),
		Tags:              ptr(dag.Tags),
	}

	statusDetails := api.DAGStatusFileDetails{
		Dag:       details,
		Error:     ptr(status.ErrorAsString()),
		File:      status.File,
		LatestRun: toRunDetails(status.Status),
		Suspended: status.Suspended,
	}

	if status.Error != nil {
		statusDetails.Error = ptr(status.Error.Error())
	}

	resp := &api.GetDAGDetails200JSONResponse{
		Title: status.DAG.Name,
		Dag:   statusDetails,
	}

	if err := status.DAG.Validate(); err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case api.DAGDetailTabStatus:
		return resp, nil

	case api.DAGDetailTabSpec:
		spec, err := a.client.GetDAGSpec(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("error getting DAG spec: %w", err)
		}
		resp.Definition = ptr(spec)

	case api.DAGDetailTabHistory:
		historyData := a.readHistoryData(ctx, status.DAG.Name)
		resp.HistoryData = &historyData

	case api.DAGDetailTabLog:
		if request.Params.Step == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "Step name is required",
			}
		}

		l, err := a.readStepLog(ctx, dag, *request.Params.Step, value(request.Params.RequestId))
		if err != nil {
			return nil, err
		}
		resp.StepLog = l

	case api.DAGDetailTabSchedulerLog:
		l, err := a.readLog(ctx, dag, value(request.Params.RequestId))
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
	dagName string,
) api.DAGHistoryData {
	defaultHistoryLimit := 30
	recentRuns := a.client.GetRecentHistory(ctx, dagName, defaultHistoryLimit)

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

	for idx, run := range recentRuns {
		for _, node := range run.Status.Nodes {
			addStatusFn(data, len(recentRuns), idx, node.Step.Name, node.Status)
		}
	}

	var grid []api.DAGLogGridItem
	for node, statusList := range data {
		var history []api.NodeStatus
		for _, s := range statusList {
			history = append(history, api.NodeStatus(s))
		}
		grid = append(grid, api.DAGLogGridItem{
			Name:    node,
			History: history,
		})
	}

	sort.Slice(grid, func(i, j int) bool {
		return strings.Compare(grid[i].Name, grid[j].Name) <= 0
	})

	handlers := map[string][]scheduler.NodeStatus{}
	for idx, log := range recentRuns {
		if n := log.Status.OnSuccess; n != nil {
			addStatusFn(handlers, len(recentRuns), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnFailure; n != nil {
			addStatusFn(handlers, len(recentRuns), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnCancel; n != nil {
			n := log.Status.OnCancel
			addStatusFn(handlers, len(recentRuns), idx, n.Step.Name, n.Status)
		}
		if n := log.Status.OnExit; n != nil {
			addStatusFn(handlers, len(recentRuns), idx, n.Step.Name, n.Status)
		}
	}

	for _, handlerType := range []digraph.HandlerType{
		digraph.HandlerOnSuccess,
		digraph.HandlerOnFailure,
		digraph.HandlerOnCancel,
		digraph.HandlerOnExit,
	} {
		if statusList, ok := handlers[handlerType.String()]; ok {
			var history []api.NodeStatus
			for _, status := range statusList {
				history = append(history, api.NodeStatus(status))
			}
			grid = append(grid, api.DAGLogGridItem{
				Name:    handlerType.String(),
				History: history,
			})
		}
	}

	var runs []api.RunDetails
	for _, log := range recentRuns {
		runs = append(runs, toRunDetails(log.Status))
	}

	return api.DAGHistoryData{
		GridData: grid,
		Runs:     lo.Reverse(runs),
	}
}

func (a *API) readLog(
	ctx context.Context,
	dag *digraph.DAG,
	reqID string,
) (*api.SchedulerLog, error) {
	status, err := a.readStatus(ctx, dag, reqID)
	if err != nil {
		return nil, err
	}

	logFile := status.Log

	content, err := readFileContent(logFile, nil)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
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
	reqID string,
) (*api.StepLog, error) {
	status, err := a.readStatus(ctx, dag, reqID)
	if err != nil {
		return nil, err
	}

	// Find the step in the status to get the log file.
	var node *persistence.Node
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
		return nil, fmt.Errorf("step %s not found in status", stepName)
	}

	var decoder *encoding.Decoder
	if strings.ToLower(a.logEncodingCharset) == "euc-jp" {
		decoder = japanese.EUCJP.NewDecoder()
	}

	logContent, err := readFileContent(node.Log, decoder)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Log, err)
	}

	return &api.StepLog{
		LogFile: node.Log,
		Step:    toNode(node),
		Content: string(logContent),
	}, nil
}

func (a *API) readStatus(ctx context.Context, dag *digraph.DAG, reqID string) (*persistence.Status, error) {
	// If a request ID is provided, fetch the status by request ID.
	if reqID != "" {
		return a.client.GetStatusByRequestID(ctx, dag, reqID)
	}

	// If no request ID is provided, fetch the latest status.
	status, err := a.client.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, err
	}
	return &status, nil
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
	var opts []client.ListStatusOption
	if request.Params.PerPage != nil {
		opts = append(opts, client.WithLimit(*request.Params.PerPage))
	}
	if request.Params.Page != nil {
		opts = append(opts, client.WithPage(*request.Params.Page))
	}
	if request.Params.Name != nil {
		opts = append(opts, client.WithName(*request.Params.Name))
	}
	if request.Params.Tag != nil {
		opts = append(opts, client.WithTag(*request.Params.Tag))
	}

	result, errList, err := a.client.ListStatus(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:     errList,
		Pagination: toPagination(*result),
	}

	for _, item := range result.Items {
		run := api.RunSummary{
			Log:        item.Status.Log,
			Name:       item.Status.Name,
			Params:     ptr(item.Status.Params),
			Pid:        ptr(int(item.Status.PID)),
			RequestId:  item.Status.RequestID,
			StartedAt:  item.Status.StartedAt,
			FinishedAt: item.Status.FinishedAt,
			Status:     api.Status(item.Status.Status),
			StatusText: api.StatusText(item.Status.StatusText),
		}

		var loadErrs digraph.ErrorList
		var errs []string
		if item.Error != nil && errors.As(item.Error, &loadErrs) {
			errs = loadErrs.ToStringList()
		} else if item.Error != nil {
			errs = []string{item.Error.Error()}
		}

		dag := api.DAGFile{
			Errors:    errs,
			LatestRun: run,
			Suspended: item.Suspended,
			Dag:       toDAG(item.DAG),
		}

		resp.Dags = append(resp.Dags, dag)
	}

	return resp, nil
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	tags, errs, err := a.client.GetTagList(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting tags: %w", err)
	}
	return &api.ListTags200JSONResponse{
		Tags:   tags,
		Errors: errs,
	}, nil
}

// GetDAGRunStatus implements api.StrictServerInterface.
func (a *API) GetDAGRunStatus(ctx context.Context, request api.GetDAGRunStatusRequestObject) (api.GetDAGRunStatusResponseObject, error) {
	dagName := request.Name
	requestId := request.RequestId

	dagWithStatus, err := a.client.GetStatus(ctx, dagName)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	if requestId == "latest" {
		return &api.GetDAGRunStatus200JSONResponse{
			Run: toRunDetails(dagWithStatus.Status),
		}, nil
	}

	run, err := a.client.GetStatusByRequestID(ctx, dagWithStatus.DAG, requestId)
	if err != nil {
		return nil, fmt.Errorf("error getting status by request ID: %w", err)
	}

	return &api.GetDAGRunStatus200JSONResponse{
		Run: toRunDetails(*run),
	}, nil
}

// PostDAGAction implements api.StrictServerInterface.
func (a *API) PostDAGAction(ctx context.Context, request api.PostDAGActionRequestObject) (api.PostDAGActionResponseObject, error) {
	action := request.Body.Action

	var status client.DAGStatus
	if action != api.DAGActionSave {
		s, err := a.client.GetStatus(ctx, request.Name)
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
		a.client.StartAsync(ctx, status.DAG, client.StartOptions{
			Params: value(request.Body.Params),
		})
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionSuspend:
		b, err := strconv.ParseBool(value(request.Body.Value))
		if err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid value for suspend, must be true or false",
			}
		}
		if err := a.client.ToggleSuspend(ctx, request.Name, b); err != nil {
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
		if err := a.client.Stop(ctx, status.DAG); err != nil {
			return nil, fmt.Errorf("error stopping DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionRetry:
		if request.Body.RequestId == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "RequestId is required for retry action",
			}
		}
		if err := a.client.Retry(ctx, status.DAG, *request.Body.RequestId); err != nil {
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
				Message:    "Cannot change status of running DAG",
			}
		}
		if request.Body.RequestId == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "RequestId is required for mark-success action",
			}
		}
		if request.Body.Step == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "Step is required for mark-success action",
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
				Message:    "Value is required for save action (DAG spec)",
			}
		}

		if err := a.client.UpdateDAG(ctx, request.Name, *request.Body.Value); err != nil {
			return nil, err
		}

		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionRename:
		if request.Body.Value == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "Value is required for rename action (new name)",
			}
		}

		newName := *request.Body.Value
		if err := a.client.Rename(ctx, request.Name, newName); err != nil {
			return nil, fmt.Errorf("error renaming DAG: %w", err)
		}

		return api.PostDAGAction200JSONResponse{
			NewName: ptr(newName),
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
	dagStatus client.DAGStatus,
	to scheduler.NodeStatus,
) error {
	status, err := a.client.GetStatusByRequestID(ctx, dagStatus.DAG, reqID)
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
			Message:    fmt.Sprintf("Step %s not found in status", step),
		}
	}

	status.Nodes[idxToUpdate].Status = to
	status.Nodes[idxToUpdate].StatusText = to.String()

	if err := a.client.UpdateStatus(ctx, dagStatus.DAG, *status); err != nil {
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
			Message:    "Query is required",
		}
	}

	ret, errs, err := a.client.Grep(ctx, query)
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
			Dag:     toDAG(item.DAG),
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
		Group:         ptr(dag.Group),
		Description:   ptr(dag.Description),
		Params:        ptr(dag.Params),
		DefaultParams: ptr(dag.DefaultParams),
		Tags:          ptr(dag.Tags),
		Schedule:      ptr(schedules),
	}
}

func toStep(obj digraph.Step) api.Step {
	var conditions []api.Precondition
	for _, cond := range obj.Preconditions {
		conditions = append(conditions, toPrecondition(cond))
	}

	repeatPolicy := api.RepeatPolicy{
		Repeat:   ptr(obj.RepeatPolicy.Repeat),
		Interval: ptr(int(obj.RepeatPolicy.Interval.Seconds())),
	}

	step := api.Step{
		Name:          obj.Name,
		Description:   ptr(obj.Description),
		Args:          ptr(obj.Args),
		CmdWithArgs:   ptr(obj.CmdWithArgs),
		Command:       ptr(obj.Command),
		Depends:       ptr(obj.Depends),
		Dir:           ptr(obj.Dir),
		MailOnError:   ptr(obj.MailOnError),
		Output:        ptr(obj.Output),
		Preconditions: ptr(conditions),
		RepeatPolicy:  ptr(repeatPolicy),
		Script:        ptr(obj.Script),
	}

	if obj.SubWorkflow != nil {
		step.Run = ptr(obj.SubWorkflow.Name)
		step.Params = ptr(obj.SubWorkflow.Params)
	}
	return step
}

func toPrecondition(obj digraph.Condition) api.Precondition {
	return api.Precondition{
		Condition: ptr(obj.Condition),
		Expected:  ptr(obj.Expected),
	}
}

func toRunDetails(s persistence.Status) api.RunDetails {
	status := api.RunDetails{
		Log:        s.Log,
		Name:       s.Name,
		Params:     ptr(s.Params),
		Pid:        ptr(int(s.PID)),
		RequestId:  s.RequestID,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Status:     api.Status(s.Status),
		StatusText: api.StatusText(s.StatusText),
	}
	for _, n := range s.Nodes {
		status.Nodes = append(status.Nodes, toNode(n))
	}
	if s.OnSuccess != nil {
		status.OnSuccess = ptr(toNode(s.OnSuccess))
	}
	if s.OnFailure != nil {
		status.OnFailure = ptr(toNode(s.OnFailure))
	}
	if s.OnCancel != nil {
		status.OnCancel = ptr(toNode(s.OnCancel))
	}
	if s.OnExit != nil {
		status.OnExit = ptr(toNode(s.OnExit))
	}
	return status
}

func toNode(node *persistence.Node) api.Node {
	return api.Node{
		DoneCount:  node.DoneCount,
		FinishedAt: node.FinishedAt,
		Log:        node.Log,
		RetryCount: node.RetryCount,
		StartedAt:  node.StartedAt,
		Status:     api.NodeStatus(node.Status),
		StatusText: api.NodeStatusText(node.StatusText),
		Step:       toStep(node.Step),
		Error:      ptr(node.Error),
	}
}
