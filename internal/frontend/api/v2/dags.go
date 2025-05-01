package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
)

func (a *API) CreateNewDAG(ctx context.Context, request api.CreateNewDAGRequestObject) (api.CreateNewDAGResponseObject, error) {
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
	return &api.CreateNewDAG201JSONResponse{
		Name: name,
	}, nil
}

func (a *API) DeleteDAGByFileName(ctx context.Context, request api.DeleteDAGByFileNameRequestObject) (api.DeleteDAGByFileNameResponseObject, error) {
	_, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if err := a.client.DeleteDAG(ctx, request.FileName); err != nil {
		return nil, fmt.Errorf("error deleting DAG: %w", err)
	}
	return &api.DeleteDAGByFileName204Response{}, nil
}

func (a *API) GetDAGDefinition(ctx context.Context, request api.GetDAGDefinitionRequestObject) (api.GetDAGDefinitionResponseObject, error) {
	spec, err := a.client.GetDAGSpec(ctx, request.FileName)
	if err != nil {
		return nil, err
	}

	// Validate the spec
	dag, err := a.client.LoadYAML(ctx, []byte(spec), digraph.WithName(request.FileName))
	var errs []string

	var loadErrs digraph.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else if err != nil {
		return nil, err
	}

	return &api.GetDAGDefinition200JSONResponse{
		Dag:    toDAGDetails(dag),
		Spec:   spec,
		Errors: errs,
	}, nil
}

func (a *API) UpdateDAGDefinition(ctx context.Context, request api.UpdateDAGDefinitionRequestObject) (api.UpdateDAGDefinitionResponseObject, error) {
	_, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	err = a.client.UpdateDAG(ctx, request.FileName, request.Body.Spec)
	var errs []string

	var loadErrs digraph.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else {
		return nil, err
	}

	return api.UpdateDAGDefinition200JSONResponse{
		Errors: errs,
	}, nil
}

func (a *API) RenameDAG(ctx context.Context, request api.RenameDAGRequestObject) (api.RenameDAGResponseObject, error) {
	status, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if status.Status.Status == scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is running",
		}
	}
	if err := a.client.MoveDAG(ctx, request.FileName, request.Body.NewFileName); err != nil {
		return nil, fmt.Errorf("failed to move DAG: %w", err)
	}
	return api.RenameDAG200Response{}, nil
}

func (a *API) GetDAGExecutionHistory(ctx context.Context, request api.GetDAGExecutionHistoryRequestObject) (api.GetDAGExecutionHistoryResponseObject, error) {
	status, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	defaultHistoryLimit := 30
	recentRuns := a.client.GetRecentHistory(ctx, status.DAG.Name, defaultHistoryLimit)

	var runs []api.RunDetails
	for _, log := range recentRuns {
		runs = append(runs, toRunDetails(log.Status))
	}

	gridData := a.readHistoryData(ctx, recentRuns)
	return api.GetDAGExecutionHistory200JSONResponse{
		Runs:     runs,
		GridData: gridData,
	}, nil
}

func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	fileName := request.FileName
	status, err := a.client.GetDAGStatus(ctx, fileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", fileName),
		}
	}

	dag := status.DAG
	details := toDAGDetails(dag)
	var errs []string
	if status.Error != nil {
		errs = append(errs, status.Error.Error())
	}

	return api.GetDAGDetails200JSONResponse{
		Dag:       details,
		LatestRun: toRunDetails(status.Status),
		Suspended: status.Suspended,
		Errors:    errs,
	}, nil
}

func (a *API) readHistoryData(
	_ context.Context,
	recentRuns []persistence.Run,
) []api.DAGGridItem {
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

	var grid []api.DAGGridItem
	for node, statusList := range data {
		var history []api.NodeStatus
		for _, s := range statusList {
			history = append(history, api.NodeStatus(s))
		}
		grid = append(grid, api.DAGGridItem{
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
			grid = append(grid, api.DAGGridItem{
				Name:    handlerType.String(),
				History: history,
			})
		}
	}

	return grid
}

func (a *API) ListAllDAGs(ctx context.Context, request api.ListAllDAGsRequestObject) (api.ListAllDAGsResponseObject, error) {
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

	resp := &api.ListAllDAGs200JSONResponse{
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
			FileName:  item.DAG.FileName(),
			Errors:    errs,
			LatestRun: run,
			Suspended: item.Suspended,
			Dag:       toDAG(item.DAG),
		}

		resp.Dags = append(resp.Dags, dag)
	}

	return resp, nil
}

func (a *API) GetAllDAGTags(ctx context.Context, _ api.GetAllDAGTagsRequestObject) (api.GetAllDAGTagsResponseObject, error) {
	tags, errs, err := a.client.GetTagList(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting tags: %w", err)
	}
	return &api.GetAllDAGTags200JSONResponse{
		Tags:   tags,
		Errors: errs,
	}, nil
}

func (a *API) GetDAGRunDetails(ctx context.Context, request api.GetDAGRunDetailsRequestObject) (api.GetDAGRunDetailsResponseObject, error) {
	dagFileName := request.FileName
	requestId := request.RequestId

	dagWithStatus, err := a.client.GetDAGStatus(ctx, dagFileName)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	if requestId == "latest" {
		return &api.GetDAGRunDetails200JSONResponse{
			Run: toRunDetails(dagWithStatus.Status),
		}, nil
	}

	run, err := a.client.GetStatusByRequestID(ctx, dagWithStatus.DAG, requestId)
	if err != nil {
		return nil, fmt.Errorf("error getting status by request ID: %w", err)
	}

	return &api.GetDAGRunDetails200JSONResponse{
		Run: toRunDetails(*run),
	}, nil
}

func (a *API) ExecuteDAG(ctx context.Context, request api.ExecuteDAGRequestObject) (api.ExecuteDAGResponseObject, error) {
	status, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if status.Status.Status == scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeAlreadyRunning,
			Message:    "DAG is already running",
		}
	}
	if err := a.client.StartDAG(ctx, status.DAG, client.StartOptions{
		Params: value(request.Body.Params),
	}); err != nil {
		return nil, fmt.Errorf("error starting DAG: %w", err)
	}
	return api.ExecuteDAG200Response{}, nil
}

func (a *API) TerminateDAGExecution(ctx context.Context, request api.TerminateDAGExecutionRequestObject) (api.TerminateDAGExecutionResponseObject, error) {
	status, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if status.Status.Status != scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is not running",
		}
	}
	if err := a.client.StopDAG(ctx, status.DAG); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}
	return api.TerminateDAGExecution200Response{}, nil
}

func (a *API) RetryDAGExecution(ctx context.Context, request api.RetryDAGExecutionRequestObject) (api.RetryDAGExecutionResponseObject, error) {
	status, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if status.Status.Status == scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeAlreadyRunning,
			Message:    "DAG is already running",
		}
	}

	if err := a.client.RetryDAG(ctx, status.DAG, request.Body.RequestId); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGExecution200Response{}, nil
}

func (a *API) UpdateDAGSuspensionState(ctx context.Context, request api.UpdateDAGSuspensionStateRequestObject) (api.UpdateDAGSuspensionStateResponseObject, error) {
	_, err := a.client.GetDAGStatus(ctx, request.FileName)
	if err != nil {
		return &api.UpdateDAGSuspensionState404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("DAG %s not found", request.FileName),
		}, nil
	}

	if err := a.client.ToggleSuspend(ctx, request.FileName, request.Body.Suspend); err != nil {
		return nil, fmt.Errorf("error toggling suspend: %w", err)
	}

	return api.UpdateDAGSuspensionState200Response{}, nil
}

func (a *API) SearchDAGDefinitions(ctx context.Context, request api.SearchDAGDefinitionsRequestObject) (api.SearchDAGDefinitionsResponseObject, error) {
	ret, errs, err := a.client.GrepDAG(ctx, request.Params.Q)
	if err != nil {
		return nil, fmt.Errorf("error searching DAGs: %w", err)
	}

	var results []api.SearchResultItem
	for _, item := range ret {
		var matches []api.SearchDAGsMatchItem
		for _, match := range item.Matches {
			matches = append(matches, api.SearchDAGsMatchItem{
				Line:       match.Line,
				LineNumber: match.LineNumber,
				StartLine:  match.StartLine,
			})
		}

		results = append(results, api.SearchResultItem{
			Name:    item.Name,
			Dag:     toDAG(item.DAG),
			Matches: matches,
		})
	}

	return &api.SearchDAGDefinitions200JSONResponse{
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

	if obj.SubDAG != nil {
		step.Run = ptr(obj.SubDAG.Name)
		step.Params = ptr(obj.SubDAG.Params)
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

func toDAGDetails(dag *digraph.DAG) *api.DAGDetails {
	var details *api.DAGDetails
	if dag == nil {
		return details
	}

	var steps []api.Step
	for _, step := range dag.Steps {
		steps = append(steps, toStep(step))
	}

	handlers := dag.HandlerOn

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
	for _, s := range dag.Schedule {
		schedules = append(schedules, api.Schedule{
			Expression: s.Expression,
		})
	}

	var preconditions []api.Precondition
	for _, p := range dag.Preconditions {
		preconditions = append(preconditions, toPrecondition(p))
	}

	return &api.DAGDetails{
		Name:              dag.Name,
		Description:       ptr(dag.Description),
		DefaultParams:     ptr(dag.DefaultParams),
		Delay:             ptr(int(dag.Delay.Seconds())),
		Env:               ptr(dag.Env),
		Group:             ptr(dag.Group),
		HandlerOn:         ptr(handlerOn),
		HistRetentionDays: ptr(dag.HistRetentionDays),
		LogDir:            ptr(dag.LogDir),
		MaxActiveRuns:     ptr(dag.MaxActiveRuns),
		Params:            ptr(dag.Params),
		Preconditions:     ptr(preconditions),
		Schedule:          ptr(schedules),
		Steps:             ptr(steps),
		Tags:              ptr(dag.Tags),
	}
}
