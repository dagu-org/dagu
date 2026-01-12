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
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/samber/lo/mutable"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// CreateDAG implements api.StrictServerInterface.
func (a *API) CreateDAG(ctx context.Context, request api.CreateDAGRequestObject) (api.CreateDAGResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionWriteDAGs); err != nil {
		return nil, err
	}

	_, err := a.dagStore.GetMetadata(ctx, request.Body.Value)
	if err == nil {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("DAG %s already exists", request.Body.Value),
		}
	}

	spec := []byte(`steps:
  - name: step1
    command: echo hello
`)

	if err := a.dagStore.Create(ctx, request.Body.Value, spec); err != nil {
		return nil, fmt.Errorf("error creating DAG: %w", err)
	}

	return &api.CreateDAG201JSONResponse{
		DagID: request.Body.Value,
	}, nil
}

// DeleteDAG implements api.StrictServerInterface.
func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	if err := a.isAllowed(ctx, config.PermissionWriteDAGs); err != nil {
		return nil, err
	}

	_, err := a.dagStore.GetMetadata(ctx, request.Name)
	if err != nil {
		if errors.Is(err, exec.ErrDAGNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("DAG %s not found", request.Name),
			}
		}
		return nil, fmt.Errorf("error getting DAG metadata: %w", err)
	}

	if err := a.dagStore.Delete(ctx, request.Name); err != nil {
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

	dag, err := a.dagStore.GetDetails(ctx, name)
	if err != nil {
		if errors.Is(err, exec.ErrDAGNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("DAG %s not found", name),
			}
		}
		return nil, fmt.Errorf("error getting DAG metadata: %w", err)
	}

	status, err := a.dagRunManager.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", name),
		}
	}

	var steps []api.Step
	for _, step := range dag.Steps {
		steps = append(steps, toStep(step))
	}

	handlers := dag.HandlerOn

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
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{
			Expression: s.Expression,
		})
	}

	var preconditions []api.Precondition
	for i := range dag.Preconditions {
		preconditions = append(preconditions, toPrecondition(dag.Preconditions[i]))
	}

	details := api.DAGDetails{
		Name:              dag.Name,
		Description:       ptrOf(dag.Description),
		DefaultParams:     ptrOf(dag.DefaultParams),
		Delay:             ptrOf(int(dag.Delay.Seconds())),
		Env:               ptrOf(dag.Env),
		Group:             ptrOf(dag.Group),
		HandlerOn:         ptrOf(handlerOn),
		HistRetentionDays: ptrOf(dag.HistRetentionDays),
		LogDir:            ptrOf(dag.LogDir),
		MaxActiveRuns:     ptrOf(dag.MaxActiveSteps),
		Params:            ptrOf(dag.Params),
		Preconditions:     ptrOf(preconditions),
		Schedule:          ptrOf(schedules),
		Steps:             ptrOf(steps),
		Tags:              ptrOf(dag.Tags),
	}

	statusDetails := api.DAGStatusFileDetails{
		DAG:       details,
		Status:    toStatus(status),
		Suspended: a.dagStore.IsSuspended(ctx, name),
	}

	resp := &api.GetDAGDetails200JSONResponse{
		Title: dag.Name,
		DAG:   statusDetails,
	}

	if err := dag.Validate(); err != nil {
		resp.Errors = append(resp.Errors, err.Error())
	}

	switch tab {
	case api.DAGDetailTabStatus:
		return resp, nil

	case api.DAGDetailTabSpec:
		spec, err := a.dagStore.GetSpec(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("error getting DAG spec: %w", err)
		}
		resp.Definition = ptrOf(spec)

	case api.DAGDetailTabHistory:
		historyData := a.readHistoryData(ctx, dag)
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
	dag *core.DAG,
) api.DAGHistoryData {
	defaultHistoryLimit := 30
	statuses := a.dagRunManager.ListRecentStatus(ctx, dag.Name, defaultHistoryLimit)

	data := map[string][]core.NodeStatus{}

	addStatusFn := func(
		data map[string][]core.NodeStatus,
		logLen int,
		logIdx int,
		nodeName string,
		status core.NodeStatus,
	) {
		if _, ok := data[nodeName]; !ok {
			data[nodeName] = make([]core.NodeStatus, logLen)
		}
		data[nodeName][logIdx] = status
	}

	for idx, status := range statuses {
		for _, node := range status.Nodes {
			addStatusFn(data, len(statuses), idx, node.Step.Name, node.Status)
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

	handlers := map[string][]core.NodeStatus{}
	for idx, log := range statuses {
		if n := log.OnSuccess; n != nil {
			addStatusFn(handlers, len(statuses), idx, n.Step.Name, n.Status)
		}
		if n := log.OnFailure; n != nil {
			addStatusFn(handlers, len(statuses), idx, n.Step.Name, n.Status)
		}
		if n := log.OnCancel; n != nil {
			addStatusFn(handlers, len(statuses), idx, n.Step.Name, n.Status)
		}
		if n := log.OnExit; n != nil {
			addStatusFn(handlers, len(statuses), idx, n.Step.Name, n.Status)
		}
	}

	for _, handlerType := range []core.HandlerType{
		core.HandlerOnSuccess,
		core.HandlerOnFailure,
		core.HandlerOnCancel,
		core.HandlerOnExit,
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
	for _, status := range statuses {
		statusFile := api.DAGLogStatusFile{
			File:   "", // We don't provide the file name here anymore
			Status: toStatus(status),
		}
		statusList = append(statusList, statusFile)
	}

	mutable.Reverse(statusList)
	return api.DAGHistoryData{
		GridData: grid,
		Logs:     statusList,
	}
}

func (a *API) readLog(
	ctx context.Context,
	dag *core.DAG,
	statusFile string,
) (*api.SchedulerLog, error) {
	var logFile string

	if statusFile != "" {
		status, err := filedagrun.ParseStatusFile(statusFile)
		if err != nil {
			return nil, err
		}
		logFile = status.Log
	}

	if logFile == "" {
		lastStatus, err := a.dagRunManager.GetLatestStatus(ctx, dag)
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
	dag *core.DAG,
	stepName string,
	statusFile string,
) (*api.StepLog, error) {
	var status *exec.DAGRunStatus

	if statusFile != "" {
		parsedStatus, err := filedagrun.ParseStatusFile(statusFile)
		if err != nil {
			return nil, err
		}
		status = parsedStatus
	}

	if status == nil {
		latestStatus, err := a.dagRunManager.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		status = &latestStatus
	}

	// Find the step in the status to get the log file.
	var node *exec.Node
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

	logContent, err := readFileContent(node.Stdout, decoder)
	if err != nil {
		return nil, fmt.Errorf("error reading log file %s: %w", node.Stdout, err)
	}

	return &api.StepLog{
		LogFile: node.Stdout,
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
	pg := exec.NewPaginator(valueOf(request.Params.Page), valueOf(request.Params.Limit))
	result, errList, err := a.dagStore.List(ctx, exec.ListDAGsOptions{
		Paginator: &pg,
		Name:      valueOf(request.Params.SearchName),
		Tag:       valueOf(request.Params.SearchTag),
	})

	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	// Get status for each DAG
	dagStatuses := make([]exec.DAGRunStatus, len(result.Items))
	for _, item := range result.Items {
		status, err := a.dagRunManager.GetLatestStatus(ctx, item)
		if err != nil {
			errList = append(errList, err.Error())
		}
		dagStatuses = append(dagStatuses, status)
	}

	var dags []api.DAGStatusFile
	for i, item := range result.Items {
		status := api.DAGStatus{
			Log:        ptrOf(dagStatuses[i].Log),
			Name:       dagStatuses[i].Name,
			Params:     ptrOf(dagStatuses[i].Params),
			RequestId:  dagStatuses[i].DAGRunID,
			StartedAt:  dagStatuses[i].StartedAt,
			FinishedAt: dagStatuses[i].FinishedAt,
			Status:     api.RunStatus(dagStatuses[i].Status),
			StatusText: api.RunStatusText(dagStatuses[i].Status.String()),
		}

		dag := api.DAGStatusFile{
			Status:    status,
			Suspended: a.dagStore.IsSuspended(ctx, item.Name),
			DAG:       toDAG(item),
		}

		dags = append(dags, dag)
	}

	resp := &api.ListDAGs200JSONResponse{
		DAGs:      dags,
		Errors:    ptrOf(errList),
		PageCount: result.TotalPages,
		HasError:  len(errList) > 0,
	}

	return resp, nil
}

// ListTags implements api.StrictServerInterface.
func (a *API) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	tags, errs, err := a.dagStore.TagList(ctx)
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

	var status exec.DAGRunStatus
	var dag *core.DAG

	if action != api.DAGActionSave {
		d, err := a.dagStore.GetMetadata(ctx, request.Name)
		if err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("DAG %s not found", request.Name),
			}
		}
		dag = d

		s, err := a.dagRunManager.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, err
		}
		status = s
	}

	switch request.Body.Action {
	case api.DAGActionStart:
		if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
			return nil, err
		}

		if status.Status == core.Running {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeAlreadyRunning,
				Message:    "DAG is already running",
			}
		}

		spec := a.subCmdBuilder.Start(dag, runtime.StartOptions{
			Params: valueOf(request.Body.Params),
		})

		if err := runtime.Start(ctx, spec); err != nil {
			return nil, fmt.Errorf("error starting DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionSuspend:
		if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
			return nil, err
		}

		b, err := strconv.ParseBool(valueOf(request.Body.Value))
		if err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "invalid value for suspend, must be true or false",
			}
		}
		if err := a.dagStore.ToggleSuspend(ctx, request.Name, b); err != nil {
			return nil, fmt.Errorf("error toggling suspend: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionStop:
		if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
			return nil, err
		}

		if status.Status != core.Running {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeNotRunning,
				Message:    "DAG is not running",
			}
		}
		if err := a.dagRunManager.Stop(ctx, dag, ""); err != nil {
			return nil, fmt.Errorf("error stopping DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionRetry:
		if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
			return nil, err
		}

		if request.Body.RequestId == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "requestId is required for retry action",
			}
		}
		if request.Body.Step != nil && *request.Body.Step != "" {
			spec := a.subCmdBuilder.Retry(dag, *request.Body.RequestId, *request.Body.Step)
			if err := runtime.Start(ctx, spec); err != nil {
				return nil, fmt.Errorf("error retrying DAG step: %w", err)
			}
			return api.PostDAGAction200JSONResponse{}, nil
		}

		spec := a.subCmdBuilder.Retry(dag, *request.Body.RequestId, "")
		if err := runtime.Start(ctx, spec); err != nil {
			return nil, fmt.Errorf("error retrying DAG: %w", err)
		}
		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionMarkSuccess:
		fallthrough

	case api.DAGActionMarkFailed:
		if err := a.isAllowed(ctx, config.PermissionRunDAGs); err != nil {
			return nil, err
		}

		if status.Status == core.Running {
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
		toStatus := core.NodeSucceeded
		if action == api.DAGActionMarkFailed {
			toStatus = core.NodeFailed
		}

		if err := a.updateStatus(ctx, *request.Body.RequestId, *request.Body.Step, dag, toStatus); err != nil {
			return nil, fmt.Errorf("error marking DAG as success: %w", err)
		}

		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionSave:
		if err := a.isAllowed(ctx, config.PermissionWriteDAGs); err != nil {
			return nil, err
		}

		if request.Body.Value == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "value is required for save action (DAG spec)",
			}
		}

		if err := a.dagStore.UpdateSpec(ctx, request.Name, []byte(*request.Body.Value)); err != nil {
			return nil, err
		}

		return api.PostDAGAction200JSONResponse{}, nil

	case api.DAGActionRename:
		if err := a.isAllowed(ctx, config.PermissionWriteDAGs); err != nil {
			return nil, err
		}

		if request.Body.Value == nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "value is required for rename action (new name)",
			}
		}

		old, err := a.dagStore.GetMetadata(ctx, request.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting the DAG metadata: %w", err)
		}

		newName := *request.Body.Value
		if err := a.dagStore.Rename(ctx, request.Name, newName); err != nil {
			return nil, fmt.Errorf("error renaming DAG: %w", err)
		}

		renamed, err := a.dagStore.GetMetadata(ctx, newName)
		if err != nil {
			return nil, fmt.Errorf("error getting new DAG metadata: %w", err)
		}

		// Rename the dag-runs associated with the old name
		if err := a.dagRunStore.RenameDAGRuns(ctx, old.Name, renamed.Name); err != nil {
			return nil, fmt.Errorf("error renaming dag-runs: %w", err)
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
	dagRunID string,
	step string,
	dag *core.DAG,
	to core.NodeStatus,
) error {
	status, err := a.dagRunManager.GetCurrentStatus(ctx, dag, dagRunID)
	if err != nil {
		return fmt.Errorf("error getting status: %w", err)
	}

	if status.Status == core.Running {
		return &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "cannot change status of running DAG",
		}
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

	root := exec.NewDAGRunRef(dag.Name, dagRunID)
	if err := a.dagRunManager.UpdateStatus(ctx, root, *status); err != nil {
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

	ret, errs, err := a.dagStore.Grep(ctx, query)
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

func toDAG(dag *core.DAG) api.DAG {
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

func toStep(obj core.Step) api.Step {
	var conditions []api.Precondition
	for i := range obj.Preconditions {
		conditions = append(conditions, toPrecondition(obj.Preconditions[i]))
	}

	repeatPolicy := api.RepeatPolicy{
		Repeat:   ptrOf(obj.RepeatPolicy.RepeatMode != ""),
		Interval: ptrOf(int(obj.RepeatPolicy.Interval.Seconds())),
	}

	// Extract command info from Commands field (new format)
	var command, cmdWithArgs string
	var args []string
	if len(obj.Commands) > 0 {
		command = obj.Commands[0].Command
		args = obj.Commands[0].Args
		cmdWithArgs = obj.Commands[0].CmdWithArgs
	}

	step := api.Step{
		Name:          obj.Name,
		Description:   ptrOf(obj.Description),
		Args:          ptrOf(args),
		CmdWithArgs:   ptrOf(cmdWithArgs),
		Command:       ptrOf(command),
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

func toPrecondition(obj *core.Condition) api.Precondition {
	return api.Precondition{
		Condition: ptrOf(obj.Condition),
		Expected:  ptrOf(obj.Expected),
	}
}

func toStatus(s exec.DAGRunStatus) api.DAGStatusDetails {
	status := api.DAGStatusDetails{
		Log:        s.Log,
		Name:       s.Name,
		Params:     ptrOf(s.Params),
		RequestId:  s.DAGRunID,
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

func toNode(node *exec.Node) api.Node {
	return api.Node{
		DoneCount:  node.DoneCount,
		FinishedAt: node.FinishedAt,
		Log:        node.Stdout,
		RetryCount: node.RetryCount,
		StartedAt:  node.StartedAt,
		Status:     api.NodeStatus(node.Status),
		StatusText: api.NodeStatusText(node.Status.String()),
		Step:       toStep(node.Step),
		Error:      ptrOf(node.Error),
	}
}
