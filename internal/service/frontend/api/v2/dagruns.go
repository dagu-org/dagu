package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/collections"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/dagu-org/dagu/internal/service/audit"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// filenameUnsafeChars matches characters that are unsafe in Content-Disposition filenames.
var filenameUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// sanitizeFilename replaces unsafe characters with underscores for Content-Disposition headers.
func sanitizeFilename(s string) string {
	return filenameUnsafeChars.ReplaceAllString(s, "_")
}

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

	// Log DAG run execution from spec
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]any{
			"dag_name":   dag.Name,
			"dag_run_id": dagRunId,
			"inline":     true,
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

	if _, err := a.dagRunStore.FindAttempt(ctx, exec.DAGRunRef{Name: dag.Name, ID: dagRunId}); !errors.Is(err, exec.ErrDAGRunIDNotFound) {
		return nil, &Error{
			HTTPStatus: http.StatusConflict,
			Code:       api.ErrorCodeAlreadyExists,
			Message:    fmt.Sprintf("dag-run ID %s already exists for DAG %s", dagRunId, dag.Name),
		}
	}

	if err := a.enqueueDAGRun(ctx, dag, params, dagRunId, valueOf(request.Body.Name)); err != nil {
		return nil, fmt.Errorf("error enqueuing dag-run: %w", err)
	}

	// Log DAG run enqueue from spec
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]any{
			"dag_name":   dag.Name,
			"dag_run_id": dagRunId,
			"inline":     true,
		}
		if params != "" {
			detailsMap["params"] = params
		}
		details, _ := json.Marshal(detailsMap)
		entry := audit.NewEntry(audit.CategoryDAG, "dag_enqueue", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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
	var opts []exec.ListDAGRunStatusesOption
	if request.Params.Status != nil {
		opts = append(opts, exec.WithStatuses([]core.Status{
			core.Status(*request.Params.Status),
		}))
	}
	if request.Params.FromDate != nil {
		dt := exec.NewUTC(time.Unix(*request.Params.FromDate, 0))
		opts = append(opts, exec.WithFrom(dt))
	}
	if request.Params.ToDate != nil {
		dt := exec.NewUTC(time.Unix(*request.Params.ToDate, 0))
		opts = append(opts, exec.WithTo(dt))
	}
	if request.Params.Name != nil {
		opts = append(opts, exec.WithName(*request.Params.Name))
	}
	if request.Params.DagRunId != nil {
		opts = append(opts, exec.WithDAGRunID(*request.Params.DagRunId))
	}

	if tags := parseCommaSeparatedTags(request.Params.Tags); len(tags) > 0 {
		opts = append(opts, exec.WithTags(tags))
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
	opts := []exec.ListDAGRunStatusesOption{
		exec.WithExactName(request.Name),
	}

	if request.Params.Status != nil {
		opts = append(opts, exec.WithStatuses([]core.Status{
			core.Status(*request.Params.Status),
		}))
	}
	if request.Params.FromDate != nil {
		dt := exec.NewUTC(time.Unix(*request.Params.FromDate, 0))
		opts = append(opts, exec.WithFrom(dt))
	}
	if request.Params.ToDate != nil {
		dt := exec.NewUTC(time.Unix(*request.Params.ToDate, 0))
		opts = append(opts, exec.WithTo(dt))
	}
	if request.Params.DagRunId != nil {
		opts = append(opts, exec.WithDAGRunID(*request.Params.DagRunId))
	}

	dagRuns, err := a.listDAGRuns(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	return api.ListDAGRunsByName200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

func (a *API) listDAGRuns(ctx context.Context, opts []exec.ListDAGRunStatusesOption) ([]api.DAGRunSummary, error) {
	statuses, err := a.dagRunStore.ListStatuses(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	dagRuns := make([]api.DAGRunSummary, 0, len(statuses))
	for _, status := range statuses {
		dagRuns = append(dagRuns, toDAGRunSummary(*status))
	}
	return dagRuns, nil
}

// parseCommaSeparatedTags parses a comma-separated string of tags into a slice.
// Tags are normalized to lowercase and deduplicated.
func parseCommaSeparatedTags(tagsParam *string) []string {
	if tagsParam == nil || *tagsParam == "" {
		return nil
	}

	parts := strings.Split(*tagsParam, ",")
	seen := make(map[string]struct{}, len(parts))
	tags := make([]string, 0, len(parts))
	for _, tag := range parts {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = struct{}{}
				tags = append(tags, normalized)
			}
		}
	}
	return tags
}

func (a *API) GetDAGRunLog(ctx context.Context, request api.GetDAGRunLogRequestObject) (api.GetDAGRunLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := exec.NewDAGRunRef(dagName, dagRunId)
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
		if errors.Is(err, os.ErrNotExist) {
			return api.GetDAGRunLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for dag-run %s", dagRunId),
			}, nil
		}
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

// DownloadDAGRunLog implements api.StrictServerInterface.
func (a *API) DownloadDAGRunLog(ctx context.Context, request api.DownloadDAGRunLogRequestObject) (api.DownloadDAGRunLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.DownloadDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	content, err := os.ReadFile(dagStatus.Log)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return api.DownloadDAGRunLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for dag-run %s", dagRunId),
			}, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", dagStatus.Log, err)
	}

	filename := fmt.Sprintf("%s-%s-scheduler.log", sanitizeFilename(dagName), sanitizeFilename(dagRunId))
	return api.DownloadDAGRunLog200TextResponse{
		Body: string(content),
		Headers: api.DownloadDAGRunLog200ResponseHeaders{
			ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", filename),
		},
	}, nil
}

// GetDAGRunOutputs implements api.StrictServerInterface.
func (a *API) GetDAGRunOutputs(ctx context.Context, request api.GetDAGRunOutputsRequestObject) (api.GetDAGRunOutputsResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	var attempt exec.DAGRunAttempt
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
		ref := exec.NewDAGRunRef(dagName, dagRunId)
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
		outputs = &exec.DAGRunOutputs{
			Metadata: exec.OutputsMetadata{},
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

	ref := exec.NewDAGRunRef(dagName, dagRunId)
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
		if strings.Contains(err.Error(), "file not found") {
			return api.GetDAGRunStepLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for step %s", request.StepName),
			}, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}

	return api.GetDAGRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// DownloadDAGRunStepLog implements api.StrictServerInterface.
func (a *API) DownloadDAGRunStepLog(ctx context.Context, request api.DownloadDAGRunStepLogRequestObject) (api.DownloadDAGRunStepLogResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.DownloadDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return api.DownloadDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	var logFile = node.Stdout
	streamName := "stdout"
	if request.Params.Stream != nil && *request.Params.Stream == api.StreamStderr {
		logFile = node.Stderr
		streamName = "stderr"
	}

	content, err := os.ReadFile(filepath.Clean(logFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return api.DownloadDAGRunStepLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for step %s", request.StepName),
			}, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}

	filename := fmt.Sprintf("%s-%s-%s-%s.log", sanitizeFilename(dagName), sanitizeFilename(dagRunId), sanitizeFilename(request.StepName), streamName)
	return api.DownloadDAGRunStepLog200TextResponse{
		Body: string(content),
		Headers: api.DownloadDAGRunStepLog200ResponseHeaders{
			ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", filename),
		},
	}, nil
}

func (a *API) UpdateDAGRunStepStatus(ctx context.Context, request api.UpdateDAGRunStepStatusRequestObject) (api.UpdateDAGRunStepStatusResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	ref := exec.NewDAGRunRef(request.Name, request.DagRunId)
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

	stepIdx := findStepByName(dagStatus.Nodes, request.StepName)
	if stepIdx < 0 {
		return &api.UpdateDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	dagStatus.Nodes[stepIdx].Status = nodeStatusMapping[request.Body.Status]

	if err := a.dagRunMgr.UpdateStatus(ctx, ref, *dagStatus); err != nil {
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

	ref := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.ApproveDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}

	stepIdx := findStepByName(dagStatus.Nodes, request.StepName)
	if stepIdx < 0 {
		return &api.ApproveDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.ApproveDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	if err := validateRequiredInputs(dagStatus.Nodes[stepIdx].Step, request.Body); err != nil {
		return &api.ApproveDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: err.Error(),
		}, nil
	}

	// Apply approval to the node
	applyApproval(ctx, dagStatus.Nodes[stepIdx], request.Body)

	if err := a.dagRunMgr.UpdateStatus(ctx, ref, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	// Resume DAG if no more waiting steps
	shouldResume := !hasWaitingSteps(dagStatus.Nodes)
	if shouldResume {
		if err := a.resumeDAGRun(ctx, ref, request.DagRunId); err != nil {
			logger.Error(ctx, "Failed to resume DAG", tag.Error(err))
			shouldResume = false
		} else {
			logger.Info(ctx, "DAG resumed after approval",
				slog.String("dagRunId", request.DagRunId),
				slog.String("step", request.StepName),
			)
		}
	}

	a.logStepApproval(ctx, request.Name, request.DagRunId, "", request.StepName, shouldResume)

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

	rootRef := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, rootRef, request.SubDAGRunId)
	if err != nil {
		return &api.ApproveSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub DAG-run ID %s not found", request.SubDAGRunId),
		}, nil
	}

	stepIdx := findStepByName(dagStatus.Nodes, request.StepName)
	if stepIdx < 0 {
		return &api.ApproveSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in sub DAG-run %s", request.StepName, request.SubDAGRunId),
		}, nil
	}

	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.ApproveSubDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	if err := validateRequiredInputs(dagStatus.Nodes[stepIdx].Step, request.Body); err != nil {
		return &api.ApproveSubDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: err.Error(),
		}, nil
	}

	applyApproval(ctx, dagStatus.Nodes[stepIdx], request.Body)

	if err := a.dagRunMgr.UpdateStatus(ctx, rootRef, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating sub DAG-run status: %w", err)
	}

	// Resume sub-DAG if no more waiting steps
	shouldResume := !hasWaitingSteps(dagStatus.Nodes)
	if shouldResume {
		if err := a.resumeSubDAGRun(ctx, rootRef, request.SubDAGRunId); err != nil {
			logger.Error(ctx, "Failed to resume sub DAG", tag.Error(err))
			shouldResume = false
		} else {
			logger.Info(ctx, "Sub DAG resumed after approval",
				slog.String("subDagRunId", request.SubDAGRunId),
				slog.String("step", request.StepName),
			)
		}
	}

	a.logStepApproval(ctx, request.Name, request.DagRunId, request.SubDAGRunId, request.StepName, shouldResume)

	return &api.ApproveSubDAGRunStep200JSONResponse{
		DagRunId: request.SubDAGRunId,
		StepName: request.StepName,
		Resumed:  shouldResume,
	}, nil
}

// GetDAGRunStepMessages retrieves chat messages for a specific step.
func (a *API) GetDAGRunStepMessages(ctx context.Context, request api.GetDAGRunStepMessagesRequestObject) (api.GetDAGRunStepMessagesResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetDAGRunStepMessages404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return api.GetDAGRunStepMessages404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, dagName),
		}, nil
	}

	// Get the attempt to read messages
	attempt, err := a.dagRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return api.GetDAGRunStepMessages404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run attempt not found for %s/%s", dagName, dagRunId),
		}, nil
	}

	// Read messages for the step
	messages, err := attempt.ReadStepMessages(ctx, request.StepName)
	if err != nil {
		return nil, fmt.Errorf("error reading messages: %w", err)
	}

	// Convert to API types
	apiMessages := toChatMessages(messages)

	// Get tool definitions from node state
	toolDefinitions := toToolDefinitions(node.ToolDefinitions)

	// Determine if more messages might arrive
	hasMore := node.Status == core.NodeRunning

	return api.GetDAGRunStepMessages200JSONResponse{
		Messages:        apiMessages,
		ToolDefinitions: toolDefinitions,
		StepStatus:      api.NodeStatus(node.Status),
		StepStatusLabel: api.NodeStatusLabel(node.Status.String()),
		HasMore:         hasMore,
	}, nil
}

// GetSubDAGRunStepMessages retrieves chat messages for a step in a sub DAG-run.
func (a *API) GetSubDAGRunStepMessages(ctx context.Context, request api.GetSubDAGRunStepMessagesRequestObject) (api.GetSubDAGRunStepMessagesResponseObject, error) {
	rootRef := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, rootRef, request.SubDAGRunId)
	if err != nil {
		return api.GetSubDAGRunStepMessages404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return api.GetSubDAGRunStepMessages404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in sub DAG-run %s", request.StepName, request.SubDAGRunId),
		}, nil
	}

	// Get the sub-attempt to read messages
	attempt, err := a.dagRunStore.FindSubAttempt(ctx, rootRef, request.SubDAGRunId)
	if err != nil {
		return api.GetSubDAGRunStepMessages404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run attempt not found for %s", request.SubDAGRunId),
		}, nil
	}

	// Read messages for the step
	messages, err := attempt.ReadStepMessages(ctx, request.StepName)
	if err != nil {
		return nil, fmt.Errorf("error reading messages: %w", err)
	}

	// Convert to API types
	apiMessages := toChatMessages(messages)

	// Get tool definitions from node state
	toolDefinitions := toToolDefinitions(node.ToolDefinitions)

	// Determine if more messages might arrive
	hasMore := node.Status == core.NodeRunning

	return api.GetSubDAGRunStepMessages200JSONResponse{
		Messages:        apiMessages,
		ToolDefinitions: toolDefinitions,
		StepStatus:      api.NodeStatus(node.Status),
		StepStatusLabel: api.NodeStatusLabel(node.Status.String()),
		HasMore:         hasMore,
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

	ref := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return &api.RejectDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("dag-run ID %s not found for DAG %s", request.DagRunId, request.Name),
		}, nil
	}

	stepIdx := findStepByName(dagStatus.Nodes, request.StepName)
	if stepIdx < 0 {
		return &api.RejectDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.RejectDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	var reason *string
	if request.Body != nil {
		reason = request.Body.Reason
	}
	applyRejection(ctx, dagStatus.Nodes[stepIdx], dagStatus, reason)

	if err := a.dagRunMgr.UpdateStatus(ctx, ref, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	logger.Info(ctx, "Step rejected",
		slog.String("dagRunId", request.DagRunId),
		slog.String("step", request.StepName),
	)

	a.logStepRejection(ctx, request.Name, request.DagRunId, "", request.StepName, reason)

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

	rootRef := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, rootRef, request.SubDAGRunId)
	if err != nil {
		return &api.RejectSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub DAG-run ID %s not found", request.SubDAGRunId),
		}, nil
	}

	stepIdx := findStepByName(dagStatus.Nodes, request.StepName)
	if stepIdx < 0 {
		return &api.RejectSubDAGRunStep404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in sub DAG-run %s", request.StepName, request.SubDAGRunId),
		}, nil
	}

	if dagStatus.Nodes[stepIdx].Status != core.NodeWaiting {
		return &api.RejectSubDAGRunStep400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: fmt.Sprintf("step %s is not waiting for approval (status: %s)", request.StepName, dagStatus.Nodes[stepIdx].Status),
		}, nil
	}

	var reason *string
	if request.Body != nil {
		reason = request.Body.Reason
	}
	applyRejection(ctx, dagStatus.Nodes[stepIdx], dagStatus, reason)

	if err := a.dagRunMgr.UpdateStatus(ctx, rootRef, *dagStatus); err != nil {
		return nil, fmt.Errorf("error updating sub DAG-run status: %w", err)
	}

	logger.Info(ctx, "Sub DAG step rejected",
		slog.String("subDagRunId", request.SubDAGRunId),
		slog.String("step", request.StepName),
	)

	a.logStepRejection(ctx, request.Name, request.DagRunId, request.SubDAGRunId, request.StepName, reason)

	return &api.RejectSubDAGRunStep200JSONResponse{
		DagRunId: request.SubDAGRunId,
		StepName: request.StepName,
	}, nil
}

// GetDAGRunDetails implements api.StrictServerInterface.
func (a *API) GetDAGRunDetails(ctx context.Context, request api.GetDAGRunDetailsRequestObject) (api.GetDAGRunDetailsResponseObject, error) {
	resp, err := a.getDAGRunDetailsData(ctx, request.Name, request.DagRunId)
	if err != nil {
		return &api.GetDAGRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: err.Error(),
		}, nil
	}
	return &resp, nil
}

// getDAGRunDetailsData returns DAG run details data. Used by both HTTP handler and SSE fetcher.
func (a *API) getDAGRunDetailsData(ctx context.Context, dagName, dagRunId string) (api.GetDAGRunDetails200JSONResponse, error) {
	if dagRunId == "latest" {
		attempt, err := a.dagRunStore.LatestAttempt(ctx, dagName)
		if err != nil {
			return api.GetDAGRunDetails200JSONResponse{}, fmt.Errorf("no dag-runs found for DAG %s", dagName)
		}
		status, err := attempt.ReadStatus(ctx)
		if err != nil {
			return api.GetDAGRunDetails200JSONResponse{}, fmt.Errorf("error getting latest status: %w", err)
		}
		return api.GetDAGRunDetails200JSONResponse{
			DagRunDetails: ToDAGRunDetails(*status),
		}, nil
	}

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return api.GetDAGRunDetails200JSONResponse{}, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}
	return api.GetDAGRunDetails200JSONResponse{
		DagRunDetails: ToDAGRunDetails(*dagStatus),
	}, nil
}

// GetDAGRunSpec implements api.StrictServerInterface.
// This endpoint returns the YAML spec that was used for a specific DAG-run.
// It reads from the DAG-run attempt's YamlData field to ensure we return
// the exact spec used at execution time, not the current spec.
func (a *API) GetDAGRunSpec(ctx context.Context, request api.GetDAGRunSpecRequestObject) (api.GetDAGRunSpecResponseObject, error) {
	dagName := request.Name
	dagRunId := request.DagRunId

	var attempt exec.DAGRunAttempt
	var err error
	var notFoundMsg string

	// Handle "latest" by getting the most recent attempt
	if dagRunId == "latest" {
		attempt, err = a.dagRunStore.LatestAttempt(ctx, dagName)
		notFoundMsg = fmt.Sprintf("no dag-runs found for DAG %s", dagName)
	} else {
		// Get spec from the specific DAG-run attempt
		attempt, err = a.dagRunStore.FindAttempt(ctx, exec.NewDAGRunRef(dagName, dagRunId))
		notFoundMsg = fmt.Sprintf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	if err != nil {
		return &api.GetDAGRunSpec404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: notFoundMsg,
		}, nil
	}

	spec, err := a.getSpecFromAttempt(ctx, attempt)
	if err != nil {
		return &api.GetDAGRunSpec404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("DAG spec not found for dag-run %s", dagRunId),
		}, nil
	}

	return &api.GetDAGRunSpec200JSONResponse{
		Spec: spec,
	}, nil
}

// GetSubDAGRunDetails implements api.StrictServerInterface.
func (a *API) GetSubDAGRunDetails(ctx context.Context, request api.GetSubDAGRunDetailsRequestObject) (api.GetSubDAGRunDetailsResponseObject, error) {
	root := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.GetSubDAGRunDetails404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}
	return &api.GetSubDAGRunDetails200JSONResponse{
		DagRunDetails: ToDAGRunDetails(*dagStatus),
	}, nil
}

// GetSubDAGRunSpec implements api.StrictServerInterface.
// This endpoint returns the YAML spec that was used for a specific sub-DAG run.
func (a *API) GetSubDAGRunSpec(ctx context.Context, request api.GetSubDAGRunSpecRequestObject) (api.GetSubDAGRunSpecResponseObject, error) {
	root := exec.NewDAGRunRef(request.Name, request.DagRunId)

	// Find the sub-DAG run attempt using the existing store method
	attempt, err := a.dagRunStore.FindSubAttempt(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.GetSubDAGRunSpec404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}

	spec, err := a.getSpecFromAttempt(ctx, attempt)
	if err != nil {
		return &api.GetSubDAGRunSpec404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("DAG spec not found for sub dag-run %s", request.SubDAGRunId),
		}, nil
	}

	return &api.GetSubDAGRunSpec200JSONResponse{
		Spec: spec,
	}, nil
}

// getSpecFromAttempt reads YAML spec from DAG run attempt.
// Returns spec string and nil error on success, or empty string and error on failure.
func (a *API) getSpecFromAttempt(ctx context.Context, attempt exec.DAGRunAttempt) (string, error) {
	dag, err := attempt.ReadDAG(ctx)
	if err != nil || dag == nil || len(dag.YamlData) == 0 {
		return "", fmt.Errorf("DAG spec not found")
	}
	return string(dag.YamlData), nil
}

// GetSubDAGRunLog implements api.StrictServerInterface.
func (a *API) GetSubDAGRunLog(ctx context.Context, request api.GetSubDAGRunLogRequestObject) (api.GetSubDAGRunLogResponseObject, error) {
	root := exec.NewDAGRunRef(request.Name, request.DagRunId)
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
		if strings.Contains(err.Error(), "file not found") {
			return &api.GetSubDAGRunLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for sub dag-run %s", request.SubDAGRunId),
			}, nil
		}
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

// DownloadSubDAGRunLog implements api.StrictServerInterface.
func (a *API) DownloadSubDAGRunLog(ctx context.Context, request api.DownloadSubDAGRunLogRequestObject) (api.DownloadSubDAGRunLogResponseObject, error) {
	root := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.DownloadSubDAGRunLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}

	content, err := os.ReadFile(dagStatus.Log)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &api.DownloadSubDAGRunLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for sub dag-run %s", request.SubDAGRunId),
			}, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", dagStatus.Log, err)
	}

	filename := fmt.Sprintf("%s-%s-sub-%s-scheduler.log", sanitizeFilename(request.Name), sanitizeFilename(request.DagRunId), sanitizeFilename(request.SubDAGRunId))
	return &api.DownloadSubDAGRunLog200TextResponse{
		Body: string(content),
		Headers: api.DownloadSubDAGRunLog200ResponseHeaders{
			ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", filename),
		},
	}, nil
}

// GetSubDAGRunStepLog implements api.StrictServerInterface.
func (a *API) GetSubDAGRunStepLog(ctx context.Context, request api.GetSubDAGRunStepLogRequestObject) (api.GetSubDAGRunStepLogResponseObject, error) {
	root := exec.NewDAGRunRef(request.Name, request.DagRunId)
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
		if strings.Contains(err.Error(), "file not found") {
			return &api.GetSubDAGRunStepLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for step %s", request.StepName),
			}, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}

	return &api.GetSubDAGRunStepLog200JSONResponse{
		Content:    content,
		LineCount:  ptrOf(lineCount),
		TotalLines: ptrOf(totalLines),
		HasMore:    ptrOf(hasMore),
		IsEstimate: ptrOf(isEstimate),
	}, nil
}

// DownloadSubDAGRunStepLog implements api.StrictServerInterface.
func (a *API) DownloadSubDAGRunStepLog(ctx context.Context, request api.DownloadSubDAGRunStepLogRequestObject) (api.DownloadSubDAGRunStepLogResponseObject, error) {
	root := exec.NewDAGRunRef(request.Name, request.DagRunId)
	dagStatus, err := a.dagRunMgr.FindSubDAGRunStatus(ctx, root, request.SubDAGRunId)
	if err != nil {
		return &api.DownloadSubDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("sub dag-run ID %s not found for DAG %s", request.SubDAGRunId, request.Name),
		}, nil
	}

	node, err := dagStatus.NodeByName(request.StepName)
	if err != nil {
		return &api.DownloadSubDAGRunStepLog404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	var logFile = node.Stdout
	streamName := "stdout"
	if request.Params.Stream != nil && *request.Params.Stream == api.StreamStderr {
		logFile = node.Stderr
		streamName = "stderr"
	}

	content, err := os.ReadFile(filepath.Clean(logFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &api.DownloadSubDAGRunStepLog404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: fmt.Sprintf("log file not found for step %s", request.StepName),
			}, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", logFile, err)
	}

	filename := fmt.Sprintf("%s-%s-sub-%s-%s-%s.log", sanitizeFilename(request.Name), sanitizeFilename(request.DagRunId), sanitizeFilename(request.SubDAGRunId), sanitizeFilename(request.StepName), streamName)
	return &api.DownloadSubDAGRunStepLog200TextResponse{
		Body: string(content),
		Headers: api.DownloadSubDAGRunStepLog200ResponseHeaders{
			ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", filename),
		},
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

	root := exec.NewDAGRunRef(request.Name, request.DagRunId)
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

	stepIdx := findStepByName(dagStatus.Nodes, request.StepName)
	if stepIdx < 0 {
		return &api.UpdateSubDAGRunStepStatus404JSONResponse{
			Code:    api.ErrorCodeNotFound,
			Message: fmt.Sprintf("step %s not found in DAG %s", request.StepName, request.Name),
		}, nil
	}

	dagStatus.Nodes[stepIdx].Status = nodeStatusMapping[request.Body.Status]

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

	attempt, err := a.dagRunStore.FindAttempt(ctx, exec.NewDAGRunRef(request.Name, request.DagRunId))
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

	stepName := valueOf(request.Body.StepName)

	// Check for workerSelector - dispatch to coordinator for distributed execution
	if len(dag.WorkerSelector) > 0 {
		if a.coordinatorCli == nil {
			return nil, &Error{
				HTTPStatus: http.StatusServiceUnavailable,
				Code:       api.ErrorCodeInternalError,
				Message:    "coordinator not configured for distributed DAG retry",
			}
		}

		// Get previous status for retry context
		prevStatus, err := attempt.ReadStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("error reading status: %w", err)
		}

		// Create and dispatch retry task to coordinator
		opts := []executor.TaskOption{
			executor.WithWorkerSelector(dag.WorkerSelector),
			executor.WithPreviousStatus(prevStatus),
		}
		if stepName != "" {
			opts = append(opts, executor.WithStep(stepName))
		}

		task := executor.CreateTask(
			dag.Name,
			string(dag.YamlData),
			coordinatorv1.Operation_OPERATION_RETRY,
			request.Body.DagRunId,
			opts...,
		)

		if err := a.coordinatorCli.Dispatch(ctx, task); err != nil {
			return nil, fmt.Errorf("error dispatching retry to coordinator: %w", err)
		}

		// Log distributed DAG retry
		a.logRetryAudit(ctx, request.Name, request.DagRunId, stepName, true)
		return api.RetryDAGRun200Response{}, nil
	}

	// Local retry path
	spec := a.subCmdBuilder.Retry(dag, request.Body.DagRunId, stepName)
	if err := runtime.Start(ctx, spec); err != nil {
		return nil, fmt.Errorf("error retrying DAG: %w", err)
	}

	// Log local DAG retry
	a.logRetryAudit(ctx, request.Name, request.DagRunId, stepName, false)
	return api.RetryDAGRun200Response{}, nil
}

// logRetryAudit logs a retry audit event.
func (a *API) logRetryAudit(ctx context.Context, dagName, dagRunID, stepName string, distributed bool) {
	if a.auditService == nil {
		return
	}

	currentUser, _ := auth.UserFromContext(ctx)
	clientIP, _ := auth.ClientIPFromContext(ctx)
	detailsMap := map[string]any{
		"dag_name":    dagName,
		"dag_run_id":  dagRunID,
		"distributed": distributed,
	}
	if stepName != "" {
		detailsMap["step_name"] = stepName
	}
	details, _ := json.Marshal(detailsMap)
	entry := audit.NewEntry(audit.CategoryDAG, "dag_retry", currentUser.ID, currentUser.Username).
		WithDetails(string(details)).
		WithIPAddress(clientIP)
	_ = a.auditService.Log(ctx, entry)
}

func (a *API) TerminateDAGRun(ctx context.Context, request api.TerminateDAGRunRequestObject) (api.TerminateDAGRunResponseObject, error) {
	if err := a.isAllowed(config.PermissionRunDAGs); err != nil {
		return nil, err
	}
	if err := a.requireExecute(ctx); err != nil {
		return nil, err
	}

	ref := exec.NewDAGRunRef(request.Name, request.DagRunId)
	attempt, err := a.dagRunStore.FindAttempt(ctx, ref)
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

	// Get saved status to check if it's a distributed DAG
	savedStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG status not found for %s", request.Name),
		}
	}

	// Check if it's a distributed DAG (has WorkerID)
	if savedStatus.WorkerID != "" {
		// For distributed DAGs, use saved status for running check
		if savedStatus.Status != core.Running {
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeNotRunning,
				Message:    "DAG is not running",
			}
		}
		// Send cancel request via coordinator
		if a.coordinatorCli == nil {
			return nil, &Error{
				HTTPStatus: http.StatusServiceUnavailable,
				Code:       api.ErrorCodeInternalError,
				Message:    "coordinator not configured for distributed DAG cancellation",
			}
		}
		if err := a.coordinatorCli.RequestCancel(ctx, request.Name, request.DagRunId, nil); err != nil {
			return nil, fmt.Errorf("error requesting cancel: %w", err)
		}
	} else {
		// For local DAGs, use existing logic with GetCurrentStatus and socket
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
	}

	// Log DAG termination
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{
			"dag_name":   request.Name,
			"dag_run_id": request.DagRunId,
		})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_terminate", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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

	dagRun := exec.NewDAGRunRef(request.Name, request.DagRunId)
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

	// Log DAG dequeue
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, _ := json.Marshal(map[string]string{
			"dag_name":   request.Name,
			"dag_run_id": request.DagRunId,
		})
		entry := audit.NewEntry(audit.CategoryDAG, "dag_dequeue", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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

	attempt, err := a.dagRunStore.FindAttempt(ctx, exec.NewDAGRunRef(request.Name, request.DagRunId))
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

	// Log DAG reschedule
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]any{
			"dag_name":        request.Name,
			"from_dag_run_id": request.DagRunId,
			"new_dag_run_id":  newDagRunID,
			"queued":          queued,
		}
		if nameOverride != "" {
			detailsMap["name_override"] = nameOverride
		}
		details, _ := json.Marshal(detailsMap)
		entry := audit.NewEntry(audit.CategoryDAG, "dag_reschedule", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
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
	rootRef := exec.NewDAGRunRef(dagName, dagRunId)

	var dagStatus *exec.DAGRunStatus
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
			detail, err := a.getSubDAGRunDetail(ctx, rootRef, subRun.DAGRunID, subRun.Params, subRun.DAGName)
			if err != nil {
				// Skip if we can't fetch details
				continue
			}
			subRuns = append(subRuns, detail)
		}

		// Collect repeated sub runs
		for _, subRun := range node.SubRunsRepeated {
			detail, err := a.getSubDAGRunDetail(ctx, rootRef, subRun.DAGRunID, subRun.Params, subRun.DAGName)
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
func (a *API) getSubDAGRunDetail(ctx context.Context, parentRef exec.DAGRunRef, subRunID string, params string, dagName string) (api.SubDAGRunDetail, error) {
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

	if dagName != "" {
		detail.DagName = &dagName
	}

	return detail, nil
}

// applyApproval updates a node with approval information.
func applyApproval(ctx context.Context, node *exec.Node, body *api.ApproveStepRequest) {
	node.Status = core.NodeSucceeded
	node.ApprovedAt = time.Now().Format(time.RFC3339)

	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		node.ApprovedBy = user.Username
	}

	if body != nil && body.Inputs != nil {
		if node.OutputVariables == nil {
			node.OutputVariables = &collections.SyncMap{}
		}
		camelInputs := make(map[string]string, len(*body.Inputs))
		for k, v := range *body.Inputs {
			node.OutputVariables.Store(k, k+"="+v)
			camelInputs[stringutil.ScreamingSnakeToCamel(k)] = v
		}
		node.ApprovalInputs = camelInputs
	}
}

// applyRejection updates a node and status with rejection information.
func applyRejection(ctx context.Context, node *exec.Node, status *exec.DAGRunStatus, reason *string) {
	node.Status = core.NodeRejected
	node.RejectedAt = time.Now().Format(time.RFC3339)

	if user, ok := auth.UserFromContext(ctx); ok && user != nil {
		node.RejectedBy = user.Username
	}

	if reason != nil {
		node.RejectionReason = *reason
	}

	status.Status = core.Rejected
	status.FinishedAt = time.Now().Format(time.RFC3339)
}

// resumeDAGRun attempts to resume a DAG run after approval.
func (a *API) resumeDAGRun(ctx context.Context, ref exec.DAGRunRef, dagRunID string) error {
	attempt, err := a.dagRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return fmt.Errorf("find attempt: %w", err)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("read DAG: %w", err)
	}

	retrySpec := a.subCmdBuilder.Retry(dag, dagRunID, "")
	return runtime.Start(ctx, retrySpec)
}

// resumeSubDAGRun attempts to resume a sub-DAG run after approval.
func (a *API) resumeSubDAGRun(ctx context.Context, rootRef exec.DAGRunRef, subDAGRunID string) error {
	attempt, err := a.dagRunStore.FindSubAttempt(ctx, rootRef, subDAGRunID)
	if err != nil {
		return fmt.Errorf("find sub-attempt: %w", err)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("read sub-DAG: %w", err)
	}

	retrySpec := a.subCmdBuilder.Retry(dag, subDAGRunID, "")
	return runtime.Start(ctx, retrySpec)
}

// logStepApproval logs a step approval audit event.
func (a *API) logStepApproval(ctx context.Context, dagName, dagRunID, subDAGRunID, stepName string, resumed bool) {
	if a.auditService == nil {
		return
	}

	currentUser, _ := auth.UserFromContext(ctx)
	clientIP, _ := auth.ClientIPFromContext(ctx)

	detailsMap := map[string]any{
		"dag_name":   dagName,
		"dag_run_id": dagRunID,
		"step_name":  stepName,
		"resumed":    resumed,
	}
	if subDAGRunID != "" {
		detailsMap["sub_dag_run_id"] = subDAGRunID
	}

	action := "dag_step_approve"
	if subDAGRunID != "" {
		action = "sub_dag_step_approve"
	}

	details, _ := json.Marshal(detailsMap)
	entry := audit.NewEntry(audit.CategoryDAG, action, currentUser.ID, currentUser.Username).
		WithDetails(string(details)).
		WithIPAddress(clientIP)
	_ = a.auditService.Log(ctx, entry)
}

// logStepRejection logs a step rejection audit event.
func (a *API) logStepRejection(ctx context.Context, dagName, dagRunID, subDAGRunID, stepName string, reason *string) {
	if a.auditService == nil {
		return
	}

	currentUser, _ := auth.UserFromContext(ctx)
	clientIP, _ := auth.ClientIPFromContext(ctx)

	detailsMap := map[string]any{
		"dag_name":   dagName,
		"dag_run_id": dagRunID,
		"step_name":  stepName,
	}
	if subDAGRunID != "" {
		detailsMap["sub_dag_run_id"] = subDAGRunID
	}
	if reason != nil {
		detailsMap["reason"] = *reason
	}

	action := "dag_step_reject"
	if subDAGRunID != "" {
		action = "sub_dag_step_reject"
	}

	details, _ := json.Marshal(detailsMap)
	entry := audit.NewEntry(audit.CategoryDAG, action, currentUser.ID, currentUser.Username).
		WithDetails(string(details)).
		WithIPAddress(clientIP)
	_ = a.auditService.Log(ctx, entry)
}

// findStepByName searches for a step by name in the given nodes.
// Returns the step index or -1 if not found.
func findStepByName(nodes []*exec.Node, stepName string) int {
	for idx, n := range nodes {
		if n.Step.Name == stepName {
			return idx
		}
	}
	return -1
}

// hasWaitingSteps checks if any node is in Waiting status.
func hasWaitingSteps(nodes []*exec.Node) bool {
	for _, n := range nodes {
		if n.Status == core.NodeWaiting {
			return true
		}
	}
	return false
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
	if !ok || len(required) == 0 {
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
		if providedInputs == nil || providedInputs[fieldName] == "" {
			missing = append(missing, fieldName)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required inputs: %v", missing)
	}

	return nil
}

// SSE Data Methods for DAG Runs

// DAGRunLogsResponse represents the response for DAG run logs SSE.
type DAGRunLogsResponse struct {
	SchedulerLog SchedulerLogInfo `json:"schedulerLog"`
	StepLogs     []StepLogInfo    `json:"stepLogs"`
}

// SchedulerLogInfo contains scheduler log metadata.
type SchedulerLogInfo struct {
	Content    string `json:"content"`
	LineCount  int    `json:"lineCount"`
	TotalLines int    `json:"totalLines"`
	HasMore    bool   `json:"hasMore"`
}

// StepLogInfo contains step log metadata.
type StepLogInfo struct {
	StepName    string         `json:"stepName"`
	Status      api.NodeStatus `json:"status"`
	StatusLabel string         `json:"statusLabel"`
	StartedAt   string         `json:"startedAt"`
	FinishedAt  string         `json:"finishedAt"`
	HasStdout   bool           `json:"hasStdout"`
	HasStderr   bool           `json:"hasStderr"`
}

// StepLogResponse represents the response for step log SSE.
type StepLogResponse struct {
	StdoutContent string `json:"stdoutContent"`
	StderrContent string `json:"stderrContent"`
	LineCount     int    `json:"lineCount"`
	TotalLines    int    `json:"totalLines"`
	HasMore       bool   `json:"hasMore"`
}

// GetDAGRunDetailsData returns DAG run details for SSE.
// Identifier format: "dagName/dagRunId"
func (a *API) GetDAGRunDetailsData(ctx context.Context, identifier string) (any, error) {
	dagName, dagRunId, ok := strings.Cut(identifier, "/")
	if !ok {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId')", identifier)
	}
	return a.getDAGRunDetailsData(ctx, dagName, dagRunId)
}

// GetDAGRunLogsData returns DAG run logs for SSE.
// Identifier format: "dagName/dagRunId" or "dagName/dagRunId?tail=N"
func (a *API) GetDAGRunLogsData(ctx context.Context, identifier string) (any, error) {
	// Parse query params if present
	pathPart := identifier
	var queryParams url.Values
	if idx := strings.Index(identifier, "?"); idx != -1 {
		pathPart = identifier[:idx]
		var err error
		queryParams, err = url.ParseQuery(identifier[idx+1:])
		if err != nil {
			logger.Warn(ctx, "Failed to parse query string in identifier",
				tag.Error(err),
				slog.String("identifier", identifier),
			)
		}
	}

	dagName, dagRunId, ok := strings.Cut(pathPart, "/")
	if !ok {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId')", identifier)
	}

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	// Parse tail parameter with bounds validation (100-10000, default 500)
	tail := 500
	if queryParams != nil {
		tail = clampInt(parseIntParam(queryParams.Get("tail"), 500), 100, 10000)
	}

	options := fileutil.LogReadOptions{
		Tail:     tail,
		Encoding: a.logEncodingCharset,
	}

	content, lineCount, totalLines, hasMore, _, err := fileutil.ReadLogContent(dagStatus.Log, options)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("error reading scheduler log: %w", err)
	}

	schedulerLog := SchedulerLogInfo{
		Content:    content,
		LineCount:  lineCount,
		TotalLines: totalLines,
		HasMore:    hasMore,
	}

	// Build step logs info
	stepLogs := make([]StepLogInfo, 0, len(dagStatus.Nodes))
	for _, node := range dagStatus.Nodes {
		stepLog := StepLogInfo{
			StepName:    node.Step.Name,
			Status:      api.NodeStatus(node.Status),
			StatusLabel: node.Status.String(),
			StartedAt:   node.StartedAt,
			FinishedAt:  node.FinishedAt,
			HasStdout:   node.Stdout != "" && fileExists(node.Stdout),
			HasStderr:   node.Stderr != "" && fileExists(node.Stderr),
		}
		stepLogs = append(stepLogs, stepLog)
	}

	return DAGRunLogsResponse{
		SchedulerLog: schedulerLog,
		StepLogs:     stepLogs,
	}, nil
}

// GetStepLogData returns step log for SSE.
// Identifier format: "dagName/dagRunId/stepName"
func (a *API) GetStepLogData(ctx context.Context, identifier string) (any, error) {
	parts := strings.SplitN(identifier, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid identifier format: %s (expected 'dagName/dagRunId/stepName')", identifier)
	}
	dagName, dagRunId, stepName := parts[0], parts[1], parts[2]

	ref := exec.NewDAGRunRef(dagName, dagRunId)
	dagStatus, err := a.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("dag-run ID %s not found for DAG %s", dagRunId, dagName)
	}

	node, err := dagStatus.NodeByName(stepName)
	if err != nil {
		return nil, fmt.Errorf("step %s not found in DAG %s", stepName, dagName)
	}

	options := fileutil.LogReadOptions{
		Tail:     1000, // Last 1000 lines by default
		Encoding: a.logEncodingCharset,
	}

	// Read stdout
	var stdoutContent string
	var lineCount, totalLines int
	var hasMore bool
	if node.Stdout != "" {
		stdoutContent, lineCount, totalLines, hasMore, _, err = fileutil.ReadLogContent(node.Stdout, options)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error reading stdout: %w", err)
		}
	}

	// Read stderr
	var stderrContent string
	if node.Stderr != "" {
		stderrContent, _, _, _, _, err = fileutil.ReadLogContent(node.Stderr, options)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// Log warning for real errors, return empty stderr
			logger.Warn(ctx, "Failed to read stderr log",
				tag.Error(err),
				slog.String("stepName", stepName),
				slog.String("stderrPath", node.Stderr),
			)
			stderrContent = ""
		}
	}

	return StepLogResponse{
		StdoutContent: stdoutContent,
		StderrContent: stderrContent,
		LineCount:     lineCount,
		TotalLines:    totalLines,
		HasMore:       hasMore,
	}, nil
}

// GetDAGRunsListData returns DAG runs list for SSE.
// Identifier format: URL query string (e.g., "status=running&name=mydag")
func (a *API) GetDAGRunsListData(ctx context.Context, queryString string) (any, error) {
	params, err := url.ParseQuery(queryString)
	if err != nil {
		logger.Warn(ctx, "Failed to parse query string for DAG runs list",
			tag.Error(err),
			slog.String("queryString", queryString),
		)
	}
	var opts []exec.ListDAGRunStatusesOption

	if status := params.Get("status"); status != "" {
		if statusInt, err := strconv.Atoi(status); err == nil {
			opts = append(opts, exec.WithStatuses([]core.Status{core.Status(statusInt)}))
		} else {
			logger.Warn(ctx, "Invalid status parameter",
				slog.String("status", status),
				tag.Error(err),
			)
		}
	}
	if fromDate := params.Get("fromDate"); fromDate != "" {
		if ts, err := strconv.ParseInt(fromDate, 10, 64); err == nil {
			opts = append(opts, exec.WithFrom(exec.NewUTC(time.Unix(ts, 0))))
		} else {
			logger.Warn(ctx, "Invalid fromDate parameter",
				slog.String("fromDate", fromDate),
				tag.Error(err),
			)
		}
	}
	if toDate := params.Get("toDate"); toDate != "" {
		if ts, err := strconv.ParseInt(toDate, 10, 64); err == nil {
			opts = append(opts, exec.WithTo(exec.NewUTC(time.Unix(ts, 0))))
		} else {
			logger.Warn(ctx, "Invalid toDate parameter",
				slog.String("toDate", toDate),
				tag.Error(err),
			)
		}
	}
	if name := params.Get("name"); name != "" {
		opts = append(opts, exec.WithName(name))
	}
	if dagRunId := params.Get("dagRunId"); dagRunId != "" {
		opts = append(opts, exec.WithDAGRunID(dagRunId))
	}
	if tags := params.Get("tags"); tags != "" {
		if tagList := parseCommaSeparatedTags(&tags); len(tagList) > 0 {
			opts = append(opts, exec.WithTags(tagList))
		}
	}

	statuses, err := a.dagRunStore.ListStatuses(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error listing dag-runs: %w", err)
	}

	dagRuns := make([]api.DAGRunSummary, 0, len(statuses))
	for _, status := range statuses {
		dagRuns = append(dagRuns, toDAGRunSummary(*status))
	}

	return api.ListDAGRuns200JSONResponse{
		DagRuns: dagRuns,
	}, nil
}

// clampInt restricts value to the range [minVal, maxVal].
func clampInt(value, minVal, maxVal int) int {
	return max(minVal, min(value, maxVal))
}

// fileExists returns true if the file at path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
