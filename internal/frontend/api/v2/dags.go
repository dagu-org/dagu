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
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
)

func (a *API) CreateNewDAG(ctx context.Context, request api.CreateNewDAGRequestObject) (api.CreateNewDAGResponseObject, error) {
	spec := []byte(`steps:
  - name: step1
    command: echo hello
`)

	if err := a.dagStore.Create(ctx, request.Body.Name, spec); err != nil {
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
	_, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}
	if err := a.dagStore.Delete(ctx, request.FileName); err != nil {
		return nil, fmt.Errorf("error deleting DAG: %w", err)
	}
	return &api.DeleteDAG204Response{}, nil
}

func (a *API) GetDAGSpec(ctx context.Context, request api.GetDAGSpecRequestObject) (api.GetDAGSpecResponseObject, error) {
	spec, err := a.dagStore.GetSpec(ctx, request.FileName)
	if err != nil {
		return nil, err
	}

	// Validate the spec
	dag, err := a.dagRunMgr.LoadYAML(ctx, []byte(spec), digraph.WithName(request.FileName))
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
	err := a.dagStore.UpdateSpec(ctx, request.FileName, []byte(request.Body.Spec))

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
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
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

	old, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, fmt.Errorf("error getting the DAG metadata: %w", err)
	}

	if err := a.dagStore.Rename(ctx, request.FileName, request.Body.NewFileName); err != nil {
		return nil, fmt.Errorf("failed to move DAG: %w", err)
	}

	renamed, err := a.dagStore.GetMetadata(ctx, request.Body.NewFileName)
	if err != nil {
		return nil, fmt.Errorf("error getting new DAG metadata: %w", err)
	}

	// Rename the history as well
	if err := a.dagRunStore.RenameDAGRuns(ctx, old.Name, renamed.Name); err != nil {
		return nil, fmt.Errorf("error renaming history: %w", err)
	}

	return api.RenameDAG200Response{}, nil
}

func (a *API) GetDAGDAGRunHistory(ctx context.Context, request api.GetDAGDAGRunHistoryRequestObject) (api.GetDAGDAGRunHistoryResponseObject, error) {
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	defaultHistoryLimit := 30
	recentHistory := a.dagRunMgr.ListRecentStatus(ctx, dag.Name, defaultHistoryLimit)

	var dagRuns []api.DAGRunDetails
	for _, status := range recentHistory {
		dagRuns = append(dagRuns, toDAGRunDetails(status))
	}

	gridData := a.readHistoryData(ctx, recentHistory)
	return api.GetDAGDAGRunHistory200JSONResponse{
		DagRuns:  dagRuns,
		GridData: gridData,
	}, nil
}

func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	fileName := request.FileName
	dag, err := a.dagStore.GetDetails(ctx, fileName, digraph.WithAllowBuildErrors())
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", fileName),
		}
	}

	status, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", fileName),
		}
	}

	details := toDAGDetails(dag)

	return api.GetDAGDetails200JSONResponse{
		Dag:          details,
		LatestDAGRun: toDAGRunDetails(status),
		Suspended:    a.dagStore.IsSuspended(ctx, fileName),
	}, nil
}

func (a *API) readHistoryData(
	_ context.Context,
	statusList []models.DAGRunStatus,
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
	result, errList, err := a.dagStore.List(ctx, models.ListDAGsOptions{
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
	dagStatuses := make([]models.DAGRunStatus, len(result.Items))
	for i, item := range result.Items {
		status, err := a.dagRunMgr.GetLatestStatus(ctx, item)
		if err != nil {
			errList = append(errList, err.Error())
		}
		dagStatuses[i] = status
	}

	for i, item := range result.Items {
		dagRun := toDAGRunSummary(dagStatuses[i])
		dag := api.DAGFile{
			FileName:     item.FileName(),
			Errors:       errList,
			LatestDAGRun: dagRun,
			Suspended:    a.dagStore.IsSuspended(ctx, item.FileName()),
			Dag:          toDAG(item),
		}

		resp.Dags = append(resp.Dags, dag)
	}

	return resp, nil
}

func (a *API) GetAllDAGTags(ctx context.Context, _ api.GetAllDAGTagsRequestObject) (api.GetAllDAGTagsResponseObject, error) {
	tags, errs, err := a.dagStore.TagList(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting tags: %w", err)
	}
	return &api.GetAllDAGTags200JSONResponse{
		Tags:   tags,
		Errors: errs,
	}, nil
}

func (a *API) GetDAGDAGRunDetails(ctx context.Context, request api.GetDAGDAGRunDetailsRequestObject) (api.GetDAGDAGRunDetailsResponseObject, error) {
	dagFileName := request.FileName
	dagRunId := request.DagRunId

	dag, err := a.dagStore.GetMetadata(ctx, dagFileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", dagFileName),
		}
	}

	if dagRunId == "latest" {
		latestStatus, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		return &api.GetDAGDAGRunDetails200JSONResponse{
			DagRun: toDAGRunDetails(latestStatus),
		}, nil
	}

	status, err := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRunId)
	if err != nil {
		return nil, fmt.Errorf("error getting status by dag-run ID: %w", err)
	}

	return &api.GetDAGDAGRunDetails200JSONResponse{
		DagRun: toDAGRunDetails(*status),
	}, nil
}

func (a *API) ExecuteDAG(ctx context.Context, request api.ExecuteDAGRequestObject) (api.ExecuteDAGResponseObject, error) {
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
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

	dagRunId := valueOf(request.Body.DagRunId)
	if dagRunId == "" {
		var err error
		dagRunId, err = a.dagRunMgr.GenDAGRunID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", err)
		}
	}

	// Check the dag-run ID is not already in use
	_, err = a.dagRunStore.FindAttempt(ctx, digraph.DAGRunRef{
		Name: dag.Name,
		ID:   dagRunId,
	})
	if !errors.Is(err, models.ErrDAGRunIDNotFound) {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("dag-run ID %s already exists for DAG %s", dagRunId, dag.Name),
		}
	}

	if err := a.startDAGRun(ctx, dag, valueOf(request.Body.Params), dagRunId); err != nil {
		return nil, fmt.Errorf("error starting dag-run: %w", err)
	}

	return api.ExecuteDAG200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

func (a *API) startDAGRun(ctx context.Context, dag *digraph.DAG, params, dagRunID string) error {
	if err := a.dagRunMgr.StartDAGRun(ctx, dag, dagrun.StartOptions{
		Params:   params,
		DAGRunID: dagRunID,
		Quiet:    true,
	}); err != nil {
		return fmt.Errorf("error starting DAG: %w", err)
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
			status, _ := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
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
		return &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "DAG did not start",
		}
	}

	return nil
}

func (a *API) EnqueueDAGDAGRun(ctx context.Context, request api.EnqueueDAGDAGRunRequestObject) (api.EnqueueDAGDAGRunResponseObject, error) {
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	dagRunId := valueOf(request.Body.DagRunId)
	if dagRunId == "" {
		var err error
		dagRunId, err = a.dagRunMgr.GenDAGRunID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", err)
		}
	}

	if err := a.enqueueDAGRun(ctx, dag, valueOf(request.Body.Params), dagRunId); err != nil {
		return nil, fmt.Errorf("error enqueuing dag-run: %w", err)
	}

	return api.EnqueueDAGDAGRun200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

func (a *API) enqueueDAGRun(ctx context.Context, dag *digraph.DAG, params, dagRunID string) error {
	if err := a.dagRunMgr.EnqueueDAGRun(ctx, dag, dagrun.EnqueueOptions{
		Params:   params,
		DAGRunID: dagRunID,
	}); err != nil {
		return fmt.Errorf("error enqueuing DAG: %w", err)
	}

	// Wait for the DAG to be enqueued
	timer := time.NewTimer(3 * time.Second)
	var ok bool
	defer timer.Stop()

waitLoop:
	for {
		select {
		case <-timer.C:
			break waitLoop
		case <-ctx.Done():
			break waitLoop
		default:
			status, _ := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
			if status == nil {
				continue
			}
			if status.Status != scheduler.StatusNone {
				// If status is not None, it means the DAG has started or even finished
				ok = true
				timer.Stop()
				break waitLoop
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	if !ok {
		return &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to enqueue dagRun execution",
		}
	}

	return nil
}

func (a *API) TerminateDAGDAGRun(ctx context.Context, request api.TerminateDAGDAGRunRequestObject) (api.TerminateDAGDAGRunResponseObject, error) {
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
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
	if err := a.dagRunMgr.Stop(ctx, dag, status.DAGRunID); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}
	return api.TerminateDAGDAGRun200Response{}, nil
}

func (a *API) RetryDAGDAGRun(ctx context.Context, request api.RetryDAGDAGRunRequestObject) (api.RetryDAGDAGRunResponseObject, error) {
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	status, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
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

	if err := a.dagRunMgr.RetryDAGRun(ctx, dag, request.Body.DagRunId); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGDAGRun200Response{}, nil
}

func (a *API) UpdateDAGSuspensionState(ctx context.Context, request api.UpdateDAGSuspensionStateRequestObject) (api.UpdateDAGSuspensionStateResponseObject, error) {
	_, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if err := a.dagStore.ToggleSuspend(ctx, request.FileName, request.Body.Suspend); err != nil {
		return nil, fmt.Errorf("error toggling suspend: %w", err)
	}

	return api.UpdateDAGSuspensionState200Response{}, nil
}

func (a *API) SearchDAGs(ctx context.Context, request api.SearchDAGsRequestObject) (api.SearchDAGsResponseObject, error) {
	ret, errs, err := a.dagStore.Grep(ctx, request.Params.Q)
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
