package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime"
)

// ExecuteDAGRunFromSpec implements api.StrictServerInterface.
func (a *API) ExecuteDAGRunFromSpec(ctx context.Context, request api.ExecuteDAGRunFromSpecRequestObject) (api.ExecuteDAGRunFromSpecResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	if request.Body == nil || request.Body.Spec == "" {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "spec is required",
		}
	}

	// Determine dagRunId upfront (used for unique temp dir path)
	var dagRunId, params string
	var singleton bool
	if request.Body.DagRunId != nil {
		dagRunId = *request.Body.DagRunId
	}
	if dagRunId == "" {
		var genErr error
		dagRunId, genErr = a.dagRunMgr.GenDAGRunID(ctx)
		if genErr != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", genErr)
		}
	}
	if request.Body.Params != nil {
		params = *request.Body.Params
	}
	if request.Body.Singleton != nil {
		singleton = *request.Body.Singleton
	}

	dag, cleanup, err := a.loadInlineDAG(ctx, request.Body.Spec, request.Body.Name, dagRunId)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Ensure the dag-run ID is not already used for this DAG name
	if _, err := a.dagRunStore.FindAttempt(ctx, execution.DAGRunRef{Name: dag.Name, ID: dagRunId}); !errors.Is(err, execution.ErrDAGRunIDNotFound) {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("dag-run ID %s already exists for DAG %s", dagRunId, dag.Name),
		}
	}

	// Concurrency checks similar to ExecuteDAG
	liveCount, err := a.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to access proc store: %w", err)
	}
	if singleton || dag.MaxActiveRuns == 1 {
		if liveCount > 0 {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeMaxRunReached,
				Message:    fmt.Sprintf("DAG %s is already running, cannot start", dag.Name),
			}
		}
	}

	queuedRuns, err := a.queueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read queue: %w", err)
	}
	if dag.MaxActiveRuns > 0 && len(queuedRuns)+liveCount >= dag.MaxActiveRuns {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeMaxRunReached,
			Message:    fmt.Sprintf("DAG %s is already in the queue (maxActiveRuns=%d), cannot start", dag.Name, dag.MaxActiveRuns),
		}
	}

	if err := a.startDAGRun(ctx, dag, params, dagRunId, valueOf(request.Body.Name), singleton); err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    fmt.Sprintf("failed to start dag-run: %s", err.Error()),
		}
	}

	return api.ExecuteDAGRunFromSpec200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

// EnqueueDAGRunFromSpec implements api.StrictServerInterface.
func (a *API) EnqueueDAGRunFromSpec(ctx context.Context, request api.EnqueueDAGRunFromSpecRequestObject) (api.EnqueueDAGRunFromSpecResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	if request.Body == nil || request.Body.Spec == "" {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "spec is required",
		}
	}

	var dagRunId, params string
	if request.Body.DagRunId != nil {
		dagRunId = *request.Body.DagRunId
	}
	if dagRunId == "" {
		var genErr error
		dagRunId, genErr = a.dagRunMgr.GenDAGRunID(ctx)
		if genErr != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", genErr)
		}
	}
	if request.Body.Params != nil {
		params = *request.Body.Params
	}

	dag, cleanup, err := a.loadInlineDAG(ctx, request.Body.Spec, request.Body.Name, dagRunId)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if request.Body.Queue != nil && *request.Body.Queue != "" {
		dag.Queue = *request.Body.Queue
	}

	if _, err := a.dagRunStore.FindAttempt(ctx, execution.DAGRunRef{Name: dag.Name, ID: dagRunId}); !errors.Is(err, execution.ErrDAGRunIDNotFound) {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("dag-run ID %s already exists for DAG %s", dagRunId, dag.Name),
		}
	}

	liveCount, err := a.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to access proc store: %w", err)
	}

	queuedRuns, err := a.queueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read queue: %w", err)
	}

	if dag.Queue != "" && dag.MaxActiveRuns > 1 && len(queuedRuns)+liveCount >= dag.MaxActiveRuns {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeMaxRunReached,
			Message:    fmt.Sprintf("DAG %s is already in the queue (maxActiveRuns=%d), cannot enqueue", dag.Name, dag.MaxActiveRuns),
		}
	}

	if err := a.enqueueDAGRun(ctx, dag, params, dagRunId, valueOf(request.Body.Name)); err != nil {
		return nil, fmt.Errorf("error enqueuing dag-run: %w", err)
	}

	return api.EnqueueDAGRunFromSpec200JSONResponse{
		DagRunId: dagRunId,
	}, nil
}

func (a *API) loadInlineDAG(ctx context.Context, specContent string, name *string, dagRunID string) (*core.DAG, func(), error) {
	nameHint := "inline"
	if name != nil && *name != "" {
		if err := core.ValidateDAGName(*name); err != nil {
			return nil, func() {}, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
		nameHint = *name
	} else {
		dag, err := spec.LoadYAML(
			ctx, []byte(specContent),
			spec.WithoutEval(),
		)
		if err != nil {
			return nil, func() {}, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
		if err := dag.Validate(); err != nil {
			return nil, func() {}, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    err.Error(),
			}
		}
	}

	tmpDir := filepath.Join(os.TempDir(), nameHint, dagRunID)
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		return nil, func() {}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	tfPath := filepath.Join(tmpDir, fmt.Sprintf("%s.yaml", nameHint))
	if err := os.WriteFile(tfPath, []byte(specContent), 0o600); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("failed to write spec to temp file: %w", err)
	}

	var loadOpts []spec.LoadOption
	if name != nil && *name != "" {
		loadOpts = append(loadOpts, spec.WithName(*name))
	}
	dag, err := spec.Load(ctx, tfPath, loadOpts...)
	if err != nil {
		cleanup()
		return nil, func() {}, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    err.Error(),
		}
	}

	return dag, cleanup, nil
}

// no sanitize helper: DAG name is validated by core.ValidateDAGName

func (a *API) ListDAGRuns(ctx context.Context, request api.ListDAGRunsRequestObject) (api.ListDAGRunsResponseObject, error) {
	var opts []execution.ListDAGRunStatusesOption
	if request.Params.Status != nil {
		opts = append(opts, execution.WithStatuses([]core.Status{
			core.Status(*request.Params.Status),
		}))
	}
	if request.Params.FromDate != nil {
		dt := execution.NewUTC(time.Unix(*request.Params.FromDate, 0))
		opts = append(opts, execution.WithFrom(dt))
	}
	if request.Params.ToDate != nil {
		dt := execution.NewUTC(time.Unix(*request.Params.ToDate, 0))
		opts = append(opts, execution.WithTo(dt))
	}
	if request.Params.Name != nil {
		opts = append(opts, execution.WithName(*request.Params.Name))
	}
	if request.Params.DagRunId != nil {
		opts = append(opts, execution.WithDAGRunID(*request.Params.DagRunId))
	}

	dagRuns, err := a.listDAGRuns(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	return api.ListDAGRuns200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

func (a *API) ListDAGRunsByName(ctx context.Context, request api.ListDAGRunsByNameRequestObject) (api.ListDAGRunsByNameResponseObject, error) {
	opts := []execution.ListDAGRunStatusesOption{
		execution.WithExactName(request.Name),
	}

	if request.Params.Status != nil {
		opts = append(opts, execution.WithStatuses([]core.Status{
			core.Status(*request.Params.Status),
		}))
	}
	if request.Params.FromDate != nil {
		dt := execution.NewUTC(time.Unix(*request.Params.FromDate, 0))
		opts = append(opts, execution.WithFrom(dt))
	}
	if request.Params.ToDate != nil {
		dt := execution.NewUTC(time.Unix(*request.Params.ToDate, 0))
		opts = append(opts, execution.WithTo(dt))
	}
	if request.Params.DagRunId != nil {
		opts = append(opts, execution.WithDAGRunID(*request.Params.DagRunId))
	}

	dagRuns, err := a.listDAGRuns(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	return api.ListDAGRunsByName200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

func (a *API) listDAGRuns(ctx context.Context, opts []execution.ListDAGRunStatusesOption) ([]api.DAGRunSummary, error) {
	statuses, err := a.dagRunStore.ListStatuses(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}
	var dagRuns []api.DAGRunSummary
	for _, status := range statuses {
		dagRuns = append(dagRuns, toDAGRunSummary(*status))
	}
	return dagRuns, nil
}

func (a *API) GetDAGRunLog(ctx context.Context, request api.GetDAGRunLogRequestObject) (api.GetDAGRunLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := execution.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(dagStatus.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", dagStatus.Log, err)
	}

	return api.GetDAGRunLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) GetDAGRunStepLog(ctx context.Context, request api.GetDAGRunStepLogRequestObject) (api.GetDAGRunStepLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := execution.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return api.GetDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	var logFile = node.Stdout
	if *request.Params.Stream == api.StreamStderr {
		logFile = node.Stderr
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(logFile, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Stdout, err)
	}

	return api.GetDAGRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

func (a *API) UpdateDAGRunStepStatus(ctx context.Context, request api.UpdateDAGRunStepStatusRequestObject) (api.UpdateDAGRunStepStatusResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	ref := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.UpdateDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}
	if dagStatus.Status == core.Running {
		return &api.UpdateDAGRunStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("dag-run ID %s for DAG %s is still running", request.DagRunId, request.Name),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	dagStatus.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	root := execution.NewDAGRunRef(request.Name, request.DagRunId)
	if err := a.dagRunMgr.UpdateStatus(ctx, root, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateDAGRunStepStatus200Response{}, nil
}

// GetDAGRunDetails implements api.StrictServerInterface.
func (a *API) GetDAGRunDetails(ctx context.Context, request api.GetDAGRunDetailsRequestObject) (api.GetDAGRunDetailsResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	if dagRunId == "latest" {
		// Get the DAG to retrieve the latest status
		attempt, err := a.dagRunStore.LatestAttempt(ctx, dagName)
		if err != nil {
			return &api.GetDAGRunDetails404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("no dag-runs found for DAG %s", dagName),
			}, nil
		}
		status, err := attempt.ReadStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting latest status: %w", err)
		}
		return &api.GetDAGRunDetails200JSONResponse{
			DagRunDetails: toDAGRunDetails(*status),
		}, nil
	}

	ref := execution.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.GetDAGRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}
	return &api.GetDAGRunDetails200JSONResponse{
		DagRunDetails: toDAGRunDetails(*dagStatus),
	}, nil
}

// GetSubDAGRunDetails implements api.StrictServerInterface.
func (a *API) GetSubDAGRunDetails(ctx context.Context, request api.GetSubDAGRunDetailsRequestObject) (api.GetSubDAGRunDetailsResponseObject, error) {
	root := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.GetSubDAGRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}
	return &api.GetSubDAGRunDetails200JSONResponse{
		DagRunDetails: toDAGRunDetails(*dagStatus),
	}, nil
}

// GetSubDAGRunLog implements api.StrictServerInterface.
func (a *API) GetSubDAGRunLog(ctx context.Context, request api.GetSubDAGRunLogRequestObject) (api.GetSubDAGRunLogResponseObject, error) {
	root := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.GetSubDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(dagStatus.Log, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", dagStatus.Log, err)
	}

	return &api.GetSubDAGRunLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// GetSubDAGRunStepLog implements api.StrictServerInterface.
func (a *API) GetSubDAGRunStepLog(ctx context.Context, request api.GetSubDAGRunStepLogRequestObject) (api.GetSubDAGRunStepLogResponseObject, error) {
	root := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.GetSubDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return &api.GetSubDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	// Extract pagination parameters
	options := fileutil.LogReadOptions{
		Head:   valueOf(request.Params.Head),
		Tail:   valueOf(request.Params.Tail),
		Offset: valueOf(request.Params.Offset),
		Limit:  valueOf(request.Params.Limit),
	}

	var logFile = node.Stdout
	if *request.Params.Stream == api.StreamStderr {
		logFile = node.Stderr
	}

	// Use the new log utility function
	content, lineCount, totalLines, hasMore, isEstimate, err := fileutil.ReadLogContent(logFile, options)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", node.Stdout, err)
	}

	return &api.GetSubDAGRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// UpdateSubDAGRunStepStatus implements api.StrictServerInterface.
func (a *API) UpdateSubDAGRunStepStatus(ctx context.Context, request api.UpdateSubDAGRunStepStatusRequestObject) (api.UpdateSubDAGRunStepStatusResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	root := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.UpdateSubDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}
	if dagStatus.Status == core.Running {
		return &api.UpdateSubDAGRunStepStatus400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("dag-run ID %s for DAG %s is still running", request.DagRunId, request.Name),
		}, nil
	}

	idxToUpdate := -1

	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			idxToUpdate = idx
		}
	}
	if idxToUpdate < 0 {
		return &api.UpdateSubDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	dagStatus.Nodes[idxToUpdate].Status = nodeStatusMapping[request.Body.Status]

	if err := a.dagRunMgr.UpdateStatus(ctx, root, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return &api.UpdateSubDAGRunStepStatus200Response{}, nil
}

var nodeStatusMapping = map[api.NodeStatus]core.NodeStatus{
	api.NodeStatusNotStarted: core.NodeNotStarted,
	api.NodeStatusRunning:    core.NodeRunning,
	api.NodeStatusFailed:     core.NodeFailed,
	api.NodeStatusCancelled:  core.NodeCanceled,
	api.NodeStatusSuccess:    core.NodeSucceeded,
	api.NodeStatusSkipped:    core.NodeSkipped,
}

func (a *API) RetryDAGRun(ctx context.Context, request api.RetryDAGRunRequestObject) (api.RetryDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	attempt, err := a.dagRunStore.FindAttempt(ctx, execution.NewDAGRunRef(request.Name, request.DagRunId))
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	var targetRunID string
	var generateNewRunID bool
	if request.Body != nil {
		targetRunID = strings.TrimSpace(valueOf(request.Body.DagRunId))
		generateNewRunID = valueOf(request.Body.GenerateNewRunId)
	}

	if generateNewRunID {
		targetRunID = ""
	}

	useSameRunID := !generateNewRunID && (targetRunID == "" || targetRunID == request.DagRunId)

	if request.Body != nil && request.Body.StepName != nil && *request.Body.StepName != "" && !useSameRunID {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "step retry does not support generating a new run ID; reuse the original run ID or omit stepName",
		}
	}

	// Get count of running DAGs to check against maxActiveRuns (best effort)
	liveCount, err := a.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to access proc store: %w", err)
	}
	// Count queued DAG-runs and check against maxActiveRuns
	queuedRuns, err := a.queueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read queue: %w", err)
	}
	// If the DAG has a queue configured and maxActiveRuns > 0, ensure the number
	// of active runs in the queue does not exceed this limit.
	if dag.MaxActiveRuns > 0 && len(queuedRuns)+liveCount >= dag.MaxActiveRuns {
		// The same DAG is already in the queue
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeMaxRunReached,
			Message:    fmt.Sprintf("DAG %s is already in the queue (maxActiveRuns=%d), cannot retry", dag.Name, dag.MaxActiveRuns),
		}
	}

	if useSameRunID {
		targetRunID = request.DagRunId
		if request.Body != nil && request.Body.StepName != nil && *request.Body.StepName != "" {
			spec := a.subCmdBuilder.Retry(dag, targetRunID, *request.Body.StepName, true)
			if err := runtime.Start(ctx, spec); err != nil {
				return nil, fmt.Errorf("error retrying DAG step: %w", err)
			}
			return api.RetryDAGRun200JSONResponse{DagRunId: targetRunID}, nil
		}

		spec := a.subCmdBuilder.Retry(dag, targetRunID, "", false)
		if err := runtime.Start(ctx, spec); err != nil {
			return nil, fmt.Errorf("error retrying DAG: %w", err)
		}

		return api.RetryDAGRun200JSONResponse{DagRunId: targetRunID}, nil
	}

	if targetRunID != "" {
		if err := validateDagRunID(targetRunID); err != nil {
			return nil, err
		}
	}

	if targetRunID == "" {
		var genErr error
		targetRunID, genErr = a.dagRunMgr.GenDAGRunID(ctx)
		if genErr != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", genErr)
		}
	}

	if _, err := a.dagRunStore.FindAttempt(ctx, execution.DAGRunRef{Name: dag.Name, ID: targetRunID}); !errors.Is(err, execution.ErrDAGRunIDNotFound) {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("dag-run ID %s already exists for DAG %s", targetRunID, dag.Name),
		}
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG-run status: %w", err)
	}

	if err := a.startDAGRun(ctx, dag, status.Params, targetRunID, "", false); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGRun200JSONResponse{DagRunId: targetRunID}, nil
}

const maxDagRunIDLen = 64

var dagRunIDRegex = regexp.MustCompile(`^[-a-zA-Z0-9_]+$`)

func validateDagRunID(id string) error {
	if id == "" {
		return &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "dag-run ID must be provided when generateNewRunId is false",
		}
	}
	if len(id) > maxDagRunIDLen {
		return &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("dag-run ID length must be less than %d characters", maxDagRunIDLen),
		}
	}
	if !dagRunIDRegex.MatchString(id) {
		return &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    "dag-run ID must only contain alphanumeric characters, dashes, and underscores",
		}
	}
	return nil
}

func (a *API) TerminateDAGRun(ctx context.Context, request api.TerminateDAGRunRequestObject) (api.TerminateDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	attempt, err := a.dagRunStore.FindAttempt(ctx, execution.NewDAGRunRef(request.Name, request.DagRunId))
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	dagStatus, err := a.dagRunMgr.GetCurrentStatus(ctx, dag, request.DagRunId)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.Name),
		}
	}

	if dagStatus.Status != core.Running {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeNotRunning,
			Message:    "DAG is not running",
		}
	}

	if err := a.dagRunMgr.Stop(ctx, dag, dagStatus.DAGRunID); err != nil {
		return nil, fmt.Errorf("error stopping DAG: %w", err)
	}

	return api.TerminateDAGRun200Response{}, nil
}

func (a *API) DequeueDAGRun(ctx context.Context, request api.DequeueDAGRunRequestObject) (api.DequeueDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}

	dagRun := execution.NewDAGRunRef(request.Name, request.DagRunId)
	attempt, err := a.dagRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading DAG: %w", err)
	}

	latestStatus, err := a.dagRunMgr.GetCurrentStatus(ctx, dag, dagRun.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting latest status: %w", err)
	}

	if latestStatus.Status != core.Queued {
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    fmt.Sprintf("DAGRun status is not queued: %s", latestStatus.Status),
		}
	}

	spec := a.subCmdBuilder.Dequeue(dag, dagRun)
	if err := runtime.Run(ctx, spec); err != nil {
		return nil, fmt.Errorf("error dequeueing dag-run: %w", err)
	}

	return api.DequeueDAGRun200Response{}, nil
}

// GetSubDAGRuns returns timing and status information for all sub DAG runs
func (a *API) GetSubDAGRuns(ctx context.Context, request api.GetSubDAGRunsRequestObject) (api.GetSubDAGRunsResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	// Get the parent DAG run status
	parentRef := execution.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, parentRef)
	if err != nil {
		return &api.GetSubDAGRuns404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	subRuns := make([]api.SubDAGRunDetail, 0)

	// Iterate through all nodes to find sub DAG runs
	for _, node := range dagStatus.Nodes {
		// Skip nodes without sub DAG runs
		if len(node.SubRuns) == 0 && len(node.SubRunsRepeated) == 0 {
			continue
		}

		// Collect regular sub runs
		for _, subRun := range node.SubRuns {
			detail, err := a.getSubDAGRunDetail(ctx, parentRef, subRun.DAGRunID, subRun.Params)
			if err != nil {
				// Skip if we can't fetch details
				continue
			}
			subRuns = append(subRuns, detail)
		}

		// Collect repeated sub runs
		for _, subRun := range node.SubRunsRepeated {
			detail, err := a.getSubDAGRunDetail(ctx, parentRef, subRun.DAGRunID, subRun.Params)
			if err != nil {
				// Skip if we can't fetch details
				continue
			}
			subRuns = append(subRuns, detail)
		}
	}

	return &api.GetSubDAGRuns200JSONResponse{
		SubRuns: subRuns,
	}, nil
}

// getSubDAGRunDetail fetches timing and status info for a single sub DAG run
func (a *API) getSubDAGRunDetail(ctx context.Context, parentRef execution.DAGRunRef, subRunID string, params string) (api.SubDAGRunDetail, error) {
	// Use FindSubDAGRunStatus to properly fetch sub DAG run from parent's storage
	status, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, parentRef, subRunID)
	if err != nil {
		return api.SubDAGRunDetail{}, err
	}

	detail := api.SubDAGRunDetail{
		DagRunId:    subRunID,
		Status:      api.Status(status.Status),
		StatusLabel: api.StatusLabel(status.Status.String()),
		StartedAt:   status.StartedAt,
		FinishedAt:  &status.FinishedAt,
	}

	if params != "" {
		detail.Params = &params
	}

	return detail, nil
}
