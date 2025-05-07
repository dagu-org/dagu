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
	"github.com/dagu-org/dagu/internal/dagstore"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/history"
)

func (a *API) CreateNewDAG(ctx context.Context, request api.CreateNewDAGRequestObject) (api.CreateNewDAGResponseObject, error) {
	name, err := a.dagClient.Create(ctx, request.Body.Name)
	if err != nil {
		if errors.Is(err, dagstore.ErrDAGAlreadyExists) {
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

func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	_, err := a.dagClient.Status(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if err := a.dagClient.Delete(ctx, request.FileName); err != nil {
		return nil, fmt.Errorf("error deleting DAG: %w", err)
	}
	return &api.DeleteDAG204Response{}, nil
}

func (a *API) GetDAGSpec(ctx context.Context, request api.GetDAGSpecRequestObject) (api.GetDAGSpecResponseObject, error) {
	spec, err := a.dagClient.GetSpec(ctx, request.FileName)
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
	_, err := a.dagClient.Status(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	err = a.dagClient.UpdateSpec(ctx, request.FileName, []byte(request.Body.Spec))
	var errs []string

	var loadErrs digraph.ErrorList
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
	status, err := a.dagClient.Status(ctx, request.FileName)
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
	if err := a.dagClient.Move(ctx, request.FileName, request.Body.NewFileName); err != nil {
		return nil, fmt.Errorf("failed to move DAG: %w", err)
	}
	return api.RenameDAG200Response{}, nil
}

func (a *API) GetDAGRunHistory(ctx context.Context, request api.GetDAGRunHistoryRequestObject) (api.GetDAGRunHistoryResponseObject, error) {
	status, err := a.dagClient.Status(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	defaultHistoryLimit := 30
	recentHistory := a.historyManager.ListRecentHistory(ctx, status.DAG.Name, defaultHistoryLimit)

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
	status, err := a.dagClient.Status(ctx, fileName)
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
	statusList []history.Status,
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
	var opts []dagstore.ListDAGOption
	if request.Params.PerPage != nil {
		opts = append(opts, dagstore.WithLimit(*request.Params.PerPage))
	}
	if request.Params.Page != nil {
		opts = append(opts, dagstore.WithPage(*request.Params.Page))
	}
	if request.Params.Name != nil {
		opts = append(opts, dagstore.WithName(*request.Params.Name))
	}
	if request.Params.Tag != nil {
		opts = append(opts, dagstore.WithTag(*request.Params.Tag))
	}

	result, errList, err := a.dagClient.List(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	resp := &api.ListDAGs200JSONResponse{
		Errors:     errList,
		Pagination: toPagination(*result),
	}

	for _, item := range result.Items {
		run := api.RunSummary{
			Log:         item.Status.Log,
			Name:        item.Status.Name,
			Params:      ptrOf(item.Status.Params),
			Pid:         ptrOf(int(item.Status.PID)),
			RequestId:   item.Status.RequestID,
			StartedAt:   item.Status.StartedAt,
			FinishedAt:  item.Status.FinishedAt,
			Status:      api.Status(item.Status.Status),
			StatusLabel: api.StatusLabel(item.Status.Status.String()),
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
	tags, errs, err := a.dagClient.TagList(ctx)
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

	dagWithStatus, err := a.dagClient.Status(ctx, dagFileName)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	if requestId == "latest" {
		return &api.GetDAGRunDetails200JSONResponse{
			Run: toRunDetails(dagWithStatus.Status),
		}, nil
	}

	status, err := a.historyManager.GetRealtimeStatus(ctx, dagWithStatus.DAG, requestId)
	if err != nil {
		return nil, fmt.Errorf("error getting status by request ID: %w", err)
	}

	return &api.GetDAGRunDetails200JSONResponse{
		Run: toRunDetails(*status),
	}, nil
}

func (a *API) ExecuteDAG(ctx context.Context, request api.ExecuteDAGRequestObject) (api.ExecuteDAGResponseObject, error) {
	status, err := a.dagClient.Status(ctx, request.FileName)
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

	requestID, err := a.historyManager.GenerateRequestID(ctx)
	if err != nil {
		return nil, fmt.Errorf("error generating request ID: %w", err)
	}

	if err := a.historyManager.Start(ctx, status.DAG, history.StartOptions{
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
			status, _ := a.historyManager.GetRealtimeStatus(ctx, status.DAG, requestID)
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
	status, err := a.dagClient.Status(ctx, request.FileName)
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
	if err := a.historyManager.Stop(ctx, status.DAG, status.Status.RequestID); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}
	return api.TerminateDAGRun200Response{}, nil
}

func (a *API) RetryDAGRun(ctx context.Context, request api.RetryDAGRunRequestObject) (api.RetryDAGRunResponseObject, error) {
	status, err := a.dagClient.Status(ctx, request.FileName)
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

	if err := a.historyManager.Retry(ctx, status.DAG, request.Body.RequestId); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGRun200Response{}, nil
}

func (a *API) UpdateDAGSuspensionState(ctx context.Context, request api.UpdateDAGSuspensionStateRequestObject) (api.UpdateDAGSuspensionStateResponseObject, error) {
	_, err := a.dagClient.Status(ctx, request.FileName)
	if err != nil {
		return &api.UpdateDAGSuspensionState404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("DAG %s not found", request.FileName),
		}, nil
	}

	if err := a.dagClient.ToggleSuspend(ctx, request.FileName, request.Body.Suspend); err != nil {
		return nil, fmt.Errorf("error toggling suspend: %w", err)
	}

	return api.UpdateDAGSuspensionState200Response{}, nil
}

func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	ret, errs, err := a.dagClient.Grep(ctx, request.Params.Q)
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
