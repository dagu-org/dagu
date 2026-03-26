// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

func TestNewSubCmdBuilder(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/path/to/dagu",
			ConfigFileUsed: "/path/to/config.yaml",
		},
		Core: config.Core{
			BaseEnv: config.NewBaseEnv([]string{"TEST_ENV=value"}),
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	require.NotNil(t, builder)
}

func TestSubCmdBuilderEnvIncludesDockerVars(t *testing.T) {
	t.Parallel()

	baseEnv := []string{
		"PATH=/usr/bin",
		"DOCKER_HOST=tcp://remote:2375",
		"DOCKER_TLS_VERIFY=1",
		"DOCKER_CERT_PATH=/certs",
		"DOCKER_API_VERSION=1.41",
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/path/to/dagu",
			ConfigFileUsed: "/path/to/config.yaml",
		},
		Core: config.Core{
			BaseEnv: config.NewBaseEnv(baseEnv),
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	dag := &core.DAG{Location: "/tmp/test.yaml"}
	spec := builder.Start(dag, runtime.StartOptions{})

	envMap := make(map[string]string)
	for _, e := range spec.Env {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}

	assert.Equal(t, "tcp://remote:2375", envMap["DOCKER_HOST"])
	assert.Equal(t, "1", envMap["DOCKER_TLS_VERIFY"])
	assert.Equal(t, "/certs", envMap["DOCKER_CERT_PATH"])
	assert.Equal(t, "1.41", envMap["DOCKER_API_VERSION"])

	// DOCKER_AUTH_CONFIG should not be present (it was never in baseEnv).
	_, found := envMap["DOCKER_AUTH_CONFIG"]
	assert.False(t, found, "DOCKER_AUTH_CONFIG must not appear in subprocess env")
}

func TestRunRetryWithBuiltExecutable(t *testing.T) {
	th := test.Setup(t, test.WithBuiltExecutable())

	dagFile := th.DAG(t, `name: built-exec-retry
steps:
  - name: step1
    command: echo built exec retry
`)

	runID := "built-exec-retry-run"
	attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dagFile.DAG, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	logPath := filepath.Join(th.Config.Paths.LogDir, "built-exec-retry.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o750))

	status := transform.NewStatusBuilder(dagFile.DAG).Create(
		runID,
		core.Queued,
		0,
		time.Time{},
		transform.WithAttemptID(attempt.ID()),
		transform.WithTriggerType(core.TriggerTypeRetry),
		transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
		transform.WithLogFilePath(logPath),
	)

	require.NoError(t, attempt.Open(th.Context))
	require.NoError(t, attempt.Write(th.Context, status))
	require.NoError(t, attempt.Close(th.Context))

	spec := th.SubCmdBuilder.Retry(dagFile.DAG, runID, "")
	err = runtime.Run(th.Context, spec)
	require.NoError(t, err, "env=%s", strings.Join(spec.Env, "\n"))
}

func TestRunRetryWithBuiltExecutableFromQueuedQueueStatus(t *testing.T) {
	th := test.Setup(t, test.WithBuiltExecutable())

	dagFile := th.DAG(t, `name: built-exec-queue-retry
steps:
  - name: step1
    command: echo built exec queue retry
`)

	runID := "built-exec-queue-retry-run"
	attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dagFile.DAG, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	logPath := filepath.Join(th.Config.Paths.LogDir, dagFile.Name, runID+".log")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o750))

	status := transform.NewStatusBuilder(dagFile.DAG).Create(
		runID,
		core.Queued,
		0,
		time.Time{},
		transform.WithLogFilePath(logPath),
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(exec.NewDAGRunRef(dagFile.Name, runID), exec.DAGRunRef{}),
	)

	require.NoError(t, attempt.Open(th.Context))
	require.NoError(t, attempt.Write(th.Context, status))
	require.NoError(t, attempt.Close(th.Context))

	spec := th.SubCmdBuilder.Retry(dagFile.DAG, runID, "")
	err = runtime.Run(th.Context, spec)
	require.NoError(t, err, "env=%s", strings.Join(spec.Env, "\n"))
}

func TestRunRetryWithBuiltExecutableFromQueuedQueueStatusUsingSetupCommand(t *testing.T) {
	th := test.SetupCommand(t, test.WithBuiltExecutable())

	dagFile := th.DAG(t, `name: built-exec-command-queue-retry
steps:
  - name: step1
    command: echo built exec command queue retry
`)

	runID := "built-exec-command-queue-retry-run"
	attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dagFile.DAG, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	logPath := filepath.Join(th.Config.Paths.LogDir, dagFile.Name, runID+".log")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o750))

	status := transform.NewStatusBuilder(dagFile.DAG).Create(
		runID,
		core.Queued,
		0,
		time.Time{},
		transform.WithLogFilePath(logPath),
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(exec.NewDAGRunRef(dagFile.Name, runID), exec.DAGRunRef{}),
	)

	require.NoError(t, attempt.Open(th.Context))
	require.NoError(t, attempt.Write(th.Context, status))
	require.NoError(t, attempt.Close(th.Context))

	spec := th.SubCmdBuilder.Retry(dagFile.DAG, runID, "")
	err = runtime.Run(th.Context, spec)
	require.NoError(t, err, "env=%s", strings.Join(spec.Env, "\n"))
}

func TestRunRetryWithBuiltExecutableFromFreshLoadedConfig(t *testing.T) {
	th := test.Setup(t, test.WithBuiltExecutable())

	dagFile := th.DAG(t, `name: built-exec-fresh-config-retry
steps:
  - name: step1
    command: echo built exec fresh config retry
`)

	runID := "built-exec-fresh-config-retry-run"
	attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dagFile.DAG, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	logPath := filepath.Join(th.Config.Paths.LogDir, dagFile.Name, runID+".log")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o750))

	status := transform.NewStatusBuilder(dagFile.DAG).Create(
		runID,
		core.Queued,
		0,
		time.Time{},
		transform.WithLogFilePath(logPath),
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(exec.NewDAGRunRef(dagFile.Name, runID), exec.DAGRunRef{}),
	)

	require.NoError(t, attempt.Open(th.Context))
	require.NoError(t, attempt.Write(th.Context, status))
	require.NoError(t, attempt.Close(th.Context))

	loader := config.NewConfigLoader(
		viper.New(),
		config.WithConfigFile(th.Config.Paths.ConfigFileUsed),
		config.WithAppHomeDir(filepath.Dir(th.Config.Paths.DAGsDir)),
	)
	freshCfg, err := loader.Load()
	require.NoError(t, err)

	spec := runtime.NewSubCmdBuilder(freshCfg).Retry(dagFile.DAG, runID, "")
	err = runtime.Run(th.Context, spec)
	require.NoError(t, err, "env=%s", strings.Join(spec.Env, "\n"))
}

func TestStart(t *testing.T) {
	t.Parallel()
	baseEnv := config.NewBaseEnv([]string{"PATH=/usr/bin", "HOME=/tmp/test-home"})
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
		Core: config.Core{
			BaseEnv: baseEnv,
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	dag := &core.DAG{
		Name:     "test-dag",
		Location: "/path/to/dag.yaml",
	}

	t.Run("BasicStart", func(t *testing.T) {
		t.Parallel()
		opts := runtime.StartOptions{}
		spec := builder.Start(dag, opts)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "start")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
		assert.Equal(t, baseEnv.AsSlice(), spec.Env)
	})

	t.Run("StartWithParams", func(t *testing.T) {
		t.Parallel()
		opts := runtime.StartOptions{
			Params: "key=value",
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"key=value"`)
	})

	t.Run("StartWithQuiet", func(t *testing.T) {
		t.Parallel()
		opts := runtime.StartOptions{
			Quiet: true,
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "-q")
	})

	t.Run("StartWithDAGRunID", func(t *testing.T) {
		t.Parallel()
		opts := runtime.StartOptions{
			DAGRunID: "test-run-id",
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "--run-id=test-run-id")
	})

	t.Run("StartWithAllOptions", func(t *testing.T) {
		t.Parallel()
		opts := runtime.StartOptions{
			Params:   "env=prod",
			Quiet:    true,
			DAGRunID: "full-test-id",
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "start")
		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"env=prod"`)
		assert.Contains(t, spec.Args, "-q")
		assert.Contains(t, spec.Args, "--run-id=full-test-id")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
	})

	t.Run("StartWithoutConfigFile", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable:     "/usr/bin/dagu",
				ConfigFileUsed: "",
			},
		}
		builderNoFile := runtime.NewSubCmdBuilder(cfgNoFile)
		opts := runtime.StartOptions{}
		spec := builderNoFile.Start(dag, opts)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestEnqueue(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	dag := &core.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicEnqueue", func(t *testing.T) {
		t.Parallel()
		opts := runtime.EnqueueOptions{}
		spec := builder.Enqueue(dag, opts)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "enqueue")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
		assert.Equal(t, os.Stdout, spec.Stdout)
		assert.Equal(t, os.Stderr, spec.Stderr)
	})

	t.Run("EnqueueWithParams", func(t *testing.T) {
		t.Parallel()
		opts := runtime.EnqueueOptions{
			Params: "key=value",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"key=value"`)
	})

	t.Run("EnqueueWithQuiet", func(t *testing.T) {
		t.Parallel()
		opts := runtime.EnqueueOptions{
			Quiet: true,
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "-q")
	})

	t.Run("EnqueueWithDAGRunID", func(t *testing.T) {
		t.Parallel()
		opts := runtime.EnqueueOptions{
			DAGRunID: "enqueue-run-id",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "--run-id=enqueue-run-id")
	})

	t.Run("EnqueueWithQueue", func(t *testing.T) {
		t.Parallel()
		opts := runtime.EnqueueOptions{
			Queue: "custom-queue",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "--queue")
		assert.Contains(t, spec.Args, "custom-queue")
	})

	t.Run("EnqueueWithAllOptions", func(t *testing.T) {
		t.Parallel()
		opts := runtime.EnqueueOptions{
			Params:   "env=staging",
			Quiet:    true,
			DAGRunID: "full-enqueue-id",
			Queue:    "priority-queue",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "enqueue")
		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"env=staging"`)
		assert.Contains(t, spec.Args, "-q")
		assert.Contains(t, spec.Args, "--run-id=full-enqueue-id")
		assert.Contains(t, spec.Args, "--queue")
		assert.Contains(t, spec.Args, "priority-queue")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
	})
}

func TestDequeue(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	dag := &core.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicDequeue", func(t *testing.T) {
		t.Parallel()
		dagRun := exec.NewDAGRunRef("test-dag", "run-123")
		spec := builder.Dequeue(dag, dagRun)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "dequeue")
		// Queue name should be the first argument after "dequeue"
		assert.Equal(t, "test-dag", spec.Args[1])
		assert.Contains(t, spec.Args, "--dag-run=test-dag:run-123")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Equal(t, os.Stdout, spec.Stdout)
		assert.Equal(t, os.Stderr, spec.Stderr)
	})

	t.Run("DequeueWithoutConfig", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
		}
		builderNoFile := runtime.NewSubCmdBuilder(cfgNoFile)
		dagRun := exec.NewDAGRunRef("test-dag", "run-456")
		spec := builderNoFile.Dequeue(dag, dagRun)

		assert.NotContains(t, spec.Args, "--config")
		// Queue name should be the first argument after "dequeue"
		assert.Equal(t, "test-dag", spec.Args[1])
		assert.Contains(t, spec.Args, "--dag-run=test-dag:run-456")
	})

	t.Run("DequeueWithCustomQueue", func(t *testing.T) {
		t.Parallel()
		dagWithQueue := &core.DAG{
			Name:       "test-dag",
			Queue:      "custom-queue",
			Location:   "/path/to/dag.yaml",
			WorkingDir: "/path/to",
		}
		dagRun := exec.NewDAGRunRef("test-dag", "run-789")
		spec := builder.Dequeue(dagWithQueue, dagRun)

		// Queue name should be the custom queue, not the DAG name
		assert.Equal(t, "custom-queue", spec.Args[1])
		assert.Contains(t, spec.Args, "--dag-run=test-dag:run-789")
	})
}

func TestRestart(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	dag := &core.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicRestart", func(t *testing.T) {
		t.Parallel()
		opts := runtime.RestartOptions{}
		spec := builder.Restart(dag, opts)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "restart")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
	})

	t.Run("RestartWithQuiet", func(t *testing.T) {
		t.Parallel()
		opts := runtime.RestartOptions{
			Quiet: true,
		}
		spec := builder.Restart(dag, opts)

		assert.Contains(t, spec.Args, "-q")
	})

	t.Run("RestartWithScheduleTime", func(t *testing.T) {
		t.Parallel()
		opts := runtime.RestartOptions{
			ScheduleTime: "2026-03-13T10:00:00Z",
		}
		spec := builder.Restart(dag, opts)

		assert.Contains(t, spec.Args, "--schedule-time=2026-03-13T10:00:00Z")
	})

	t.Run("RestartWithoutConfig", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
		}
		builderNoFile := runtime.NewSubCmdBuilder(cfgNoFile)
		opts := runtime.RestartOptions{}
		spec := builderNoFile.Restart(dag, opts)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestRetry(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)
	dag := &core.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicRetry", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "retry-run-id", "")

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=retry-run-id")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "test-dag")
	})

	t.Run("RetryWithStepName", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "retry-run-id", "step-1")

		assert.Contains(t, spec.Args, "--step=step-1")
	})

	t.Run("RetryWithAllOptions", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "full-retry-id", "step-2")

		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=full-retry-id")
		assert.Contains(t, spec.Args, "--step=step-2")
		assert.Contains(t, spec.Args, "test-dag")
	})

	t.Run("RetryWithoutConfig", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
		}
		builderNoFile := runtime.NewSubCmdBuilder(cfgNoFile)
		spec := builderNoFile.Retry(dag, "retry-run-id", "")

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestTaskStart(t *testing.T) {
	t.Parallel()
	baseEnv := config.NewBaseEnv([]string{"PATH=/usr/bin", "HOME=/tmp/test-home"})
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
		Core: config.Core{
			BaseEnv: baseEnv,
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)

	t.Run("BasicTaskStart", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:  "task-run-id",
			AttemptId: "attempt-1",
			Target:    "/path/to/task.yaml",
		}
		spec := builder.TaskStart(task, nil)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "start")
		assert.Contains(t, spec.Args, "--run-id=task-run-id")
		assert.Contains(t, spec.Args, "--attempt-id=attempt-1")

		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/task.yaml")
		assert.Equal(t, baseEnv.AsSlice(), spec.Env)
	})

	t.Run("TaskStartWithHierarchy", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:         "child-run-id",
			Target:           "/path/to/child.yaml",
			RootDagRunId:     "root-id",
			RootDagRunName:   "root-dag",
			ParentDagRunId:   "parent-id",
			ParentDagRunName: "parent-dag",
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Args, "--root=root-dag:root-id")
		assert.Contains(t, spec.Args, "--parent=parent-dag:parent-id")
		assert.Contains(t, spec.Args, "--run-id=child-run-id")

	})

	t.Run("TaskStartWithParams", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:  "task-run-id",
			AttemptId: "attempt-1",
			Target:    "/path/to/task.yaml",
			Params:    "env=production",
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Args, "--")
		assert.Contains(t, spec.Args, "env=production")
	})

	t.Run("TaskStartWithRootOnly", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:       "task-run-id",
			Target:         "/path/to/task.yaml",
			RootDagRunId:   "root-id",
			RootDagRunName: "root-dag",
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Args, "--root=root-dag:root-id")
		// Should not contain parent flags
		for _, arg := range spec.Args {
			assert.NotContains(t, arg, "--parent=")
		}
	})

	t.Run("TaskStartWithParentOnly", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:         "task-run-id",
			Target:           "/path/to/task.yaml",
			ParentDagRunId:   "parent-id",
			ParentDagRunName: "parent-dag",
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Args, "--parent=parent-dag:parent-id")
		// Should not contain root flags
		for _, arg := range spec.Args {
			assert.NotContains(t, arg, "--root=")
		}
	})

	t.Run("TaskStartWithTags", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:  "task-run-id",
			AttemptId: "attempt-1",
			Target:    "/path/to/task.yaml",
			Tags:      "env=prod,team=backend",
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Args, "--tags=env=prod,team=backend")
	})

	t.Run("TaskStartWithScheduleTime", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:     "task-run-id",
			AttemptId:    "attempt-1",
			Target:       "/path/to/task.yaml",
			ScheduleTime: "2026-03-13T10:00:00Z",
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Args, "--schedule-time=2026-03-13T10:00:00Z")
	})

	t.Run("TaskStartWithExternalStepRetry", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:          "task-run-id",
			AttemptId:         "attempt-1",
			Target:            "/path/to/task.yaml",
			ExternalStepRetry: true,
		}
		spec := builder.TaskStart(task, nil)

		assert.Contains(t, spec.Env, exec.EnvKeyExternalStepRetry+"=1")
	})

	t.Run("TaskStartWithoutTags", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:  "task-run-id",
			AttemptId: "attempt-1",
			Target:    "/path/to/task.yaml",
		}
		spec := builder.TaskStart(task, nil)

		for _, arg := range spec.Args {
			assert.NotContains(t, arg, "--tags=")
		}
	})

	t.Run("TaskStartWithoutConfig", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
		}
		builderNoFile := runtime.NewSubCmdBuilder(cfgNoFile)
		task := &coordinatorv1.Task{
			DagRunId:  "task-run-id",
			AttemptId: "attempt-1",
			Target:    "/path/to/task.yaml",
		}
		spec := builderNoFile.TaskStart(task, nil)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestTaskRetry(t *testing.T) {
	t.Parallel()
	baseEnv := config.NewBaseEnv([]string{"PATH=/usr/bin", "HOME=/tmp/test-home"})
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
		Core: config.Core{
			BaseEnv: baseEnv,
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)

	t.Run("BasicTaskRetry", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:       "retry-run-id",
			AttemptId:      "attempt-2",
			Target:         "/path/to/task.yaml",
			RootDagRunName: "root-dag",
		}
		spec := builder.TaskRetry(task, nil)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=retry-run-id")
		assert.Contains(t, spec.Args, "--attempt-id=attempt-2")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "root-dag")
		assert.Equal(t, baseEnv.AsSlice(), spec.Env)
	})

	t.Run("TaskRetryWithStep", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:  "retry-run-id",
			AttemptId: "attempt-2",
			Target:    "/path/to/task.yaml",
			Step:      "failed-step",
		}
		spec := builder.TaskRetry(task, nil)

		assert.Contains(t, spec.Args, "--step=failed-step")
	})

	t.Run("TaskRetryWithExternalStepRetry", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:          "retry-run-id",
			AttemptId:         "attempt-2",
			Target:            "/path/to/task.yaml",
			RootDagRunName:    "root-dag",
			ExternalStepRetry: true,
		}
		spec := builder.TaskRetry(task, nil)

		assert.Contains(t, spec.Env, "PATH=/usr/bin")
		assert.Contains(t, spec.Env, "HOME=/tmp/test-home")
		assert.Contains(t, spec.Env, exec.EnvKeyExternalStepRetry+"=1")
	})

	t.Run("TaskRetryWithoutConfig", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
		}
		builderNoFile := runtime.NewSubCmdBuilder(cfgNoFile)
		task := &coordinatorv1.Task{
			DagRunId:  "retry-run-id",
			AttemptId: "attempt-2",
			Target:    "/path/to/task.yaml",
		}
		spec := builderNoFile.TaskRetry(task, nil)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestCmdSpec(t *testing.T) {
	t.Parallel()
	t.Run("CmdSpecStructure", func(t *testing.T) {
		t.Parallel()
		spec := runtime.CmdSpec{
			Executable: "/usr/bin/test",
			Args:       []string{"arg1", "arg2"},
			Env:        []string{"VAR=value"},
			Stdout:     os.Stdout,
			Stderr:     os.Stderr,
		}

		assert.Equal(t, "/usr/bin/test", spec.Executable)
		assert.Equal(t, []string{"arg1", "arg2"}, spec.Args)
		assert.Equal(t, []string{"VAR=value"}, spec.Env)
		assert.Equal(t, os.Stdout, spec.Stdout)
		assert.Equal(t, os.Stderr, spec.Stderr)
	})
}
