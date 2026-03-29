// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	"github.com/dagu-org/dagu/internal/runtime"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// TaskHandler defines the interface for executing tasks
type TaskHandler interface {
	Handle(ctx context.Context, task *coordinatorv1.Task) error
}

var _ TaskHandler = (*taskHandler)(nil)

// NewTaskHandler creates a new TaskHandler
func NewTaskHandler(cfg *config.Config) TaskHandler {
	return &taskHandler{
		subCmdBuilder: runtime.NewSubCmdBuilder(cfg),
		baseConfig:    cfg.Paths.BaseConfig,
	}
}

type taskHandler struct {
	subCmdBuilder *runtime.SubCmdBuilder
	baseConfig    string
}

// Handle runs the task using the dagrun.Manager.
func (e *taskHandler) Handle(ctx context.Context, task *coordinatorv1.Task) error {
	logger.Info(ctx, "Executing task",
		slog.String("operation", task.Operation.String()),
		tag.Target(task.Target),
		tag.RunID(task.DagRunId),
		slog.String("root-dag-run-id", task.RootDagRunId),
		slog.String("parent-dag-run-id", task.ParentDagRunId),
		slog.String("worker-id", task.WorkerId))

	logger.Info(ctx, "Creating temporary DAG file from definition",
		tag.DAG(task.Target),
		tag.Size(len(task.Definition)))

	tempFile, err := fileutil.CreateTempDAGFile("worker-dags", task.Target, []byte(task.Definition))
	if err != nil {
		return fmt.Errorf("failed to create temp DAG file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			logger.Errorf(ctx, "Failed to remove temp DAG file: %v", err)
		}
	}()

	originalTarget := task.Target
	task.Target = tempFile

	logger.Info(ctx, "Created temporary DAG file",
		tag.File(tempFile))

	spec, err := e.buildCommandSpec(ctx, task, originalTarget)
	if err != nil {
		return err
	}

	if err := runtime.Run(ctx, spec); err != nil {
		logger.Error(ctx, "Distributed task execution failed",
			slog.String("operation", task.Operation.String()),
			tag.Target(task.Target),
			tag.RunID(task.DagRunId),
			tag.Error(err))
		return err
	}

	logger.Info(ctx, "Distributed task execution finished",
		slog.String("operation", task.Operation.String()),
		tag.Target(task.Target),
		tag.RunID(task.DagRunId))

	return nil
}

func (e *taskHandler) buildCommandSpec(ctx context.Context, task *coordinatorv1.Task, originalTarget string) (runtime.CmdSpec, error) {
	dagName := dagNameHint(originalTarget)

	switch task.Operation {
	case coordinatorv1.Operation_OPERATION_START:
		hints, err := e.subprocessHints(ctx, task, originalTarget)
		if err != nil {
			return runtime.CmdSpec{}, err
		}
		return e.subCmdBuilder.TaskStart(task, hints.env, dagName), nil

	case coordinatorv1.Operation_OPERATION_RETRY:
		hints, err := e.subprocessHints(ctx, task, originalTarget)
		if err != nil {
			return runtime.CmdSpec{}, err
		}
		return e.subCmdBuilder.TaskRetry(task, hints.env, dagName), nil

	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return runtime.CmdSpec{}, fmt.Errorf("operation not specified")

	default:
		return runtime.CmdSpec{}, fmt.Errorf("unknown operation: %v", task.Operation)
	}
}

type subprocessHintSet struct {
	env []string
}

func (e *taskHandler) subprocessHints(ctx context.Context, task *coordinatorv1.Task, originalTarget string) (*subprocessHintSet, error) {
	dagName := dagNameHint(originalTarget)

	var loadOpts []spec.LoadOption
	if dagName != "" {
		loadOpts = append(loadOpts, spec.WithName(dagName))
	}
	if task.BaseConfig != "" {
		loadOpts = append(loadOpts, spec.WithBaseConfigContent([]byte(task.BaseConfig)))
	} else if e.baseConfig != "" {
		loadOpts = append(loadOpts, spec.WithBaseConfig(e.baseConfig))
	}

	dag, err := spec.Load(ctx, task.Target, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load DAG for subprocess hints: %w", err)
	}

	params, err := retryParams(task, dag)
	if err != nil {
		return nil, err
	}
	if task.Operation == coordinatorv1.Operation_OPERATION_START {
		params = task.Params
	}

	env, err := spec.ResolveEnv(ctx, dag, params, spec.ResolveEnvOptions{
		BaseConfig: e.baseConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve DAG env for subprocess: %w", err)
	}

	return &subprocessHintSet{
		env: env,
	}, nil
}

func dagNameHint(target string) string {
	name := strings.TrimSpace(target)
	if name == "" {
		return ""
	}
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	if ext == ".yaml" || ext == ".yml" {
		return strings.TrimSuffix(base, ext)
	}
	return base
}

func retryParams(task *coordinatorv1.Task, dag *core.DAG) (any, error) {
	if task.Operation != coordinatorv1.Operation_OPERATION_RETRY || task.PreviousStatus == nil {
		return nil, nil
	}

	status, err := convert.ProtoToDAGRunStatus(task.PreviousStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to decode previous task status: %w", err)
	}

	return spec.QuoteRuntimeParams(status.ParamsList, dag.ParamDefs), nil
}
