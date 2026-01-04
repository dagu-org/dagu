package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
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
	if err := a.requireExecute(ctx); err != nil {
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

	if err := a.ensureDAGRunIDUnique(ctx, dag, dagRunId); err != nil {
		return nil, err
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
	if err := a.requireExecute(ctx); err != nil {
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

	workDir, _ := os.Getwd()
	if workDir == "" {
		workDir, _ = os.UserHomeDir()
	}
	loadOpts := []spec.LoadOption{spec.WithDefaultWorkingDir(workDir)}
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
		Head:     valueOf(request.Params.Head),
		Tail:     valueOf(request.Params.Tail),
		Offset:   valueOf(request.Params.Offset),
		Limit:    valueOf(request.Params.Limit),
		Encoding: a.logEncodingCharset,
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

// GetDAGRunOutputs implements api.StrictServerInterface.
func (a *API) GetDAGRunOutputs(ctx context.Context, request api.GetDAGRunOutputsRequestObject) (api.GetDAGRunOutputsResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	var attempt execution.DAGRunAttempt
	var err error

	if dagRunId == "latest" {
		attempt, err = a.dagRunStore.LatestAttempt(ctx, dagName)
		if err != nil {
			return api.GetDAGRunOutputs404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("no dag-runs found for DAG %s", dagName),
			}, nil
		}
	} else {
		ref := execution.NewDAGRunRef(dagName, dagRunId)
		attempt, err = a.dagRunStore.FindAttempt(ctx, ref)
		if err != nil {
			return api.GetDAGRunOutputs404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
			}, nil
		}
	}

	outputs, err := attempt.ReadOutputs(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read outputs",
			tag.Error(err),
			slog.String("dag", dagName),
			slog.String("dagRunId", dagRunId),
		)
		return nil, fmt.Errorf("error reading outputs: %w", err)
	}

	// Return empty structure if no outputs (DAG-run exists but captured no outputs)
	if outputs == nil {
		outputs = &execution.DAGRunOutputs{
			Metadata: execution.OutputsMetadata{},
			Outputs:  make(map[string]string),
		}
	}

	// Parse CompletedAt from RFC3339 string to time.Time
	var completedAt time.Time
	if outputs.Metadata.CompletedAt != "" {
		if t, err := time.Parse(time.RFC3339, outputs.Metadata.CompletedAt); err == nil {
			completedAt = t
		}
	}

	return api.GetDAGRunOutputs200JSONResponse{
		Metadata: api.OutputsMetadata{
			DagName:     outputs.Metadata.DAGName,
			DagRunId:    outputs.Metadata.DAGRunID,
			AttemptId:   outputs.Metadata.AttemptID,
			Status:      api.StatusLabel(outputs.Metadata.Status),
			CompletedAt: completedAt,
			Params:      &outputs.Metadata.Params,
		},
		Outputs: outputs.Outputs,
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
		Head:     valueOf(request.Params.Head),
		Tail:     valueOf(request.Params.Tail),
		Offset:   valueOf(request.Params.Offset),
		Limit:    valueOf(request.Params.Limit),
		Encoding: a.logEncodingCharset,
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
	if err := a.requireExecute(ctx); err != nil {
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

// ApproveDAGRunStep approves a waiting step for HITL (Human-in-the-Loop).
func (a *API) ApproveDAGRunStep(ctx context.Context, request api.ApproveDAGRunStepRequestObject) (api.ApproveDAGRunStepResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	ref := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.ApproveDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}

	// Find the step to approve
	stepIdx := -1
	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			stepIdx = idx
			break
		}
	}
	if stepIdx < 0 {
		return &api.ApproveDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	// Verify the step is in Waiting status
	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.ApproveDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	// Validate required inputs from wait step config
	if err := validateRequiredInputs(dagStatus.Nodes[stepIdx].Step, request.Body); err != nil {
		return &api.ApproveDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: err.Error(),
		}, nil
	}

	// Update the node status to Succeeded
	dagStatus.Nodes[stepIdx].Status = core.NodeSucceeded
	dagStatus.Nodes[stepIdx].ApprovedAt = time.Now().Format(time.RFC3339)

	// Record who approved
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		dagStatus.Nodes[stepIdx].ApprovedBy = user.Username
	}

	// Store approval inputs in OutputVariables (original keys for env vars)
	// and ApprovalInputs (camelCase for UI display)
	if request.Body != nil && request.Body.Inputs != nil {
		// Store in OutputVariables with original keys for env vars
		if dagStatus.Nodes[stepIdx].OutputVariables == nil {
			dagStatus.Nodes[stepIdx].OutputVariables = &collections.SyncMap{}
		}
		camelInputs := make(map[string]string, len(*request.Body.Inputs))
		for k, v := range *request.Body.Inputs {
			dagStatus.Nodes[stepIdx].OutputVariables.Store(k, k+"="+v)
			camelInputs[stringutil.ScreamingSnakeToCamel(k)] = v
		}
		dagStatus.Nodes[stepIdx].ApprovalInputs = camelInputs
	}

	// Save the updated status
	if err := a.dagRunMgr.UpdateStatus(ctx, ref, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	// Check if DAG should be resumed (no more waiting steps blocking)
	shouldResume := true
	for _, n := range dagStatus.Nodes {
		if n.Status == core.NodeWaiting {
			shouldResume = false
			break
		}
	}

	if shouldResume {
		// Re-enqueue the DAG run for execution
		attempt, err := a.dagRunStore.FindAttempt(ctx, ref)
		if err != nil {
			logger.Error(ctx, "Failed to find attempt for resume", tag.Error(err))
			return &api.ApproveDAGRunStep200JSONResponse{
				DagRunId: request.DagRunId,
				StepName: request.StepName,
				Resumed:  false,
			}, nil
		}

		dag, err := attempt.ReadDAG(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to read DAG for resume", tag.Error(err))
			return &api.ApproveDAGRunStep200JSONResponse{
				DagRunId: request.DagRunId,
				StepName: request.StepName,
				Resumed:  false,
			}, nil
		}

		// Use retry to resume the DAG from where it left off
		retrySpec := a.subCmdBuilder.Retry(dag, request.DagRunId, "")
		if err := runtime.Start(ctx, retrySpec); err != nil {
			logger.Error(ctx, "Failed to resume DAG", tag.Error(err))
			return &api.ApproveDAGRunStep200JSONResponse{
				DagRunId: request.DagRunId,
				StepName: request.StepName,
				Resumed:  false,
			}, nil
		}

		logger.Info(ctx, "DAG resumed after approval",
			slog.String("dagRunId", request.DagRunId),
			slog.String("step", request.StepName),
		)
	}

	return &api.ApproveDAGRunStep200JSONResponse{
		DagRunId: request.DagRunId,
		StepName: request.StepName,
		Resumed:  shouldResume,
	}, nil
}

func (a *API) ApproveSubDAGRunStep(ctx context.Context, request api.ApproveSubDAGRunStepRequestObject) (api.ApproveSubDAGRunStepResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	// Get the sub DAG run status
	rootRef := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, rootRef, request.SubDAGRunId)
	if err != nil {
		return &api.ApproveSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub DAG-run ID %s not found", request.SubDAGRunId),
		}, nil
	}

	// Find the step to approve
	stepIdx := -1
	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			stepIdx = idx
			break
		}
	}
	if stepIdx < 0 {
		return &api.ApproveSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in sub DAG-run %s", request.StepName, request.SubDAGRunId),
		}, nil
	}

	// Verify the step is in Waiting status
	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.ApproveSubDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	// Validate required inputs from wait step config
	if err := validateRequiredInputs(dagStatus.Nodes[stepIdx].Step, request.Body); err != nil {
		return &api.ApproveSubDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: err.Error(),
		}, nil
	}

	// Update the node status to Succeeded
	dagStatus.Nodes[stepIdx].Status = core.NodeSucceeded
	dagStatus.Nodes[stepIdx].ApprovedAt = time.Now().Format(time.RFC3339)

	// Record who approved
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		dagStatus.Nodes[stepIdx].ApprovedBy = user.Username
	}

	// Store approval inputs in OutputVariables (original keys for env vars)
	// and ApprovalInputs (camelCase for UI display)
	if request.Body != nil && request.Body.Inputs != nil {
		// Store in OutputVariables with original keys for env vars
		if dagStatus.Nodes[stepIdx].OutputVariables == nil {
			dagStatus.Nodes[stepIdx].OutputVariables = &collections.SyncMap{}
		}
		camelInputs := make(map[string]string, len(*request.Body.Inputs))
		for k, v := range *request.Body.Inputs {
			dagStatus.Nodes[stepIdx].OutputVariables.Store(k, k+"="+v)
			camelInputs[stringutil.ScreamingSnakeToCamel(k)] = v
		}
		dagStatus.Nodes[stepIdx].ApprovalInputs = camelInputs
	}

	// Save the updated status
	if err := a.dagRunMgr.UpdateStatus(ctx, rootRef, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating sub DAG-run status: %w", err)
	}

	// Check if sub DAG should be resumed (no more waiting steps blocking)
	shouldResume := true
	for _, n := range dagStatus.Nodes {
		if n.Status == core.NodeWaiting {
			shouldResume = false
			break
		}
	}

	if shouldResume {
		// Re-enqueue the sub DAG run for execution
		attempt, err := a.dagRunStore.FindSubAttempt(ctx, rootRef, request.SubDAGRunId)
		if err != nil {
			logger.Error(ctx, "Failed to find sub DAG attempt for resume", tag.Error(err))
			return &api.ApproveSubDAGRunStep200JSONResponse{
				DagRunId: request.SubDAGRunId,
				StepName: request.StepName,
				Resumed:  false,
			}, nil
		}

		dag, err := attempt.ReadDAG(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to read sub DAG for resume", tag.Error(err))
			return &api.ApproveSubDAGRunStep200JSONResponse{
				DagRunId: request.SubDAGRunId,
				StepName: request.StepName,
				Resumed:  false,
			}, nil
		}

		// Use retry to resume the sub DAG from where it left off
		retrySpec := a.subCmdBuilder.Retry(dag, request.SubDAGRunId, "")
		if err := runtime.Start(ctx, retrySpec); err != nil {
			logger.Error(ctx, "Failed to resume sub DAG", tag.Error(err))
			return &api.ApproveSubDAGRunStep200JSONResponse{
				DagRunId: request.SubDAGRunId,
				StepName: request.StepName,
				Resumed:  false,
			}, nil
		}

		logger.Info(ctx, "Sub DAG resumed after approval",
			slog.String("subDagRunId", request.SubDAGRunId),
			slog.String("step", request.StepName),
		)
	}

	return &api.ApproveSubDAGRunStep200JSONResponse{
		DagRunId: request.SubDAGRunId,
		StepName: request.StepName,
		Resumed:  shouldResume,
	}, nil
}

// RejectDAGRunStep rejects a waiting step for HITL (Human-in-the-Loop).
func (a *API) RejectDAGRunStep(ctx context.Context, request api.RejectDAGRunStepRequestObject) (api.RejectDAGRunStepResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	ref := execution.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.RejectDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}

	// Find the step to reject
	stepIdx := -1
	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			stepIdx = idx
			break
		}
	}
	if stepIdx < 0 {
		return &api.RejectDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	// Verify the step is in Waiting status
	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.RejectDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	// Update the node status to Rejected
	dagStatus.Nodes[stepIdx].Status = core.NodeRejected
	dagStatus.Nodes[stepIdx].RejectedAt = time.Now().Format(time.RFC3339)

	// Record who rejected
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		dagStatus.Nodes[stepIdx].RejectedBy = user.Username
	}

	// Store rejection reason if provided
	if request.Body != nil && request.Body.Reason != nil {
		dagStatus.Nodes[stepIdx].RejectionReason = *request.Body.Reason
	}

	// Save the updated status
	if err := a.dagRunMgr.UpdateStatus(ctx, ref, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	logger.Info(ctx, "Step rejected",
		slog.String("dagRunId", request.DagRunId),
		slog.String("step", request.StepName),
	)

	return &api.RejectDAGRunStep200JSONResponse{
		DagRunId: request.DagRunId,
		StepName: request.StepName,
	}, nil
}

// RejectSubDAGRunStep rejects a waiting step in a sub DAG-run.
func (a *API) RejectSubDAGRunStep(ctx context.Context, request api.RejectSubDAGRunStepRequestObject) (api.RejectSubDAGRunStepResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	// Find the sub DAG run
	ref := execution.NewDAGRunRef(request.Name, request.DagRunId)
	subRef := execution.NewDAGRunRef("", request.SubDAGRunId)

	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, subRef)
	if err != nil {
		// Try to find via parent
		parentStatus, parentErr := a.dagRunMgr.GetSavedStatus(ctx, ref)
		if parentErr != nil {
			return &api.RejectSubDAGRunStep404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("sub dag-run ID %s not found", request.SubDAGRunId),
			}, nil
		}
		_ = parentStatus
		return &api.RejectSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found", request.SubDAGRunId),
		}, nil
	}

	// Find the step to reject
	stepIdx := -1
	for idx, n := range dagStatus.Nodes {
		if n.Step.Name == request.StepName {
			stepIdx = idx
			break
		}
	}
	if stepIdx < 0 {
		return &api.RejectSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in sub DAG-run %s", request.StepName, request.SubDAGRunId),
		}, nil
	}

	// Verify the step is in Waiting status
	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.RejectSubDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	// Update the node status to Rejected
	dagStatus.Nodes[stepIdx].Status = core.NodeRejected
	dagStatus.Nodes[stepIdx].RejectedAt = time.Now().Format(time.RFC3339)

	// Record who rejected
	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		dagStatus.Nodes[stepIdx].RejectedBy = user.Username
	}

	// Store rejection reason if provided
	if request.Body != nil && request.Body.Reason != nil {
		dagStatus.Nodes[stepIdx].RejectionReason = *request.Body.Reason
	}

	// Save the updated status
	if err := a.dagRunMgr.UpdateStatus(ctx, subRef, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	logger.Info(ctx, "Sub DAG step rejected",
		slog.String("subDagRunId", request.SubDAGRunId),
		slog.String("step", request.StepName),
	)

	return &api.RejectSubDAGRunStep200JSONResponse{
		DagRunId: request.SubDAGRunId,
		StepName: request.StepName,
	}, nil
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

// GetDAGRunSpec implements api.StrictServerInterface.
// This endpoint returns the YAML spec that was used for a specific DAG-run.
// It reads from the DAG-run attempt's YamlData field to ensure we return
// the exact spec used at execution time, not the current spec.
func (a *API) GetDAGRunSpec(ctx context.Context, request api.GetDAGRunSpecRequestObject) (api.GetDAGRunSpecResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	// Handle "latest" by getting the most recent attempt
	if dagRunId == "latest" {
		attempt, err := a.dagRunStore.LatestAttempt(ctx, dagName)
		if err != nil {
			return &api.GetDAGRunSpec404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("no dag-runs found for DAG %s", dagName),
			}, nil
		}
		dag, err := attempt.ReadDAG(ctx)
		if err != nil || dag == nil || len(dag.YamlData) == 0 {
			return &api.GetDAGRunSpec404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("DAG spec not found for %s", dagName),
			}, nil
		}
		return &api.GetDAGRunSpec200JSONResponse{
			Spec: string(dag.YamlData),
		}, nil
	}

	// Get spec from the specific DAG-run attempt
	attempt, err := a.dagRunStore.FindAttempt(ctx, execution.NewDAGRunRef(dagName, dagRunId))
	if err != nil {
		return &api.GetDAGRunSpec404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil || dag == nil || len(dag.YamlData) == 0 {
		return &api.GetDAGRunSpec404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("DAG spec not found for dag-run %s", dagRunId),
		}, nil
	}

	return &api.GetDAGRunSpec200JSONResponse{
		Spec: string(dag.YamlData),
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
		Head:     valueOf(request.Params.Head),
		Tail:     valueOf(request.Params.Tail),
		Offset:   valueOf(request.Params.Offset),
		Limit:    valueOf(request.Params.Limit),
		Encoding: a.logEncodingCharset,
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
		Head:     valueOf(request.Params.Head),
		Tail:     valueOf(request.Params.Tail),
		Offset:   valueOf(request.Params.Offset),
		Limit:    valueOf(request.Params.Limit),
		Encoding: a.logEncodingCharset,
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
	if err := a.requireExecute(ctx); err != nil {
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
	api.NodeStatusAborted:    core.NodeAborted,
	api.NodeStatusSuccess:    core.NodeSucceeded,
	api.NodeStatusSkipped:    core.NodeSkipped,
	api.NodeStatusWaiting:    core.NodeWaiting,
	api.NodeStatusRejected:   core.NodeRejected,
}

func (a *API) RetryDAGRun(ctx context.Context, request api.RetryDAGRunRequestObject) (api.RetryDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
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

	if request.Body.StepName != nil && *request.Body.StepName != "" {
		spec := a.subCmdBuilder.Retry(dag, request.Body.DagRunId, *request.Body.StepName)
		if err := runtime.Start(ctx, spec); err != nil {
			return nil, fmt.Errorf("error retrying DAG step: %w", err)
		}
		return api.RetryDAGRun200Response{}, nil
	}

	spec := a.subCmdBuilder.Retry(dag, request.Body.DagRunId, "")
	if err := runtime.Start(ctx, spec); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	return api.RetryDAGRun200Response{}, nil
}

func (a *API) TerminateDAGRun(ctx context.Context, request api.TerminateDAGRunRequestObject) (api.TerminateDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
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
	if err := a.requireExecute(ctx); err != nil {
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

func (a *API) RescheduleDAGRun(ctx context.Context, request api.RescheduleDAGRunRequestObject) (api.RescheduleDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
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

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read DAG snapshot: %w", err)
	}

	dag.Params = status.ParamsList

	var (
		nameOverride string
		newDagRunID  string
		// Enforce singleton mode for rescheduled DAG runs to prevent duplicate runs of the same workflow execution.
		singleton = true
	)

	if body := request.Body; body != nil {
		if body.DagName != nil && *body.DagName != "" {
			nameOverride = *body.DagName
			if err := core.ValidateDAGName(nameOverride); err != nil {
				return nil, &Error{
					HTTPStatus: http.StatusBadRequest,
					Code:       api.ErrorCodeBadRequest,
					Message:    err.Error(),
				}
			}
			dag.Name = nameOverride
		}
		if body.DagRunId != nil && *body.DagRunId != "" {
			newDagRunID = *body.DagRunId
		}
	}

	if newDagRunID == "" {
		id, genErr := a.dagRunMgr.GenDAGRunID(ctx)
		if genErr != nil {
			return nil, fmt.Errorf("error generating dag-run ID: %w", genErr)
		}
		newDagRunID = id
	}

	if err := a.ensureDAGRunIDUnique(ctx, dag, newDagRunID); err != nil {
		return nil, err
	}

	logger.Info(ctx, "Rescheduling dag-run",
		tag.DAG(dag.Name),
		slog.String("from-dag-run-id", request.DagRunId),
		tag.RunID(newDagRunID),
		slog.Bool("singleton", singleton))

	if err := a.startDAGRunWithOptions(ctx, dag, startDAGRunOptions{
		dagRunID:     newDagRunID,
		nameOverride: nameOverride,
		fromRunID:    request.DagRunId,
		target:       request.Name,
		singleton:    singleton,
	}); err != nil {
		return nil, fmt.Errorf("failed to start dag-run: %w", err)
	}

	queued := false
	if dagStatus, _ := a.dagRunMgr.GetCurrentStatus(ctx, dag, newDagRunID); dagStatus != nil {
		queued = dagStatus.Status == core.Queued
	}

	return api.RescheduleDAGRun200JSONResponse{
		DagRunId: newDagRunID,
		Queued:   queued,
	}, nil
}

// GetSubDAGRuns returns timing and status information for all sub DAG runs.
// When parentSubDAGRunId is provided, it returns sub-runs of that specific sub DAG run
// (for multi-level nested DAGs).
func (a *API) GetSubDAGRuns(ctx context.Context, request api.GetSubDAGRunsRequestObject) (api.GetSubDAGRunsResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId
	parentSubDAGRunId := request.Params.ParentSubDAGRunId

	// The root reference is always used for storage lookups
	rootRef := execution.NewDAGRunRef(dagName, dagRunId)

	var dagStatus *execution.DAGRunStatus
	var err error

	if parentSubDAGRunId != nil && *parentSubDAGRunId != "" {
		// For multi-level nested DAGs: get the status of the parent sub DAG run
		dagStatus, err = a.dagRunMgr.FindSubDAGRunStatus(ctx, rootRef, *parentSubDAGRunId)
		if err != nil {
			return &api.GetSubDAGRuns404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("sub dag-run ID %s not found for root DAG %s/%s", *parentSubDAGRunId, dagName, dagRunId),
			}, nil
		}
	} else {
		// Default: get the root DAG run status
		dagStatus, err = a.dagRunMgr.GetSavedStatus(ctx, rootRef)
		if err != nil {
			return &api.GetSubDAGRuns404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
			}, nil
		}
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
			detail, err := a.getSubDAGRunDetail(ctx, rootRef, subRun.DAGRunID, subRun.Params)
			if err != nil {
				// Skip if we can't fetch details
				continue
			}
			subRuns = append(subRuns, detail)
		}

		// Collect repeated sub runs
		for _, subRun := range node.SubRunsRepeated {
			detail, err := a.getSubDAGRunDetail(ctx, rootRef, subRun.DAGRunID, subRun.Params)
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

// validateRequiredInputs checks that all required inputs from a wait step config are provided.
func validateRequiredInputs(step core.Step, body *api.ApproveStepRequest) error {
	if step.ExecutorConfig.Config == nil {
		return nil
	}

	// Extract required fields from step config
	requiredFields, ok := step.ExecutorConfig.Config["required"]
	if !ok {
		return nil
	}

	required, ok := requiredFields.([]any)
	if !ok {
		return nil
	}

	if len(required) == 0 {
		return nil
	}

	// Get provided inputs
	var providedInputs map[string]string
	if body != nil && body.Inputs != nil {
		providedInputs = *body.Inputs
	}

	// Check each required field
	var missing []string
	for _, r := range required {
		fieldName, ok := r.(string)
		if !ok {
			continue
		}
		if providedInputs == nil {
			missing = append(missing, fieldName)
			continue
		}
		if _, exists := providedInputs[fieldName]; !exists {
			missing = append(missing, fieldName)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required inputs: %v", missing)
	}

	return nil
}
