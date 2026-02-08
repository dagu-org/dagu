package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/persis/filedag"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/dagu-org/dagu/internal/persis/filequeue"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// namespaceScopedStores holds namespace-scoped store instances for a single request.
type namespaceScopedStores struct {
	namespace     *exec.Namespace
	dagStore      exec.DAGStore
	dagRunStore   exec.DAGRunStore
	procStore     exec.ProcStore
	queueStore    exec.QueueStore
	dagRunMgr     runtime.Manager
	subCmdBuilder *runtime.SubCmdBuilder
}

// resolveNamespaceStores resolves a namespace by name and creates namespace-scoped stores.
func (a *API) resolveNamespaceStores(ctx context.Context, namespaceName string) (*namespaceScopedStores, error) {
	if a.namespaceStore == nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "namespace store not configured",
		}
	}

	ns, err := a.namespaceStore.Get(ctx, namespaceName)
	if err != nil {
		if errors.Is(err, exec.ErrNamespaceNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("Namespace %q not found", namespaceName),
			}
		}
		return nil, err
	}

	namespacedDAGsDir := filepath.Join(a.config.Paths.DAGsDir, ns.ID)
	nsBase := exec.NamespaceDir(a.config.Paths.DataDir, ns.ID)
	namespacedDAGRunsDir := filepath.Join(nsBase, "dag-runs")
	namespacedProcDir := filepath.Join(nsBase, "proc")
	namespacedQueueDir := filepath.Join(nsBase, "queue")
	namespacedSuspendDir := filepath.Join(nsBase, "suspend")

	dagStore := filedag.New(
		namespacedDAGsDir,
		filedag.WithFlagsBaseDir(namespacedSuspendDir),
		filedag.WithSkipExamples(true),
	)

	dagRunStore := filedagrun.New(namespacedDAGRunsDir)
	procStore := fileproc.New(namespacedProcDir)
	queueStore := filequeue.New(namespacedQueueDir)
	dagRunMgr := runtime.NewManager(dagRunStore, procStore, a.config)

	return &namespaceScopedStores{
		namespace:     ns,
		dagStore:      dagStore,
		dagRunStore:   dagRunStore,
		procStore:     procStore,
		queueStore:    queueStore,
		dagRunMgr:     dagRunMgr,
		subCmdBuilder: runtime.NewSubCmdBuilder(a.config),
	}, nil
}

// ListNamespaceDAGs lists DAGs within a specific namespace.
// Requires the user to have access to the namespace (any valid role).
func (a *API) ListNamespaceDAGs(ctx context.Context, request api.ListNamespaceDAGsRequestObject) (api.ListNamespaceDAGsResponseObject, error) {
	if err := a.requireNamespaceAccess(ctx, request.NamespaceName); err != nil {
		return nil, err
	}

	stores, err := a.resolveNamespaceStores(ctx, request.NamespaceName)
	if err != nil {
		return nil, err
	}

	sortField := cmp.Or(a.config.UI.DAGs.SortField, "name")
	if request.Params.Sort != nil {
		sortField = string(*request.Params.Sort)
	}

	sortOrder := cmp.Or(a.config.UI.DAGs.SortOrder, "asc")
	if request.Params.Order != nil {
		sortOrder = string(*request.Params.Order)
	}

	pg := exec.NewPaginator(valueOf(request.Params.Page), valueOf(request.Params.PerPage))
	tags := parseCommaSeparatedTags(request.Params.Tags)

	result, errList, err := stores.dagStore.List(ctx, exec.ListDAGsOptions{
		Paginator: &pg,
		Name:      valueOf(request.Params.Name),
		Tags:      tags,
		Sort:      sortField,
		Order:     sortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing DAGs: %w", err)
	}

	dagFiles := make([]api.DAGFile, 0, len(result.Items))
	for _, item := range result.Items {
		dagStatus, statusErr := stores.dagRunMgr.GetLatestStatus(ctx, item)
		if statusErr != nil {
			errList = append(errList, statusErr.Error())
		}

		dagFiles = append(dagFiles, api.DAGFile{
			FileName:     item.FileName(),
			LatestDAGRun: toDAGRunSummary(dagStatus),
			Suspended:    stores.dagStore.IsSuspended(ctx, item.FileName()),
			Dag:          toDAG(item),
			Errors:       extractBuildErrors(item.BuildErrors),
		})
	}

	return &api.ListNamespaceDAGs200JSONResponse{
		Dags:       dagFiles,
		Errors:     errList,
		Pagination: toPagination(result),
	}, nil
}

// CreateNamespaceDAG creates a new DAG within a specific namespace.
// Requires namespace-scoped write permission.
func (a *API) CreateNamespaceDAG(ctx context.Context, request api.CreateNamespaceDAGRequestObject) (api.CreateNamespaceDAGResponseObject, error) {
	if err := a.requireNamespaceDAGWrite(ctx, request.NamespaceName); err != nil {
		return nil, err
	}

	stores, err := a.resolveNamespaceStores(ctx, request.NamespaceName)
	if err != nil {
		return nil, err
	}

	var yamlSpec []byte
	if request.Body.Spec != nil && strings.TrimSpace(*request.Body.Spec) != "" {
		_, err := spec.LoadYAML(ctx,
			[]byte(*request.Body.Spec),
			spec.WithName(request.Body.Name),
			spec.WithoutEval(),
		)
		if err != nil {
			var verrs core.ErrorList
			if errors.As(err, &verrs) {
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
		yamlSpec = []byte(`steps:
  - command: echo hello
`)
	}

	if err := stores.dagStore.Create(ctx, request.Body.Name, yamlSpec); err != nil {
		if errors.Is(err, exec.ErrDAGAlreadyExists) {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
			}
		}
		return nil, fmt.Errorf("error creating DAG: %w", err)
	}

	a.logAuditEntry(ctx, audit.CategoryDAG, "dag_create", map[string]any{
		"dag_name":  request.Body.Name,
		"namespace": request.NamespaceName,
	})

	return &api.CreateNamespaceDAG201JSONResponse{
		Name: request.Body.Name,
	}, nil
}

// GetNamespaceDAGDetails retrieves comprehensive DAG information within a namespace.
// Requires the user to have access to the namespace (any valid role).
func (a *API) GetNamespaceDAGDetails(ctx context.Context, request api.GetNamespaceDAGDetailsRequestObject) (api.GetNamespaceDAGDetailsResponseObject, error) {
	if err := a.requireNamespaceAccess(ctx, request.NamespaceName); err != nil {
		return nil, err
	}

	stores, err := a.resolveNamespaceStores(ctx, request.NamespaceName)
	if err != nil {
		return nil, err
	}

	dag, err := stores.dagStore.GetDetails(ctx, request.FileName, spec.WithAllowBuildErrors())
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found in namespace %s", request.FileName, request.NamespaceName),
		}
	}

	dagStatus, err := stores.dagRunMgr.GetLatestStatus(ctx, dag)
	if err != nil && !errors.Is(err, exec.ErrNoStatusData) {
		return nil, fmt.Errorf("failed to get latest status for DAG %s", request.FileName)
	}

	yamlSpec, err := stores.dagStore.GetSpec(ctx, request.FileName)
	if err != nil {
		yamlSpec = ""
	}

	details := toDAGDetails(dag)

	localDAGs := make([]api.LocalDag, 0, len(dag.LocalDAGs))
	for _, localDAG := range dag.LocalDAGs {
		localDAGs = append(localDAGs, toLocalDAG(localDAG))
	}

	sort.Slice(localDAGs, func(i, j int) bool {
		return localDAGs[i].Name < localDAGs[j].Name
	})

	return &api.GetNamespaceDAGDetails200JSONResponse{
		Dag:          details,
		LatestDAGRun: ToDAGRunDetails(dagStatus),
		Suspended:    stores.dagStore.IsSuspended(ctx, request.FileName),
		LocalDags:    localDAGs,
		Errors:       extractBuildErrors(dag.BuildErrors),
		Spec:         &yamlSpec,
	}, nil
}

// TriggerNamespaceDAGRun triggers a DAG run within a specific namespace.
// Requires namespace-scoped execute permission.
func (a *API) TriggerNamespaceDAGRun(ctx context.Context, request api.TriggerNamespaceDAGRunRequestObject) (api.TriggerNamespaceDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireNamespaceExecute(ctx, request.NamespaceName); err != nil {
		return nil, err
	}

	stores, err := a.resolveNamespaceStores(ctx, request.NamespaceName)
	if err != nil {
		return nil, err
	}

	dag, err := stores.dagStore.GetDetails(ctx, request.FileName, spec.WithAllowBuildErrors())
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found in namespace %s", request.FileName, request.NamespaceName),
		}
	}

	if err := buildErrorsToAPIError(dag.BuildErrors); err != nil {
		return nil, err
	}

	// Set namespace on the DAG for correct socket addressing and sub-DAG propagation.
	dag.Namespace = stores.namespace.Name

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
		dagRunId, err = stores.dagRunMgr.GenDAGRunID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", err)
		}
	}

	if singleton {
		if err := a.checkSingletonRunning(ctx, stores.procStore, dag); err != nil {
			return nil, err
		}
	}

	if err := a.ensureDAGRunIDUnique(ctx, stores.dagRunStore, dag, dagRunId); err != nil {
		return nil, err
	}

	if err := a.startNamespaceScopedDAGRun(ctx, stores, dag, params, dagRunId, nameOverride); err != nil {
		return nil, fmt.Errorf("error starting dag-run: %w", err)
	}

	detailsMap := map[string]any{
		"dag_name":   request.FileName,
		"dag_run_id": dagRunId,
		"namespace":  request.NamespaceName,
	}
	if params != "" {
		detailsMap["params"] = params
	}
	a.logAuditEntry(ctx, audit.CategoryDAG, "dag_execute", detailsMap)

	return &api.TriggerNamespaceDAGRun200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

// startNamespaceScopedDAGRun starts a DAG run using namespace-scoped stores.
func (a *API) startNamespaceScopedDAGRun(
	ctx context.Context,
	stores *namespaceScopedStores,
	dag *core.DAG,
	params, dagRunID, nameOverride string,
) error {
	triggerTypeStr := core.TriggerTypeManual.String()
	startSpec := stores.subCmdBuilder.Start(dag, runtime.StartOptions{
		Params:       params,
		DAGRunID:     dagRunID,
		Quiet:        true,
		NameOverride: nameOverride,
		TriggerType:  triggerTypeStr,
	})

	if err := runtime.Start(ctx, startSpec); err != nil {
		return fmt.Errorf("error starting DAG: %w", err)
	}

	if !a.waitForDAGStatusChange(ctx, stores.dagRunMgr, dag, dagRunID, 5*time.Second) {
		return &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "DAG did not start",
		}
	}

	return nil
}

// listAccessibleNamespaces returns all namespaces the current user can access.
// When auth is disabled, returns all namespaces.
func (a *API) listAccessibleNamespaces(ctx context.Context) ([]*exec.Namespace, error) {
	if a.namespaceStore == nil {
		return nil, nil
	}

	namespaces, err := a.namespaceStore.List(ctx)
	if err != nil {
		return nil, err
	}

	if a.authService == nil {
		return namespaces, nil
	}

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, nil
	}

	filtered := make([]*exec.Namespace, 0, len(namespaces))
	for _, ns := range namespaces {
		if user.HasNamespaceAccess(ns.Name) {
			filtered = append(filtered, ns)
		}
	}
	return filtered, nil
}

// aggregatedDAGsResult holds the result of aggregating DAGs across namespaces.
type aggregatedDAGsResult struct {
	dagFiles []api.DAGFile
	errors   []string
	total    int
}

// listDAGsAcrossNamespaces aggregates DAGs from all accessible namespaces.
func (a *API) listDAGsAcrossNamespaces(ctx context.Context, listOpts exec.ListDAGsOptions) (*aggregatedDAGsResult, error) {
	namespaces, err := a.listAccessibleNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	if len(namespaces) == 0 {
		return &aggregatedDAGsResult{}, nil
	}

	var allDAGFiles []api.DAGFile
	var allErrors []string

	for _, ns := range namespaces {
		stores, storeErr := a.resolveNamespaceStores(ctx, ns.Name)
		if storeErr != nil {
			allErrors = append(allErrors, fmt.Sprintf("namespace %s: %v", ns.Name, storeErr))
			continue
		}

		// List without pagination — we aggregate then paginate.
		noPagOpts := listOpts
		noPagOpts.Paginator = nil

		result, errList, listErr := stores.dagStore.List(ctx, noPagOpts)
		if listErr != nil {
			allErrors = append(allErrors, fmt.Sprintf("namespace %s: %v", ns.Name, listErr))
			continue
		}
		allErrors = append(allErrors, errList...)

		nsName := ns.Name
		for _, item := range result.Items {
			dagStatus, statusErr := stores.dagRunMgr.GetLatestStatus(ctx, item)
			if statusErr != nil {
				allErrors = append(allErrors, statusErr.Error())
			}

			allDAGFiles = append(allDAGFiles, api.DAGFile{
				FileName:     item.FileName(),
				LatestDAGRun: toDAGRunSummary(dagStatus),
				Suspended:    stores.dagStore.IsSuspended(ctx, item.FileName()),
				Dag:          toDAG(item),
				Errors:       extractBuildErrors(item.BuildErrors),
				Namespace:    &nsName,
			})
		}
	}

	// Apply sorting (currently only "name" is supported).
	sortOrder := cmp.Or(listOpts.Order, "asc")
	sort.Slice(allDAGFiles, func(i, j int) bool {
		less := allDAGFiles[i].Dag.Name < allDAGFiles[j].Dag.Name
		if sortOrder == "desc" {
			return !less
		}
		return less
	})

	total := len(allDAGFiles)

	// Apply pagination if requested.
	if listOpts.Paginator != nil {
		pg := listOpts.Paginator
		start := pg.Offset()
		if start > len(allDAGFiles) {
			start = len(allDAGFiles)
		}
		end := start + pg.Limit()
		if end > len(allDAGFiles) {
			end = len(allDAGFiles)
		}
		allDAGFiles = allDAGFiles[start:end]
	}

	return &aggregatedDAGsResult{
		dagFiles: allDAGFiles,
		errors:   allErrors,
		total:    total,
	}, nil
}

// listDAGRunsAcrossNamespaces aggregates DAG runs from all accessible namespaces.
func (a *API) listDAGRunsAcrossNamespaces(ctx context.Context, opts []exec.ListDAGRunStatusesOption) ([]api.DAGRunSummary, error) {
	namespaces, err := a.listAccessibleNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	if len(namespaces) == 0 {
		return nil, nil
	}

	var allRuns []api.DAGRunSummary

	for _, ns := range namespaces {
		stores, storeErr := a.resolveNamespaceStores(ctx, ns.Name)
		if storeErr != nil {
			continue
		}

		statuses, listErr := stores.dagRunStore.ListStatuses(ctx, opts...)
		if listErr != nil {
			continue
		}

		nsName := ns.Name
		for _, status := range statuses {
			summary := toDAGRunSummary(*status)
			summary.Namespace = &nsName
			allRuns = append(allRuns, summary)
		}
	}

	// Sort by start time descending (most recent first).
	sort.Slice(allRuns, func(i, j int) bool {
		return allRuns[i].StartedAt > allRuns[j].StartedAt
	})

	return allRuns, nil
}

// searchDAGsAcrossNamespaces aggregates search results from all accessible namespaces.
func (a *API) searchDAGsAcrossNamespaces(ctx context.Context, query string) ([]api.SearchResultItem, []string, error) {
	namespaces, err := a.listAccessibleNamespaces(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(namespaces) == 0 {
		return nil, nil, nil
	}

	var allResults []api.SearchResultItem
	var allErrors []string

	for _, ns := range namespaces {
		stores, storeErr := a.resolveNamespaceStores(ctx, ns.Name)
		if storeErr != nil {
			allErrors = append(allErrors, fmt.Sprintf("namespace %s: %v", ns.Name, storeErr))
			continue
		}

		ret, errs, grepErr := stores.dagStore.Grep(ctx, query)
		if grepErr != nil {
			allErrors = append(allErrors, fmt.Sprintf("namespace %s: %v", ns.Name, grepErr))
			continue
		}
		allErrors = append(allErrors, errs...)

		nsName := ns.Name
		for _, item := range ret {
			allResults = append(allResults, toSearchResultItem(item, &nsName))
		}
	}

	return allResults, allErrors, nil
}
