package runtime_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
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

func TestStart(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
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
		assert.NotNil(t, spec.Env)
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
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)

	t.Run("BasicTaskStart", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId: "task-run-id",
			Target:   "/path/to/task.yaml",
		}
		spec := builder.TaskStart(task)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "start")
		assert.Contains(t, spec.Args, "--run-id=task-run-id")

		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/task.yaml")
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
		spec := builder.TaskStart(task)

		assert.Contains(t, spec.Args, "--root=root-dag:root-id")
		assert.Contains(t, spec.Args, "--parent=parent-dag:parent-id")
		assert.Contains(t, spec.Args, "--run-id=child-run-id")

	})

	t.Run("TaskStartWithParams", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId: "task-run-id",
			Target:   "/path/to/task.yaml",
			Params:   "env=production",
		}
		spec := builder.TaskStart(task)

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
		spec := builder.TaskStart(task)

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
		spec := builder.TaskStart(task)

		assert.Contains(t, spec.Args, "--parent=parent-dag:parent-id")
		// Should not contain root flags
		for _, arg := range spec.Args {
			assert.NotContains(t, arg, "--root=")
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
			DagRunId: "task-run-id",
			Target:   "/path/to/task.yaml",
		}
		spec := builderNoFile.TaskStart(task)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestTaskRetry(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable:     "/usr/bin/dagu",
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := runtime.NewSubCmdBuilder(cfg)

	t.Run("BasicTaskRetry", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId:       "retry-run-id",
			Target:         "/path/to/task.yaml",
			RootDagRunName: "root-dag",
		}
		spec := builder.TaskRetry(task)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=retry-run-id")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "root-dag")
	})

	t.Run("TaskRetryWithStep", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId: "retry-run-id",
			Target:   "/path/to/task.yaml",
			Step:     "failed-step",
		}
		spec := builder.TaskRetry(task)

		assert.Contains(t, spec.Args, "--step=failed-step")
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
			DagRunId: "retry-run-id",
			Target:   "/path/to/task.yaml",
		}
		spec := builderNoFile.TaskRetry(task)

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
