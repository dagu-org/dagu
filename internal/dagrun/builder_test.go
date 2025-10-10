package dagrun_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

func TestNewSubCmdBuilder(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/path/to/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/path/to/config.yaml",
			BaseEnv:        config.NewBaseEnv([]string{"TEST_ENV=value"}),
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)
	require.NotNil(t, builder)
}

func TestStart(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)
	dag := &digraph.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicStart", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.StartOptions{}
		spec := builder.Start(dag, opts)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "start")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
		assert.Equal(t, "/path/to", spec.WorkingDir)
		assert.NotNil(t, spec.Env)
	})

	t.Run("StartWithParams", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.StartOptions{
			Params: "key=value",
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"key=value"`)
	})

	t.Run("StartWithQuiet", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.StartOptions{
			Quiet: true,
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "-q")
	})

	t.Run("StartWithNoQueue", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.StartOptions{
			NoQueue: true,
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "--no-queue")
	})

	t.Run("StartWithDAGRunID", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.StartOptions{
			DAGRunID: "test-run-id",
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "--run-id=test-run-id")
	})

	t.Run("StartWithAllOptions", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.StartOptions{
			Params:   "env=prod",
			Quiet:    true,
			NoQueue:  true,
			DAGRunID: "full-test-id",
		}
		spec := builder.Start(dag, opts)

		assert.Contains(t, spec.Args, "start")
		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"env=prod"`)
		assert.Contains(t, spec.Args, "-q")
		assert.Contains(t, spec.Args, "--no-queue")
		assert.Contains(t, spec.Args, "--run-id=full-test-id")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
	})

	t.Run("StartWithoutConfigFile", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
			Global: config.Global{
				ConfigFileUsed: "",
			},
		}
		builderNoFile := dagrun.NewSubCmdBuilder(cfgNoFile)
		opts := dagrun.StartOptions{}
		spec := builderNoFile.Start(dag, opts)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestEnqueue(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)
	dag := &digraph.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicEnqueue", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.EnqueueOptions{}
		spec := builder.Enqueue(dag, opts)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "enqueue")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
		assert.Equal(t, "/path/to", spec.WorkingDir)
		assert.Equal(t, os.Stdout, spec.Stdout)
		assert.Equal(t, os.Stderr, spec.Stderr)
	})

	t.Run("EnqueueWithParams", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.EnqueueOptions{
			Params: "key=value",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "-p")
		assert.Contains(t, spec.Args, `"key=value"`)
	})

	t.Run("EnqueueWithQuiet", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.EnqueueOptions{
			Quiet: true,
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "-q")
	})

	t.Run("EnqueueWithDAGRunID", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.EnqueueOptions{
			DAGRunID: "enqueue-run-id",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "--run-id=enqueue-run-id")
	})

	t.Run("EnqueueWithQueue", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.EnqueueOptions{
			Queue: "custom-queue",
		}
		spec := builder.Enqueue(dag, opts)

		assert.Contains(t, spec.Args, "--queue")
		assert.Contains(t, spec.Args, "custom-queue")
	})

	t.Run("EnqueueWithAllOptions", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.EnqueueOptions{
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
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)
	dag := &digraph.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicDequeue", func(t *testing.T) {
		t.Parallel()
		dagRun := digraph.NewDAGRunRef("test-dag", "run-123")
		spec := builder.Dequeue(dag, dagRun)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "dequeue")
		assert.Contains(t, spec.Args, "--dag-run=test-dag:run-123")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Equal(t, "/path/to", spec.WorkingDir)
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
		builderNoFile := dagrun.NewSubCmdBuilder(cfgNoFile)
		dagRun := digraph.NewDAGRunRef("test-dag", "run-456")
		spec := builderNoFile.Dequeue(dag, dagRun)

		assert.NotContains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "--dag-run=test-dag:run-456")
	})
}

func TestRestart(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)
	dag := &digraph.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicRestart", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.RestartOptions{}
		spec := builder.Restart(dag, opts)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "restart")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/dag.yaml")
		assert.Equal(t, "/path/to", spec.WorkingDir)
	})

	t.Run("RestartWithQuiet", func(t *testing.T) {
		t.Parallel()
		opts := dagrun.RestartOptions{
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
		builderNoFile := dagrun.NewSubCmdBuilder(cfgNoFile)
		opts := dagrun.RestartOptions{}
		spec := builderNoFile.Restart(dag, opts)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestRetry(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)
	dag := &digraph.DAG{
		Name:       "test-dag",
		Location:   "/path/to/dag.yaml",
		WorkingDir: "/path/to",
	}

	t.Run("BasicRetry", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "retry-run-id", "", false)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=retry-run-id")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "test-dag")
		assert.Equal(t, "/path/to", spec.WorkingDir)
	})

	t.Run("RetryWithStepName", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "retry-run-id", "step-1", false)

		assert.Contains(t, spec.Args, "--step=step-1")
	})

	t.Run("RetryWithDisableMaxActiveRuns", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "retry-run-id", "", true)

		assert.Contains(t, spec.Args, "--disable-max-active-runs")
	})

	t.Run("RetryWithAllOptions", func(t *testing.T) {
		t.Parallel()
		spec := builder.Retry(dag, "full-retry-id", "step-2", true)

		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=full-retry-id")
		assert.Contains(t, spec.Args, "--step=step-2")
		assert.Contains(t, spec.Args, "--disable-max-active-runs")
		assert.Contains(t, spec.Args, "test-dag")
	})

	t.Run("RetryWithoutConfig", func(t *testing.T) {
		t.Parallel()
		cfgNoFile := &config.Config{
			Paths: config.PathsConfig{
				Executable: "/usr/bin/dagu",
			},
		}
		builderNoFile := dagrun.NewSubCmdBuilder(cfgNoFile)
		spec := builderNoFile.Retry(dag, "retry-run-id", "", false)

		assert.NotContains(t, spec.Args, "--config")
	})
}

func TestTaskStart(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)

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
		assert.Contains(t, spec.Args, "--no-queue")
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
		assert.Contains(t, spec.Args, "--no-queue")
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
		builderNoFile := dagrun.NewSubCmdBuilder(cfgNoFile)
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
			Executable: "/usr/bin/dagu",
		},
		Global: config.Global{
			ConfigFileUsed: "/etc/dagu/config.yaml",
		},
	}

	builder := dagrun.NewSubCmdBuilder(cfg)

	t.Run("BasicTaskRetry", func(t *testing.T) {
		t.Parallel()
		task := &coordinatorv1.Task{
			DagRunId: "retry-run-id",
			Target:   "/path/to/task.yaml",
		}
		spec := builder.TaskRetry(task)

		assert.Equal(t, "/usr/bin/dagu", spec.Executable)
		assert.Contains(t, spec.Args, "retry")
		assert.Contains(t, spec.Args, "--run-id=retry-run-id")
		assert.Contains(t, spec.Args, "--config")
		assert.Contains(t, spec.Args, "/etc/dagu/config.yaml")
		assert.Contains(t, spec.Args, "/path/to/task.yaml")
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
		builderNoFile := dagrun.NewSubCmdBuilder(cfgNoFile)
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
		spec := dagrun.CmdSpec{
			Executable: "/usr/bin/test",
			Args:       []string{"arg1", "arg2"},
			WorkingDir: "/tmp",
			Env:        []string{"VAR=value"},
			Stdout:     os.Stdout,
			Stderr:     os.Stderr,
		}

		assert.Equal(t, "/usr/bin/test", spec.Executable)
		assert.Equal(t, []string{"arg1", "arg2"}, spec.Args)
		assert.Equal(t, "/tmp", spec.WorkingDir)
		assert.Equal(t, []string{"VAR=value"}, spec.Env)
		assert.Equal(t, os.Stdout, spec.Stdout)
		assert.Equal(t, os.Stderr, spec.Stderr)
	})
}
