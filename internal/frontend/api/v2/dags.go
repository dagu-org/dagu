package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/models"
)

func (a *API) CreateNewDAG(ctx context.Context, request api.CreateNewDAGRequestObject) (api.CreateNewDAGResponseObject, error) {
	spec := []byte(`steps:
  - name: step1
    command: echo hello
`)

	if err := a.dagRepository.Create(ctx, request.Body.Name, spec); err != nil {
		if errors.Is(err, models.ErrDAGAlreadyExists) {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
			}
		}
		return nil, fmt.Errorf("error creating DAG: %w", err)
	}

	return &api.CreateNewDAG201JSONResponse{
		Name: request.Body.Name,
	}, nil
}

func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	_, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if err := a.dagRepository.Delete(ctx, request.FileName); err != nil {
		return nil, fmt.Errorf("error deleting DAG: %w", err)
	}
	return &api.DeleteDAG204Response{}, nil
}

func (a *API) GetDAGSpec(ctx context.Context, request api.GetDAGSpecRequestObject) (api.GetDAGSpecResponseObject, error) {
	spec, err := a.dagRepository.GetSpec(ctx, request.FileName)
	if err != nil {
		return nil, err
	}

	// Validate the spec
	dag, err := a.historyManager.LoadYAML(ctx, []byte(spec), digraph.WithName(request.FileName))
	var errs []string

	var loadErrs digraph.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else if err != nil {
		return nil, err
	}

	return &api.GetDAGSpec200JSONResponse{
		Dag:    toDAGDetails(dag),
		Spec:   spec,
		Errors: errs,
	}, nil
}

func (a *API) UpdateDAGSpec(ctx context.Context, request api.UpdateDAGSpecRequestObject) (api.UpdateDAGSpecResponseObject, error) {
	err := a.dagRepository.UpdateSpec(ctx, request.FileName, []byte(request.Body.Spec))

	var loadErrs digraph.ErrorList
	var errs []string

	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else {
		return nil, err
	}

	return api.UpdateDAGSpec200JSONResponse{
		Errors: errs,
	}, nil
}

func (a *API) RenameDAG(ctx context.Context, request api.RenameDAGRequestObject) (api.RenameDAGResponseObject, error) {
	dag, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.historyManager.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if status.Status == scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is running",
		}
	}

	old, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, fmt.Errorf("error getting the DAG metadata: %w", err)
	}

	if err := a.dagRepository.Rename(ctx, request.FileName, request.Body.NewFileName); err != nil {
		return nil, fmt.Errorf("failed to move DAG: %w", err)
	}

	renamed, err := a.dagRepository.GetMetadata(ctx, request.Body.NewFileName)
	if err != nil {
		return nil, fmt.Errorf("error getting new DAG metadata: %w", err)
	}

	// Rename the history as well
	if err := a.historyManager.Rename(ctx, old.Name, renamed.Name); err != nil {
		return nil, fmt.Errorf("error renaming history: %w", err)
	}

	return api.RenameDAG200Response{}, nil
}

func (a *API) GetDAGRunHistory(ctx context.Context, request api.GetDAGRunHistoryRequestObject) (api.GetDAGRunHistoryResponseObject, error) {
	dag, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	defaultHistoryLimit := 30
	recentHistory := a.historyManager.ListRecentHistory(ctx, dag.Name, defaultHistoryLimit)

	var runs []api.RunDetails
	for _, status := range recentHistory {
		runs = append(runs, toRunDetails(status))
	}

	gridData := a.readHistoryData(ctx, recentHistory)
	return api.GetDAGRunHistory200JSONResponse{
		Runs:     runs,
		GridData: gridData,
	}, nil
}

func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	fileName := request.FileName
	dag, err := a.dagRepository.GetMetadata(ctx, fileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", fileName),
		}
	}

	status, err := a.historyManager.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", fileName),
		}
	}

	details := toDAGDetails(dag)

	return api.GetDAGDetails200JSONResponse{
		Dag:       details,
		LatestRun: toRunDetails(status),
		Suspended: a.dagRepository.IsSuspended(ctx, fileName),
	}, nil
}

func (a *API) readHistoryData(
	_ context.Context,
	statusList []models.Status,
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

	for idx, status := range statusList {
		for _, node := range status.Nodes {
			addStatusFn(data, len(statusList), idx, node.Step.Name, node.Status)
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
	for idx, status := range statusList {
		if n := status.OnSuccess; n != nil {
			addStatusFn(handlers, len(statusList), idx, n.Step.Name, n.Status)
		}
		if n := status.OnFailure; n != nil {
			addStatusFn(handlers, len(statusList), idx, n.Step.Name, n.Status)
		}
		if n := status.OnCancel; n != nil {
			addStatusFn(handlers, len(statusList), idx, n.Step.Name, n.Status)
		}
		if n := status.OnExit; n != nil {
			addStatusFn(handlers, len(statusList), idx, n.Step.Name, n.Status)
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

func (a *API) ListDAGs(ctx context.Context, request api.ListDAGsRequestObject) (api.ListDAGsResponseObject, error) {
	pg := models.NewPaginator(valueOf(request.Params.Page), valueOf(request.Params.PerPage))
	result, errList, err := a.dagRepository.List(ctx, models.ListOptions{
		Paginator: &pg,
		Name:      valueOf(request.Params.Name),
		Tag:       valueOf(request.Params.Tag),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:     errList,
		Pagination: toPagination(result),
	}

	// Get status for each DAG
	dagStatuses := make([]models.Status, len(result.Items))
	for _, item := range result.Items {
		status, err := a.historyManager.GetLatestStatus(ctx, item)
		if err != nil {
			errList = append(errList, err.Error())
		}
		dagStatuses = append(dagStatuses, status)
	}

	for i, item := range result.Items {
		run := api.RunSummary{
			Log:         dagStatuses[i].Log,
			Name:        dagStatuses[i].Name,
			Params:      ptrOf(dagStatuses[i].Params),
			Pid:         ptrOf(int(dagStatuses[i].PID)),
			RequestId:   dagStatuses[i].RequestID,
			StartedAt:   dagStatuses[i].StartedAt,
			FinishedAt:  dagStatuses[i].FinishedAt,
			Status:      api.Status(dagStatuses[i].Status),
			StatusLabel: api.StatusLabel(dagStatuses[i].Status.String()),
		}

		dag := api.DAGFile{
			FileName:  item.FileName(),
			Errors:    errList,
			LatestRun: run,
			Suspended: a.dagRepository.IsSuspended(ctx, item.FileName()),
			Dag:       toDAG(item),
		}

		resp.Dags = append(resp.Dags, dag)
	}

	return resp, nil
}

func (a *API) GetAllDAGTags(ctx context.Context, _ api.GetAllDAGTagsRequestObject) (api.GetAllDAGTagsResponseObject, error) {
	tags, errs, err := a.dagRepository.TagList(ctx)
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

	dag, err := a.dagRepository.GetMetadata(ctx, dagFileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", dagFileName),
		}
	}

	if requestId == "latest" {
		latestStatus, err := a.historyManager.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		return &api.GetDAGRunDetails200JSONResponse{
			Run: toRunDetails(latestStatus),
		}, nil
	}

	status, err := a.historyManager.GetRealtimeStatus(ctx, dag, requestId)
	if err != nil {
		return nil, fmt.Errorf("error getting status by request ID: %w", err)
	}

	return &api.GetDAGRunDetails200JSONResponse{
		Run: toRunDetails(*status),
	}, nil
}

func (a *API) ExecuteDAG(ctx context.Context, request api.ExecuteDAGRequestObject) (api.ExecuteDAGResponseObject, error) {
	dag, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.historyManager.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if status.Status == scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeAlreadyRunning,
			Message:    "DAG is already running",
		}
	}

	requestID, err := a.historyManager.GenerateRequestID(ctx)
	if err != nil {
		return nil, fmt.Errorf("error generating request ID: %w", err)
	}

	if err := a.historyManager.Start(ctx, dag, history.StartOptions{
		Params:    valueOf(request.Body.Params),
		RequestID: requestID,
		Quiet:     true,
	}); err != nil {
		return nil, fmt.Errorf("error starting DAG: %w", err)
	}

	// Wait for the DAG to start
	timer := time.NewTimer(3 * time.Second)
	var running bool
	defer timer.Stop()

waitLoop:
	for {
		select {
		case <-timer.C:
			break waitLoop
		case <-ctx.Done():
			break waitLoop
		default:
			status, _ := a.historyManager.GetRealtimeStatus(ctx, dag, requestID)
			if status == nil {
				continue
			}
			if status.Status != scheduler.StatusNone {
				// If status is not None, it means the DAG has started or even finished
				running = true
				timer.Stop()
				break waitLoop
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	if !running {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "DAG did not start",
		}
	}

	return api.ExecuteDAG200JSONResponse{
		RequestId: requestID,
	}, nil
}

func (a *API) TerminateDAGRun(ctx context.Context, request api.TerminateDAGRunRequestObject) (api.TerminateDAGRunResponseObject, error) {
	dag, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.historyManager.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if status.Status != scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is not running",
		}
	}
	if err := a.historyManager.Stop(ctx, dag, status.RequestID); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}
	return api.TerminateDAGRun200Response{}, nil
}

func (a *API) RetryDAGRun(ctx context.Context, request api.RetryDAGRunRequestObject) (api.RetryDAGRunResponseObject, error) {
	dag, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.historyManager.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if status.Status == scheduler.StatusRunning {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeAlreadyRunning,
			Message:    "DAG is already running",
		}
	}

	if err := a.historyManager.Retry(ctx, dag, request.Body.RequestId); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGRun200Response{}, nil
}

func (a *API) UpdateDAGSuspensionState(ctx context.Context, request api.UpdateDAGSuspensionStateRequestObject) (api.UpdateDAGSuspensionStateResponseObject, error) {
	_, err := a.dagRepository.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if err := a.dagRepository.ToggleSuspend(ctx, request.FileName, request.Body.Suspend); err != nil {
		return nil, fmt.Errorf("error toggling suspend: %w", err)
	}

	return api.UpdateDAGSuspensionState200Response{}, nil
}

func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	ret, errs, err := a.dagRepository.Grep(ctx, request.Params.Q)
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

	return &api.SearchDAGs200JSONResponse{
		Results: results,
		Errors:  errs,
	}, nil
}
