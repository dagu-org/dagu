// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	exec1 "github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var errParallelCancelled = errors.New("parallel execution cancelled")

var _ executor.ParallelExecutor = (*parallelExecutor)(nil)
var _ executor.NodeStatusDeterminer = (*parallelExecutor)(nil)

type parallelExecutor struct {
	child         *executor.SubDAGExecutor
	lock          sync.Mutex
	workDir       string
	stdout        io.Writer
	stderr        io.Writer
	runParamsList []executor.RunParams
	maxConcurrent int

	// Runtime state
	results map[string]*exec1.RunStatus // Maps DAG run ID to result
	errors  []error                     // Collects errors from failed executions

	cancel     chan struct{}
	cancelOnce sync.Once
}

type scheduledAttempt struct {
	runParams executor.RunParams
	stepName  string
	readyAt   time.Time
}

type attemptResult struct {
	attempt scheduledAttempt
	result  *exec1.RunStatus
	err     error
}

func newParallelExecutor(
	ctx context.Context, step core.Step,
) (executor.Executor, error) {
	// The parallel executor doesn't use the params from the step directly
	// as they are passed through SetParamsList

	if step.SubDAG == nil {
		return nil, fmt.Errorf("sub DAG configuration is missing")
	}

	child, err := executor.NewSubDAGExecutor(ctx, step.SubDAG.Name)
	if err != nil {
		return nil, err
	}

	// Validate: sub-DAGs with approval steps cannot be dispatched to workers
	if len(step.WorkerSelector) > 0 && child.DAG.HasApprovalSteps() {
		return nil, fmt.Errorf("%w: %s", ErrApprovalStepsWithWorker, step.SubDAG.Name)
	}
	child.SetExternalStepRetry(true)

	dir := runtime.GetEnv(ctx).WorkingDir
	if dir != "" && !fileutil.FileExists(dir) {
		return nil, ErrWorkingDirNotExist
	}

	maxConcurrent := core.DefaultMaxConcurrent
	if step.Parallel != nil && step.Parallel.MaxConcurrent > 0 {
		maxConcurrent = step.Parallel.MaxConcurrent
	}

	return &parallelExecutor{
		child:         child,
		workDir:       dir,
		maxConcurrent: maxConcurrent,
		results:       make(map[string]*exec1.RunStatus),
		errors:        make([]error, 0),
		cancel:        make(chan struct{}),
	}, nil
}

func (e *parallelExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup sub DAG executor", tag.Error(err))
		}
	}()

	if len(e.runParamsList) == 0 {
		return fmt.Errorf("no sub DAG runs to execute")
	}

	// Channel to collect errors from goroutines
	errChan := make(chan error, len(e.runParamsList))

	logger.Info(ctx, "Starting parallel execution",
		slog.Int("total", len(e.runParamsList)),
		slog.Int("max-concurrent", e.maxConcurrent),
		tag.DAG(e.child.DAG.Name),
	)

	pending := make([]scheduledAttempt, 0, len(e.runParamsList))
	pendingSet := make(map[string]struct{}, len(e.runParamsList))
	busyRuns := make(map[string]struct{}, len(e.runParamsList))
	for _, params := range e.runParamsList {
		attempt := scheduledAttempt{runParams: params}
		pending = append(pending, attempt)
		pendingSet[pendingAttemptKey(attempt)] = struct{}{}
	}

	resultCh := make(chan attemptResult, len(e.runParamsList))
	inFlight := 0

	for len(pending) > 0 || inFlight > 0 {
		now := time.Now()

		for e.maxConcurrent == 0 || inFlight < e.maxConcurrent {
			idx := nextRunnableAttemptIndex(pending, now, busyRuns)
			if idx < 0 {
				break
			}

			attempt := pending[idx]
			pending = append(pending[:idx], pending[idx+1:]...)
			delete(pendingSet, pendingAttemptKey(attempt))
			busyRuns[attempt.runParams.RunID] = struct{}{}
			inFlight++

			go func(a scheduledAttempt) {
				res, err := e.runAttempt(ctx, a)
				resultCh <- attemptResult{attempt: a, result: res, err: err}
			}(attempt)
		}

		if len(pending) == 0 && inFlight == 0 {
			break
		}

		var timer *time.Timer
		if delay, ok := nextPendingDelay(pending, busyRuns, time.Now()); ok && delay > 0 {
			timer = time.NewTimer(delay)
		}

		select {
		case res := <-resultCh:
			inFlight--
			delete(busyRuns, res.attempt.runParams.RunID)

			if res.result != nil {
				e.lock.Lock()
				e.results[res.attempt.runParams.RunID] = res.result
				e.lock.Unlock()
			}

			if res.err != nil {
				logger.Error(ctx, "Sub DAG execution failed",
					tag.RunID(res.attempt.runParams.RunID),
					tag.Error(res.err),
				)
			}

			if res.result != nil && len(res.result.PendingStepRetries) > 0 {
				scheduledAt := time.Now()
				for _, retry := range res.result.PendingStepRetries {
					next := scheduledAttempt{
						runParams: res.attempt.runParams,
						stepName:  retry.StepName,
						readyAt:   scheduledAt.Add(retry.Interval),
					}
					key := pendingAttemptKey(next)
					if _, exists := pendingSet[key]; exists {
						continue
					}
					pending = append(pending, next)
					pendingSet[key] = struct{}{}
				}
			} else if res.err != nil {
				errChan <- fmt.Errorf("sub DAG %s failed: %w", res.attempt.runParams.RunID, res.err)
			}

		case <-e.cancel:
			if timer != nil {
				timer.Stop()
			}
			errChan <- errParallelCancelled
			close(errChan)
			return errParallelCancelled

		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			errChan <- ctx.Err()
			close(errChan)
			return ctx.Err()

		case <-func() <-chan time.Time {
			if timer == nil {
				return nil
			}
			return timer.C
		}():
		}

		if timer != nil {
			timer.Stop()
		}
	}

	close(errChan)

	// Collect all errors
	for err := range errChan {
		e.errors = append(e.errors, err)
	}

	// Always output aggregated results, even if some executions failed
	if err := e.outputResults(); err != nil {
		// Log the output error but don't fail the entire execution because of it
		logger.Error(ctx, "Failed to output results", tag.Error(err))
	}

	// Check if any executions failed
	if len(e.errors) > 0 {
		// Check if any error is due to context cancellation
		for _, err := range e.errors {
			if errors.Is(err, context.Canceled) {
				return fmt.Errorf("parallel execution cancelled")
			}
		}
		return fmt.Errorf("parallel execution failed with %d errors: %v", len(e.errors), e.errors[0])
	}

	// Check if any sub DAGs failed (even if they completed without execution errors)
	// Wait status is not treated as failure - DetermineNodeStatus handles it
	e.lock.Lock()
	failedCount := 0
	for _, result := range e.results {
		if !result.Status.IsSuccess() && result.Status != core.Waiting {
			failedCount++
		}
	}
	e.lock.Unlock()

	if failedCount > 0 {
		return fmt.Errorf("parallel execution failed: %d sub dag(s) failed", failedCount)
	}

	return nil
}

func (e *parallelExecutor) SetParamsList(paramsList []executor.RunParams) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.runParamsList = paramsList
}

func (e *parallelExecutor) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *parallelExecutor) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

// DetermineNodeStatus implements NodeStatusDeterminer.
func (e *parallelExecutor) DetermineNodeStatus() (core.NodeStatus, error) {
	if len(e.results) == 0 {
		if len(e.errors) > 0 {
			return core.NodeFailed, fmt.Errorf(
				"all %d sub DAG execution(s) failed; first error: %v",
				len(e.runParamsList), e.errors[0],
			)
		}
		return core.NodeFailed, fmt.Errorf("no results available for node status determination")
	}

	// Check if all sub DAGs succeeded or if any had partial success or waiting
	// For error cases, we return an error status with error message
	var (
		partialSuccess bool
		hasWaiting     bool
	)
	for _, result := range e.results {
		switch result.Status {
		case core.Succeeded:
			// continue checking other results
		case core.PartiallySucceeded:
			partialSuccess = true
		case core.Waiting:
			// Sub-DAG is waiting for human approval
			hasWaiting = true
		default:
			return core.NodeFailed, fmt.Errorf("sub DAG run %s is still in progress with status: %s", result.DAGRunID, result.Status)
		}
	}

	// If any sub-DAG is waiting, propagate the waiting status to the parent
	// This takes priority over partial success since we need human action
	if hasWaiting {
		return core.NodeWaiting, nil
	}

	// Check count of items equal to count of succeeded items
	if len(e.results) != len(e.runParamsList) {
		partialSuccess = true
	}

	if partialSuccess {
		return core.NodePartiallySucceeded, nil
	}

	return core.NodeSucceeded, nil
}

func (e *parallelExecutor) runAttempt(ctx context.Context, attempt scheduledAttempt) (*exec1.RunStatus, error) {
	if attempt.stepName != "" {
		return e.child.Retry(ctx, attempt.runParams, attempt.stepName, e.workDir)
	}
	return e.child.Execute(ctx, attempt.runParams, e.workDir)
}

func pendingAttemptKey(attempt scheduledAttempt) string {
	return attempt.runParams.RunID + ":" + attempt.stepName
}

func nextRunnableAttemptIndex(
	pending []scheduledAttempt,
	now time.Time,
	busyRuns map[string]struct{},
) int {
	bestIdx := -1
	for i, attempt := range pending {
		if _, busy := busyRuns[attempt.runParams.RunID]; busy {
			continue
		}
		if !attempt.readyAt.IsZero() && attempt.readyAt.After(now) {
			continue
		}
		if bestIdx == -1 || pending[bestIdx].readyAt.After(attempt.readyAt) {
			bestIdx = i
		}
	}
	return bestIdx
}

func nextPendingDelay(
	pending []scheduledAttempt,
	busyRuns map[string]struct{},
	now time.Time,
) (time.Duration, bool) {
	var (
		found bool
		min   time.Duration
	)
	for _, attempt := range pending {
		if _, busy := busyRuns[attempt.runParams.RunID]; busy {
			continue
		}
		delay := time.Until(attempt.readyAt)
		if attempt.readyAt.IsZero() || delay <= 0 {
			return 0, true
		}
		if !found || delay < min {
			min = delay
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return min, true
}

// outputResults aggregates and outputs all sub DAG results
func (e *parallelExecutor) outputResults() error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Create aggregated output
	output := struct {
		Summary struct {
			Total     int `json:"total"`
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
		} `json:"summary"`
		Results []exec1.RunStatus   `json:"results"`
		Outputs []map[string]string `json:"outputs"`
	}{}

	output.Summary.Total = len(e.runParamsList)
	output.Results = make([]exec1.RunStatus, 0, len(e.results))
	output.Outputs = make([]map[string]string, 0, len(e.results))

	// Collect results in order of runParamsList for consistency
	for _, params := range e.runParamsList {
		if result, ok := e.results[params.RunID]; ok {
			// Create a copy of the result to potentially modify it
			resultCopy := *result

			output.Results = append(output.Results, resultCopy)

			if result.Status.IsSuccess() {
				output.Summary.Succeeded++

				// Add output to the outputs array
				// Only include outputs from successful executions
				if result.Outputs != nil {
					output.Outputs = append(output.Outputs, result.Outputs)
				}
			} else {
				output.Summary.Failed++
			}
		}
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	// Add a newline at the end of the JSON output
	jsonData = append(jsonData, '\n')

	// Write to stdout
	if e.stdout != nil {
		if _, err := e.stdout.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write outputs: %w", err)
		}
	}

	return nil
}

func (e *parallelExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.cancelOnce.Do(func() {
		close(e.cancel)
	})
	if e.child != nil {
		return e.child.Kill(sig)
	}
	return nil
}

func init() {
	caps := core.ExecutorCapabilities{
		SubDAG:         true,
		WorkerSelector: true,
	}
	executor.RegisterExecutor(core.ExecutorTypeParallel, newParallelExecutor, nil, caps)
}
