package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	runtime1 "github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// ValidateDAGSpec implements api.StrictServerInterface.
func (a *API) ValidateDAGSpec(ctx context.Context, request api.ValidateDAGSpecRequestObject) (api.ValidateDAGSpecResponseObject, error) {
	// Parse and validate the provided spec without persisting it.
	// Use AllowBuildErrors so we can return partial DAG details alongside errors.
	name := "validated-dag"
	if request.Body != nil && request.Body.Name != nil {
		name = *request.Body.Name
	}

	if request.Body == nil {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "request body is required",
		}
	}

	// Load the DAG spec
	dag, err := spec.LoadYAML(ctx,
		[]byte(request.Body.Spec),
		spec.WithName(name),
		spec.WithAllowBuildErrors(),
		spec.WithoutEval(),
	)

	var errs []string
	var loadErrs core.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else if err != nil {
		// Unexpected fatal error
		return nil, err
	}

	if dag != nil && len(dag.BuildErrors) > 0 {
		for _, e := range dag.BuildErrors {
			errs = append(errs, e.Error())
		}
	}

	// Build response
	details := toDAGDetails(dag)

	return &api.ValidateDAGSpec200JSONResponse{
		Valid:  len(errs) == 0,
		Dag:    details,
		Errors: errs,
	}, nil
}

func (a *API) CreateNewDAG(ctx context.Context, request api.CreateNewDAGRequestObject) (api.CreateNewDAGResponseObject, error) {
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	// Determine spec to create with: provided spec or default template
	var yamlSpec []byte
	if request.Body.Spec != nil && strings.TrimSpace(*request.Body.Spec) != "" {
		// Validate provided spec before creating
		_, err := spec.LoadYAML(ctx,
			[]byte(*request.Body.Spec),
			spec.WithName(request.Body.Name),
			spec.WithoutEval(),
		)

		if err != nil {
			var verrs core.ErrorList
			if errors.As(err, &verrs) {
				// Return 400 with summary of errors
				return nil, &Error{
					HTTPStatus: http.StatusBadRequest,
					Code:       api.ErrorCodeBadRequest,
					Message:    strings.Join(verrs.ToStringList(), "; "),
				}
			}
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
		yamlSpec = []byte(*request.Body.Spec)
	} else {
		// Default minimal spec
		yamlSpec = []byte(`steps:
  - command: echo hello
`)
	}

	if err := a.dagStore.Create(ctx, request.Body.Name, yamlSpec); err != nil {
		if errors.Is(err, exec.ErrDAGAlreadyExists) {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
			}
		}
		return nil, fmt.Errorf("error creating DAG: %w", err)
	}

	// Log DAG creation
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{"dag_name": request.Body.Name})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_create", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return &api.CreateNewDAG201JSONResponse{
		Name: request.Body.Name,
	}, nil
}

func (a *API) DeleteDAG(ctx context.Context, request api.DeleteDAGRequestObject) (api.DeleteDAGResponseObject, error) {
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

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

	// Clean up associated webhook if exists (best effort - don't fail deletion if webhook cleanup fails)
	if a.authService != nil && a.authService.HasWebhookStore() {
		if err := a.authService.DeleteWebhook(ctx, request.FileName); err != nil {
			// Only log if it's not a "not found" error (webhook may not exist)
			if !errors.Is(err, auth.ErrWebhookNotFound) {
				logger.Warn(ctx, "Failed to delete webhook for deleted DAG",
					tag.Name(request.FileName),
					tag.Error(err),
				)
			}
		}
	}

	// Log DAG deletion
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{"dag_name": request.FileName})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_delete", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return &api.DeleteDAG204Response{}, nil
}

func (a *API) GetDAGSpec(ctx context.Context, request api.GetDAGSpecRequestObject) (api.GetDAGSpecResponseObject, error) {
	yamlSpec, err := a.dagStore.GetSpec(ctx, request.FileName)
	if err != nil {
		return nil, err
	}

	// Validate the spec - use WithAllowBuildErrors to return DAG even with errors
	dag, err := spec.LoadYAML(ctx,
		[]byte(yamlSpec),
		spec.WithName(request.FileName),
		spec.WithAllowBuildErrors(),
		spec.WithoutEval(),
	)
	var errs []string

	var loadErrs core.ErrorList
	if errors.As(err, &loadErrs) {
		errs = loadErrs.ToStringList()
	} else if err != nil {
		// If we still get an error with AllowBuildErrors, something is seriously wrong
		return nil, err
	}

	// If dag is still nil (shouldn't happen with AllowBuildErrors), create a minimal DAG
	if dag == nil {
		dag = &core.DAG{
			Name: request.FileName,
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	} else {
		for _, buildErr := range dag.BuildErrors {
			errs = append(errs, buildErr.Error())
		}
		errs = append(errs, dag.BuildWarnings...)
	}

	return &api.GetDAGSpec200JSONResponse{
		Dag:    toDAGDetails(dag),
		Spec:   yamlSpec,
		Errors: errs,
	}, nil
}

func (a *API) UpdateDAGSpec(ctx context.Context, request api.UpdateDAGSpecRequestObject) (api.UpdateDAGSpecResponseObject, error) {
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	err := a.dagStore.UpdateSpec(ctx, request.FileName, []byte(request.Body.Spec))

	var loadErrs core.ErrorList
	var errs []string

	if err != nil {
		if errors.As(err, &loadErrs) {
			errs = loadErrs.ToStringList()
		} else {
			return nil, err
		}
	}

	// Log DAG spec update
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{"dag_name": request.FileName})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_update", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.UpdateDAGSpec200JSONResponse{
		Errors: errs,
	}, nil
}

func (a *API) RenameDAG(ctx context.Context, request api.RenameDAGRequestObject) (api.RenameDAGResponseObject, error) {
	if err := a.requireDAGWrite(ctx); err != nil {
		return nil, err
	}

	if err := core.ValidateDAGName(request.Body.NewFileName); err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    err.Error(),
		}
	}

	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	dagStatus, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if dagStatus.Status == core.Running {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is running",
		}
	}

	if err := a.dagStore.Rename(ctx, request.FileName, request.Body.NewFileName); err != nil {
		return nil, fmt.Errorf("failed to move DAG: %w", err)
	}

	// Log DAG rename
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{
			"old_name": request.FileName,
			"new_name": request.Body.NewFileName,
		})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_rename", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.RenameDAG200Response{}, nil
}

func (a *API) GetDAGDAGRunHistory(ctx context.Context, request api.GetDAGDAGRunHistoryRequestObject) (api.GetDAGDAGRunHistoryResponseObject, error) {
	// Try to get metadata, but if it fails (e.g., due to errors), use the fileName as the DAG name
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	var dagName string
	if err != nil {
		// For DAGs with errors, we can still try to get history using the fileName as the name
		dagName = request.FileName
	} else {
		dagName = dag.Name
	}

	defaultHistoryLimit := 30
	recentHistory := a.dagRunMgr.ListRecentStatus(ctx, dagName, defaultHistoryLimit)

	var dagRuns []api.DAGRunDetails
	for _, status := range recentHistory {
		dagRuns = append(dagRuns, ToDAGRunDetails(status))
	}

	gridData := a.readHistoryData(ctx, recentHistory)
	return api.GetDAGDAGRunHistory200JSONResponse{
		DagRuns:  dagRuns,
		GridData: gridData,
	}, nil
}

func (a *API) GetDAGDetails(ctx context.Context, request api.GetDAGDetailsRequestObject) (api.GetDAGDetailsResponseObject, error) {
	resp, err := a.getDAGDetailsData(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    err.Error(),
		}
	}
	return resp, nil
}

// getDAGDetailsData returns DAG details data. Used by both HTTP handler and SSE fetcher.
func (a *API) getDAGDetailsData(ctx context.Context, fileName string) (api.GetDAGDetails200JSONResponse, error) {
	dag, err := a.dagStore.GetDetails(ctx, fileName, spec.WithAllowBuildErrors())
	if err != nil {
		return api.GetDAGDetails200JSONResponse{}, fmt.Errorf("DAG %s not found", fileName)
	}

	dagStatus, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
	if err != nil && !errors.Is(err, exec.ErrNoStatusData) {
		return api.GetDAGDetails200JSONResponse{}, fmt.Errorf("failed to get latest status for DAG %s", fileName)
	}
	// If ErrNoStatusData, dagStatus will be zero-value (empty), which is fine for DAGs with no runs

	details := toDAGDetails(dag)

	localDAGs := make([]api.LocalDag, 0, len(dag.LocalDAGs))
	for _, localDAG := range dag.LocalDAGs {
		localDAGs = append(localDAGs, toLocalDAG(localDAG))
	}

	sort.Slice(localDAGs, func(i, j int) bool {
		return localDAGs[i].Name < localDAGs[j].Name
	})

	return api.GetDAGDetails200JSONResponse{
		Dag:          details,
		LatestDAGRun: ToDAGRunDetails(dagStatus),
		Suspended:    a.dagStore.IsSuspended(ctx, fileName),
		LocalDags:    localDAGs,
		Errors:       extractBuildErrors(dag.BuildErrors),
	}, nil
}

// extractBuildErrors converts a slice of errors to a slice of strings.
func extractBuildErrors(errs []error) []string {
	result := make([]string, 0, len(errs))
	for _, e := range errs {
		result = append(result, e.Error())
	}
	return result
}

func (a *API) readHistoryData(
	_ context.Context,
	statusList []exec.DAGRunStatus,
) []api.DAGGridItem {
	data := map[string][]core.NodeStatus{}

	addStatusFn := func(
		data map[string][]core.NodeStatus,
		logLen int,
		logIdx int,
		nodeName string,
		nodeStatus core.NodeStatus,
	) {
		if _, ok := data[nodeName]; !ok {
			data[nodeName] = make([]core.NodeStatus, logLen)
		}
		data[nodeName][logIdx] = nodeStatus
	}

	for idx, st := range statusList {
		for _, node := range st.Nodes {
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

	handlers := map[string][]core.NodeStatus{}
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

	for _, handlerType := range []core.HandlerType{
		core.HandlerOnSuccess,
		core.HandlerOnFailure,
		core.HandlerOnCancel,
		core.HandlerOnExit,
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
	// Extract sort and order parameters with config defaults
	sortField := a.config.UI.DAGs.SortField
	if sortField == "" {
		sortField = "name" // fallback if config is empty
	}
	if request.Params.Sort != nil {
		sortField = string(*request.Params.Sort)
	}

	sortOrder := a.config.UI.DAGs.SortOrder
	if sortOrder == "" {
		sortOrder = "asc" // fallback if config is empty
	}
	if request.Params.Order != nil {
		sortOrder = string(*request.Params.Order)
	}

	// Use paginator from request
	pg := exec.NewPaginator(valueOf(request.Params.Page), valueOf(request.Params.PerPage))

	// Parse comma-separated tags parameter
	tags := parseCommaSeparatedTags(request.Params.Tags)

	// Let persistence layer handle sorting and pagination
	result, errList, err := a.dagStore.List(ctx, exec.ListDAGsOptions{
		Paginator: &pg,
		Name:      valueOf(request.Params.Name),
		Tags:      tags,
		Sort:      sortField,
		Order:     sortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	// Build DAG files for the paginated results
	dagFiles := make([]api.DAGFile, 0, len(result.Items))
	for _, item := range result.Items {
		dagStatus, err := a.dagRunMgr.GetLatestStatus(ctx, item)
		if err != nil {
			errList = append(errList, err.Error())
		}

		suspended := a.dagStore.IsSuspended(ctx, item.FileName())
		dagRun := toDAGRunSummary(dagStatus)

		// Include any build errors from the DAG
		var dagErrors []string
		if item.BuildErrors != nil {
			for _, err := range item.BuildErrors {
				dagErrors = append(dagErrors, err.Error())
			}
		}

		dagFile := api.DAGFile{
			FileName:     item.FileName(),
			LatestDAGRun: dagRun,
			Suspended:    suspended,
			Dag:          toDAG(item),
			Errors:       dagErrors,
		}

		dagFiles = append(dagFiles, dagFile)
	}

	resp := &api.ListDAGs200JSONResponse{
		Dags:       dagFiles,
		Errors:     errList,
		Pagination: toPagination(result),
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

	// Try to get metadata first
	dag, err := a.dagStore.GetMetadata(ctx, dagFileName)
	if err != nil {
		// For DAGs with errors, try to load with AllowBuildErrors
		dag, err = a.dagStore.GetDetails(ctx, dagFileName, spec.WithAllowBuildErrors())
		if err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("DAG %s not found", dagFileName),
			}
		}
	}

	if dagRunId == "latest" {
		latestStatus, err := a.dagRunMgr.GetLatestStatus(ctx, dag)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		return &api.GetDAGDAGRunDetails200JSONResponse{
			DagRun: ToDAGRunDetails(latestStatus),
		}, nil
	}

	dagStatus, err := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRunId)
	if err != nil {
		if errors.Is(err, exec.ErrNoStatusData) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("DAG run %s not found", dagRunId),
			}
		}
		return nil, fmt.Errorf("error getting status by dag-run ID: %w", err)
	}

	return &api.GetDAGDAGRunDetails200JSONResponse{
		DagRun: ToDAGRunDetails(*dagStatus),
	}, nil
}

func (a *API) ExecuteDAG(ctx context.Context, request api.ExecuteDAGRequestObject) (api.ExecuteDAGResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	dag, err := a.dagStore.GetDetails(ctx, request.FileName, spec.WithAllowBuildErrors())
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if err := buildErrorsToAPIError(dag.BuildErrors); err != nil {
		return nil, err
	}

	var dagRunId, params string
	var singleton bool
	var nameOverride string

	if request.Body != nil {
		dagRunId = valueOf(request.Body.DagRunId)
		params = valueOf(request.Body.Params)
		singleton = valueOf(request.Body.Singleton)
		nameOverride = strings.TrimSpace(valueOf(request.Body.DagName))
	}

	if nameOverride != "" {
		if err := core.ValidateDAGName(nameOverride); err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
		dag.Name = nameOverride
	}

	if dagRunId == "" {
		var err error
		dagRunId, err = a.dagRunMgr.GenDAGRunID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", err)
		}
	}

	if singleton {
		alive, err := a.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check singleton execution status: %w", err)
		}
		if alive > 0 {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
				Message:    fmt.Sprintf("DAG %s is already running (singleton mode)", dag.Name),
			}
		}
	}

	if err := a.ensureDAGRunIDUnique(ctx, dag, dagRunId); err != nil {
		return nil, err
	}

	if err := a.startDAGRun(ctx, dag, params, dagRunId, nameOverride, singleton); err != nil {
		return nil, fmt.Errorf("error starting dag-run: %w", err)
	}

	// Log DAG execution
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]any{
			"dag_name":   request.FileName,
			"dag_run_id": dagRunId,
		}
		if params != "" {
			detailsMap["params"] = params
		}
		details, _ := json.Marshal(detailsMap)
		entry := audit.NewEntry(audit.CategoryDAG, "dag_execute", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.ExecuteDAG200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

// ExecuteDAGSync executes a DAG and waits for completion before returning.
// It returns the full DAGRunDetails including all node statuses.
func (a *API) ExecuteDAGSync(ctx context.Context, request api.ExecuteDAGSyncRequestObject) (api.ExecuteDAGSyncResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	if request.Body == nil {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "request body is required",
		}
	}

	dag, err := a.dagStore.GetDetails(ctx, request.FileName, spec.WithAllowBuildErrors())
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if err := buildErrorsToAPIError(dag.BuildErrors); err != nil {
		return nil, err
	}

	dagRunId := valueOf(request.Body.DagRunId)
	params := valueOf(request.Body.Params)
	singleton := valueOf(request.Body.Singleton)
	nameOverride := strings.TrimSpace(valueOf(request.Body.DagName))
	timeout := request.Body.Timeout

	if nameOverride != "" {
		if err := core.ValidateDAGName(nameOverride); err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
		dag.Name = nameOverride
	}

	if dagRunId == "" {
		var err error
		dagRunId, err = a.dagRunMgr.GenDAGRunID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", err)
		}
	}

	if singleton {
		alive, err := a.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check singleton execution status: %w", err)
		}
		if alive > 0 {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
				Message:    fmt.Sprintf("DAG %s is already running (singleton mode)", dag.Name),
			}
		}
	}

	if err := a.ensureDAGRunIDUnique(ctx, dag, dagRunId); err != nil {
		return nil, err
	}

	// Start the DAG run
	if err := a.startDAGRun(ctx, dag, params, dagRunId, nameOverride, singleton); err != nil {
		return nil, fmt.Errorf("error starting dag-run: %w", err)
	}

	// Wait for DAG completion
	dagStatus, err := a.waitForDAGCompletion(ctx, dag, dagRunId, timeout)
	if err != nil {
		// Check if it's a timeout error
		if errors.Is(err, context.DeadlineExceeded) {
			return api.ExecuteDAGSync408JSONResponse{
				Code:     api.ErrorCodeTimeout,
				Message:  fmt.Sprintf("timeout waiting for DAG %s to complete after %d seconds; DAG run continues in background", dag.Name, timeout),
				DagRunId: dagRunId,
			}, nil
		}
		return nil, err
	}

	return api.ExecuteDAGSync200JSONResponse{
		DagRun: ToDAGRunDetails(*dagStatus),
	}, nil
}

// waitForDAGCompletion polls the DAG status until it reaches a terminal state or timeout.
// It returns the final DAGRunStatus or an error if timeout is exceeded.
func (a *API) waitForDAGCompletion(
	ctx context.Context,
	dag *core.DAG,
	dagRunId string,
	timeoutSeconds int,
) (*exec.DAGRunStatus, error) {
	// Create context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Adaptive polling: start at 100ms, increase to max 2s
	pollInterval := 100 * time.Millisecond
	maxPollInterval := 2 * time.Second

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastStatus *exec.DAGRunStatus

	for {
		select {
		case <-waitCtx.Done():
			// Timeout or context cancelled
			return lastStatus, waitCtx.Err()

		case <-ticker.C:
			status, err := a.dagRunMgr.GetCurrentStatus(waitCtx, dag, dagRunId)
			if err != nil {
				// Log error but continue polling - DAG might still be initializing
				logger.Debug(waitCtx, "Error getting DAG status during wait", tag.Error(err))
				continue
			}

			if status == nil {
				continue
			}

			lastStatus = status

			// Check if execution is complete (not active) or waiting for human approval.
			// We return on "waiting" status because HITL (Human-In-The-Loop) workflows
			// require external intervention that would cause indefinite blocking otherwise.
			// The client can poll the status endpoint or use callbacks to resume monitoring.
			if !status.Status.IsActive() || status.Status.IsWaiting() {
				return status, nil
			}

			// Adaptive polling: increase interval up to max
			if pollInterval < maxPollInterval {
				pollInterval = min(time.Duration(float64(pollInterval)*1.5), maxPollInterval)
				ticker.Reset(pollInterval)
			}
		}
	}
}

func (a *API) startDAGRun(ctx context.Context, dag *core.DAG, params, dagRunID, nameOverride string, singleton bool) error {
	return a.startDAGRunWithOptions(ctx, dag, startDAGRunOptions{
		params:       params,
		dagRunID:     dagRunID,
		nameOverride: nameOverride,
		singleton:    singleton,
		triggerType:  core.TriggerTypeManual,
	})
}

// buildErrorsToAPIError returns an API error if the DAG has build errors, nil otherwise.
func buildErrorsToAPIError(buildErrors []error) *Error {
	if len(buildErrors) == 0 {
		return nil
	}
	var errMessages []string
	for _, buildErr := range buildErrors {
		errMessages = append(errMessages, buildErr.Error())
	}
	return &Error{
		HTTPStatus: http.StatusBadRequest,
		Code:       api.ErrorCodeBadRequest,
		Message:    strings.Join(errMessages, "; "),
	}
}

// ensureDAGRunIDUnique validates that the given dagRunID is not already in use for this DAG.
func (a *API) ensureDAGRunIDUnique(ctx context.Context, dag *core.DAG, dagRunID string) error {
	if dagRunID == "" {
		return fmt.Errorf("dagRunID must be non-empty")
	}
	if _, err := a.dagRunStore.FindAttempt(ctx, exec.NewDAGRunRef(dag.Name, dagRunID)); err == nil {
		return &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("dag-run ID %s already exists for DAG %s", dagRunID, dag.Name),
		}
	} else if !errors.Is(err, exec.ErrDAGRunIDNotFound) {
		return fmt.Errorf("failed to verify dag-run ID uniqueness: %w", err)
	}
	return nil
}

type startDAGRunOptions struct {
	params       string
	dagRunID     string
	nameOverride string
	singleton    bool
	fromRunID    string
	target       string
	triggerType  core.TriggerType
}

func (a *API) startDAGRunWithOptions(ctx context.Context, dag *core.DAG, opts startDAGRunOptions) error {
	spec := a.subCmdBuilder.Start(dag, runtime1.StartOptions{
		Params:       opts.params,
		DAGRunID:     opts.dagRunID,
		Quiet:        true,
		NameOverride: opts.nameOverride,
		FromRunID:    opts.fromRunID,
		Target:       opts.target,
		TriggerType:  opts.triggerType.String(),
	})

	if err := runtime1.Start(ctx, spec); err != nil {
		return fmt.Errorf("error starting DAG: %w", err)
	}

	// Wait for the DAG to start
	// Use longer timeout on Windows due to slower process startup
	timeout := 5 * time.Second // default timeout
	if runtime.GOOS == "windows" {
		timeout = 10 * time.Second
	}
	timer := time.NewTimer(timeout)
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
			dagStatus, _ := a.dagRunMgr.GetCurrentStatus(ctx, dag, opts.dagRunID)
			if dagStatus == nil {
				continue
			}
			if dagStatus.Status != core.NotStarted {
				// If status is not NotStarted, it means the DAG has started or even finished
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
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	dag, err := a.dagStore.GetDetails(ctx, request.FileName, spec.WithAllowBuildErrors(), spec.WithoutEval())
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	if err := buildErrorsToAPIError(dag.BuildErrors); err != nil {
		return nil, err
	}

	// Apply queue override if provided
	if request.Body != nil && request.Body.Queue != nil && *request.Body.Queue != "" {
		dag.Queue = *request.Body.Queue
	}

	var nameOverride string
	if request.Body != nil {
		nameOverride = strings.TrimSpace(valueOf(request.Body.DagName))
	}
	if nameOverride != "" {
		if err := core.ValidateDAGName(nameOverride); err != nil {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
		dag.Name = nameOverride
	}

	dagRunId := valueOf(request.Body.DagRunId)
	if dagRunId == "" {
		var err error
		dagRunId, err = a.dagRunMgr.GenDAGRunID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", err)
		}
	}

	singleton := valueOf(request.Body.Singleton)
	if singleton {
		// Check if running
		alive, err := a.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check singleton execution status (proc): %w", err)
		}
		if alive > 0 {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
				Message:    fmt.Sprintf("DAG %s is already running (singleton mode)", dag.Name),
			}
		}

		// Check if queued
		queued, err := a.queueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check singleton execution status (queue): %w", err)
		}
		if len(queued) > 0 {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
				Message:    fmt.Sprintf("DAG %s is already in queue (singleton mode)", dag.Name),
			}
		}
	}

	if err := a.enqueueDAGRun(ctx, dag, valueOf(request.Body.Params), dagRunId, nameOverride, core.TriggerTypeManual); err != nil {
		return nil, fmt.Errorf("error enqueuing dag-run: %w", err)
	}

	// Log DAG enqueue
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]any{
			"dag_name":   request.FileName,
			"dag_run_id": dagRunId,
		}
		if request.Body.Params != nil && *request.Body.Params != "" {
			detailsMap["params"] = *request.Body.Params
		}
		details, _ := json.Marshal(detailsMap)
		entry := audit.NewEntry(audit.CategoryDAG, "dag_enqueue", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.EnqueueDAGDAGRun200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

func (a *API) enqueueDAGRun(ctx context.Context, dag *core.DAG, params, dagRunID, nameOverride string, triggerType core.TriggerType) error {
	opts := runtime1.EnqueueOptions{
		Params:       params,
		DAGRunID:     dagRunID,
		NameOverride: nameOverride,
		TriggerType:  triggerType.String(),
	}
	if dag.Queue != "" {
		opts.Queue = dag.Queue
	}

	spec := a.subCmdBuilder.Enqueue(dag, opts)
	if err := runtime1.Run(ctx, spec); err != nil {
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
			dagStatus, _ := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
			if dagStatus == nil {
				continue
			}
			if dagStatus.Status != core.NotStarted {
				// If status is not NotStarted, it means the DAG has started or even finished
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

func (a *API) UpdateDAGSuspensionState(ctx context.Context, request api.UpdateDAGSuspensionStateRequestObject) (api.UpdateDAGSuspensionStateResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

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

	// Log DAG suspension state change
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		action := "dag_suspend"
		if !request.Body.Suspend {
			action = "dag_resume"
		}
		details, _ := json.Marshal(map[string]any{
			"dag_name":  request.FileName,
			"suspended": request.Body.Suspend,
		})
		entry := audit.NewEntry(audit.CategoryDAG, action, currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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

func (a *API) StopAllDAGRuns(ctx context.Context, request api.StopAllDAGRunsRequestObject) (api.StopAllDAGRunsResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	// Get the DAG metadata to ensure it exists
	dag, err := a.dagStore.GetMetadata(ctx, request.FileName)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	// Get all running DAG-runs for this DAG
	runningStatuses, err := a.dagRunStore.ListStatuses(ctx,
		exec.WithExactName(dag.Name),
		exec.WithStatuses([]core.Status{core.Running}),
	)
	if err != nil {
		return nil, fmt.Errorf("error listing running DAG-runs: %w", err)
	}

	// Stop each running DAG-run
	var errors []string
	var stoppedRunIDs []string
	for _, runningStatus := range runningStatuses {
		runID := runningStatus.DAGRunID
		err := a.dagRunMgr.Stop(ctx, dag, runID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to stop run %q: %s", runID, err))
		} else {
			stoppedRunIDs = append(stoppedRunIDs, runID)
		}
		if ctx.Err() != nil {
			errors = append(errors, fmt.Sprintf("context is cancelled: %s", err))
			break
		}
	}

	// Log stop all DAG runs
	if a.auditService != nil && len(stoppedRunIDs) > 0 {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]any{
			"dag_name":        request.FileName,
			"stopped_run_ids": stoppedRunIDs,
			"count":           len(stoppedRunIDs),
		})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_stop_all", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return &api.StopAllDAGRuns200JSONResponse{
		Errors: errors,
	}, nil
}

// SSE Data Methods for DAGs

// GetDAGDetailsData returns DAG details for SSE.
// Identifier format: "fileName"
func (a *API) GetDAGDetailsData(ctx context.Context, fileName string) (any, error) {
	return a.getDAGDetailsData(ctx, fileName)
}

// GetDAGHistoryData returns DAG execution history for SSE.
// Identifier format: "fileName"
func (a *API) GetDAGHistoryData(ctx context.Context, fileName string) (any, error) {
	dag, err := a.dagStore.GetMetadata(ctx, fileName)
	var dagName string
	if err != nil {
		dagName = fileName
	} else {
		dagName = dag.Name
	}

	defaultHistoryLimit := 30
	recentHistory := a.dagRunMgr.ListRecentStatus(ctx, dagName, defaultHistoryLimit)

	var dagRuns []api.DAGRunDetails
	for _, status := range recentHistory {
		dagRuns = append(dagRuns, ToDAGRunDetails(status))
	}

	gridData := a.readHistoryData(ctx, recentHistory)
	return api.GetDAGDAGRunHistory200JSONResponse{
		DagRuns:  dagRuns,
		GridData: gridData,
	}, nil
}

// GetDAGsListData returns DAGs list for SSE.
// Identifier format: URL query string (e.g., "page=1&perPage=100&name=mydag")
func (a *API) GetDAGsListData(ctx context.Context, queryString string) (any, error) {
	params, err := url.ParseQuery(queryString)
	if err != nil {
		logger.Warn(ctx, "Failed to parse query string for DAGs list",
			tag.Error(err),
			slog.String("queryString", queryString),
		)
	}

	page := parseIntParam(params.Get("page"), 1)
	perPage := parseIntParam(params.Get("perPage"), 100)

	sortField := params.Get("sort")
	if sortField == "" {
		sortField = "name"
	}
	sortOrder := params.Get("order")
	if sortOrder == "" {
		sortOrder = "asc"
	}

	var tags []string
	if tagsParam := params.Get("tags"); tagsParam != "" {
		tags = parseCommaSeparatedTags(&tagsParam)
	}

	pg := exec.NewPaginator(page, perPage)
	listOpts := exec.ListDAGsOptions{
		Paginator: &pg,
		Name:      params.Get("name"),
		Tags:      tags,
		Sort:      sortField,
		Order:     sortOrder,
	}

	result, errList, err := a.dagStore.List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	dagFiles := make([]api.DAGFile, 0, len(result.Items))
	for _, item := range result.Items {
		dagStatus, statusErr := a.dagRunMgr.GetLatestStatus(ctx, item)
		dagFile := api.DAGFile{
			FileName:     item.FileName(),
			LatestDAGRun: toDAGRunSummary(dagStatus),
			Suspended:    a.dagStore.IsSuspended(ctx, item.FileName()),
			Dag:          toDAG(item),
			Errors:       extractBuildErrors(item.BuildErrors),
		}
		if statusErr != nil {
			errList = append(errList, statusErr.Error())
		}
		dagFiles = append(dagFiles, dagFile)
	}

	return api.ListDAGs200JSONResponse{
		Dags:       dagFiles,
		Errors:     errList,
		Pagination: toPagination(result),
	}, nil
}

// parseIntParam parses an integer string, returning defaultVal if parsing fails or value is <= 0.
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return defaultVal
}
