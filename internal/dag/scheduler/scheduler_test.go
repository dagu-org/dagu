// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var (
	testCommand     = "true"
	testCommandFail = "false"
	testHomeDir     string
)

func TestMain(m *testing.M) {
	testHomeDir = util.MustTempDir("scheduler-test")
	err := os.Setenv("HOME", testHomeDir)
	if err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.RemoveAll(testHomeDir)
	os.Exit(code)
}

func schedulerTextCtxWithDagContext() context.Context {
	return dag.NewContext(context.Background(), nil, nil, "", "")
}

func TestScheduler(t *testing.T) {
	g, err := NewExecutionGraph(
		logger.Default,
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommandFail, "2"),
		step("4", testCommand, "3"),
	)
	require.NoError(t, err)
	sc := New(&Config{
		MaxActiveRuns: 1, LogDir: testHomeDir, Logger: logger.Default,
	})

	var counter atomic.Int64
	done := make(chan *Node)
	go func() {
		for range done {
			counter.Add(1)
		}
	}()

	err = sc.Schedule(schedulerTextCtxWithDagContext(), g, done)
	require.Error(t, err)

	require.Equal(t, counter.Load(), int64(3))
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[2].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[3].State().Status)
}

func TestSchedulerParallel(t *testing.T) {
	g, sc := newTestScheduler(t,
		&Config{MaxActiveRuns: 1000, LogDir: testHomeDir},
		step("1", testCommand),
		step("2", testCommand),
		step("3", testCommand),
	)
	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), StatusSuccess)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)
}

func TestSchedulerFailPartially(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		step("2", testCommandFail),
		step("3", testCommand, "1"),
		step("4", testCommand, "3"),
	)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusError, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[3].State().Status)
}

func TestSchedulerContinueOnFailure(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		dag.Step{
			Name:    "2",
			Command: testCommandFail,
			Depends: []string{"1"},
			ContinueOn: dag.ContinueOn{
				Failure: true,
			},
		},
		step("3", testCommand, "2"),
	)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusError, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)
}

func TestSchedulerAllowSkipped(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		dag.Step{
			Name:    "2",
			Command: testCommand,
			Depends: []string{"1"},
			Preconditions: []dag.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "0",
				},
			},
			ContinueOn: dag.ContinueOn{Skipped: true},
		},
		step("3", testCommand, "2"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), StatusSuccess)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSkipped, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)
}

func TestSchedulerCancel(t *testing.T) {

	g, _ := NewExecutionGraph(
		logger.Default,
		step("1", testCommand),
		step("2", "sleep 1000", "1"),
		step("3", testCommandFail, "2"),
	)
	sc := New(&Config{MaxActiveRuns: 1, LogDir: testHomeDir})

	go func() {
		time.Sleep(time.Millisecond * 300)
		sc.Cancel(g)
	}()

	_ = sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)

	require.Eventually(t, func() bool {
		return sc.Status(g) == StatusCancel
	}, time.Second, time.Millisecond*10)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, NodeStatusNone, nodes[2].State().Status)
}

func TestSchedulerTimeout(t *testing.T) {
	g, _ := NewExecutionGraph(
		logger.Default,
		step("1", "sleep 1"),
		step("2", "sleep 1"),
		step("3", "sleep 3"),
		step("4", "sleep 10"),
		step("5", "sleep 1", "2"),
		step("6", "sleep 1", "5"),
	)
	sc := New(&Config{Timeout: time.Second * 2, LogDir: testHomeDir})

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[2].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[3].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[4].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[5].State().Status)
}

func TestSchedulerRetryFail(t *testing.T) {
	cmd := filepath.Join(util.MustGetwd(), "testdata/testfile.sh")
	g, sc, err := testSchedule(t,
		dag.Step{
			Name:        "1",
			Command:     cmd,
			ContinueOn:  dag.ContinueOn{Failure: true},
			RetryPolicy: &dag.RetryPolicy{Limit: 1},
		},
		dag.Step{
			Name:        "2",
			Command:     cmd,
			Args:        []string{"flag"},
			ContinueOn:  dag.ContinueOn{Failure: true},
			RetryPolicy: &dag.RetryPolicy{Limit: 1},
			Depends:     []string{"1"},
		},
		dag.Step{
			Name:    "3",
			Command: cmd,
			Depends: []string{"2"},
		},
		step("4", cmd, "3"),
	)
	require.True(t, err != nil)
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, NodeStatusError, nodes[1].State().Status)
	require.Equal(t, NodeStatusError, nodes[2].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[3].State().Status)

	require.Equal(t, nodes[0].State().RetryCount, 1)
	require.Equal(t, nodes[1].State().RetryCount, 1)
}

func TestSchedulerRetrySuccess(t *testing.T) {
	cmd := filepath.Join(util.MustGetwd(), "testdata/testfile.sh")
	tmpDir, err := os.MkdirTemp("", "scheduler_test")
	tmpFile := filepath.Join(tmpDir, "flag")

	require.NoError(t, err)
	defer os.Remove(tmpDir)

	g, sc := newTestScheduler(
		t, &Config{MaxActiveRuns: 2, LogDir: tmpDir},
		step("1", testCommand),
		dag.Step{
			Name:    "2",
			Command: cmd,
			Args:    []string{tmpFile},
			Depends: []string{"1"},
			RetryPolicy: &dag.RetryPolicy{
				Limit:    10,
				Interval: time.Millisecond * 800,
			},
		},
		step("3", testCommand, "2"),
	)

	go func() {
		// create file for successful retry
		timer1 := time.NewTimer(time.Millisecond * 300)
		defer timer1.Stop()
		<-timer1.C

		f, err := os.Create(tmpFile)
		require.NoError(t, err)
		_ = f.Close()
	}()

	go func() {
		timer2 := time.NewTimer(time.Millisecond * 500)
		defer timer2.Stop()
		<-timer2.C

		nodes := g.Nodes()

		// scheduled for retry
		require.Equal(t, 1, nodes[1].State().RetryCount)
		require.Equal(t, NodeStatusRunning, nodes[1].State().Status)
		startedAt := nodes[1].State().StartedAt

		// wait for retry
		timer3 := time.NewTimer(time.Millisecond * 500)
		defer timer3.Stop()
		<-timer3.C

		// check time difference
		retriedAt := nodes[1].State().RetriedAt
		require.Greater(t, retriedAt.Sub(startedAt), time.Millisecond*500)
	}()

	err = sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)

	require.NoError(t, err)
	require.Equal(t, sc.Status(g), StatusSuccess)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)

	if nodes[1].State().RetryCount == 0 {
		t.Error("step 2 Should be retried")
	}
}

func TestStepPreCondition(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		dag.Step{
			Name:    "2",
			Command: testCommand,
			Depends: []string{"1"},
			Preconditions: []dag.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "0",
				},
			},
		},
		step("3", testCommand, "2"),
		dag.Step{
			Name:    "4",
			Command: testCommand,
			Preconditions: []dag.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
			},
		},
		step("5", testCommand, "4"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), StatusSuccess)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSkipped, nodes[1].State().Status)
	require.Equal(t, NodeStatusSkipped, nodes[2].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[3].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[4].State().Status)
}

func TestSchedulerOnExit(t *testing.T) {
	onExitStep := step("onExit", testCommand)
	g, sc := newTestScheduler(t,
		&Config{OnExit: &onExitStep, LogDir: testHomeDir},
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)

	onExitNode := sc.HandlerNode(dag.HandlerOnExit)
	require.NotNil(t, onExitNode)
	require.Equal(t, NodeStatusSuccess, onExitNode.State().Status)
}

func TestSchedulerOnExitOnFail(t *testing.T) {
	onExitStep := step("onExit", testCommand)
	g, sc := newTestScheduler(t,
		&Config{OnExit: &onExitStep, LogDir: testHomeDir},
		step("1", testCommandFail),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)

	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(dag.HandlerOnExit).State().Status)
}

func TestSchedulerOnSignal(t *testing.T) {
	g, _ := NewExecutionGraph(logger.Default, dag.Step{
		Name:    "1",
		Command: "sleep",
		Args:    []string{"10"},
	})
	sc := New(&Config{LogDir: testHomeDir})

	go func() {
		timer := time.NewTimer(time.Millisecond * 50)
		defer timer.Stop()
		<-timer.C

		sc.Signal(g, syscall.SIGTERM, nil, false)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), StatusCancel)
	require.Equal(t, NodeStatusCancel, nodes[0].State().Status)
}

func TestSchedulerOnCancel(t *testing.T) {
	onSuccessStep := step("onSuccess", testCommand)
	onFailureStep := step("onFailure", testCommand)
	onCancelStep := step("onCancel", testCommand)
	g, sc := newTestScheduler(t,
		&Config{
			OnSuccess: &onSuccessStep,
			OnFailure: &onFailureStep,
			OnCancel:  &onCancelStep,
			LogDir:    testHomeDir,
		},
		step("1", testCommand),
		step("2", "sleep 60", "1"),
	)

	done := make(chan bool)
	go func() {
		timer := time.NewTimer(time.Millisecond * 500)
		defer timer.Stop()
		<-timer.C
		sc.Signal(g, syscall.SIGTERM, done, false)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)
	<-done // Wait for canceling finished
	require.Equal(t, sc.Status(g), StatusCancel)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(dag.HandlerOnSuccess).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(dag.HandlerOnFailure).State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(dag.HandlerOnCancel).State().Status)
}

func TestSchedulerOnSuccess(t *testing.T) {
	onExit := step("onExit", testCommand)
	onSuccess := step("onSuccess", testCommand)
	onFailure := step("onFailure", testCommand)
	g, sc := newTestScheduler(t,
		&Config{
			OnExit:    &onExit,
			OnSuccess: &onSuccess,
			OnFailure: &onFailure,
			LogDir:    testHomeDir,
		},
		step("1", testCommand),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(dag.HandlerOnExit).State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(dag.HandlerOnSuccess).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(dag.HandlerOnFailure).State().Status)
}

func TestSchedulerOnFailure(t *testing.T) {
	onExit := step("onExit", testCommand)
	onSuccess := step("onSuccess", testCommand)
	onFailure := step("onFailure", testCommand)
	onCancel := step("onCancel", testCommand)
	g, sc := newTestScheduler(t,
		&Config{
			OnExit:    &onExit,
			OnSuccess: &onSuccess,
			OnFailure: &onFailure,
			OnCancel:  &onCancel,
			LogDir:    testHomeDir,
		},
		step("1", testCommandFail),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(dag.HandlerOnExit).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(dag.HandlerOnSuccess).State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(dag.HandlerOnFailure).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(dag.HandlerOnCancel).State().Status)
}

func TestRepeat(t *testing.T) {
	g, _ := NewExecutionGraph(
		logger.Default,
		dag.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			RepeatPolicy: dag.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := New(&Config{LogDir: testHomeDir})

	go func() {
		timer := time.NewTimer(time.Millisecond * 3000)
		defer timer.Stop()
		<-timer.C
		sc.Cancel(g)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), StatusCancel)
	require.Equal(t, NodeStatusCancel, nodes[0].State().Status)
	require.Equal(t, nodes[0].data.State.DoneCount, 2)
}

func TestRepeatFail(t *testing.T) {
	g, _ := NewExecutionGraph(
		logger.Default,
		dag.Step{
			Name:    "1",
			Command: testCommandFail,
			RepeatPolicy: dag.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := New(&Config{LogDir: testHomeDir})
	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, sc.Status(g), StatusError)
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, nodes[0].data.State.DoneCount, 1)
}

func TestStopRepetitiveTaskGracefully(t *testing.T) {
	g, _ := NewExecutionGraph(
		logger.Default,
		dag.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			RepeatPolicy: dag.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := New(&Config{LogDir: testHomeDir})

	done := make(chan bool)
	go func() {
		timer := time.NewTimer(time.Millisecond * 100)
		defer timer.Stop()
		<-timer.C
		sc.Signal(g, syscall.SIGTERM, done, false)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)
	<-done

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), StatusSuccess)
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, nodes[0].data.State.DoneCount, 1)
}

func TestSchedulerStatusText(t *testing.T) {
	for k, v := range map[Status]string{
		StatusNone:    "not started",
		StatusRunning: "running",
		StatusError:   "failed",
		StatusCancel:  "canceled",
		StatusSuccess: "finished",
	} {
		require.Equal(t, k.String(), v)
	}

	for k, v := range map[NodeStatus]string{
		NodeStatusNone:    "not started",
		NodeStatusRunning: "running",
		NodeStatusError:   "failed",
		NodeStatusCancel:  "canceled",
		NodeStatusSuccess: "finished",
		NodeStatusSkipped: "skipped",
	} {
		require.Equal(t, k.String(), v)
	}
}

func TestNodeSetupFailure(t *testing.T) {
	g, _ := NewExecutionGraph(
		logger.Default,
		dag.Step{
			Name:    "1",
			Command: "sh",
			Dir:     "~/",
			Script:  "echo 1",
		},
	)
	sc := New(&Config{LogDir: testHomeDir})
	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, nodes[0].data.State.DoneCount, 0)
}

func TestNodeTeardownFailure(t *testing.T) {
	g, _ := NewExecutionGraph(
		logger.Default,
		dag.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
		},
	)
	sc := New(&Config{LogDir: testHomeDir})

	nodes := g.Nodes()
	go func() {
		time.Sleep(time.Millisecond * 300)
		nodes[0].mu.Lock()
		_ = nodes[0].logFile.Close()
		nodes[0].mu.Unlock()
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	// file already closed
	require.Error(t, err)

	require.Equal(t, sc.Status(g), StatusError)
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Error(t, nodes[0].State().Error)
}

func TestTakeOutputFromPrevStep(t *testing.T) {
	s1 := step("1", "echo take-output")
	s1.Output = "PREV_OUT"

	s2 := step("2", "sh", "1")
	s2.Script = "echo $PREV_OUT"
	s2.Output = "TOOK_PREV_OUT"

	g, sc := newTestScheduler(t, &Config{LogDir: testHomeDir}, s1, s2)
	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)

	require.Equal(t, "take-output", os.ExpandEnv("$TOOK_PREV_OUT"))
}

func step(name, command string, depends ...string) dag.Step {
	cmd, args := util.SplitCommand(command)
	return dag.Step{
		Name:    name,
		Command: cmd,
		Args:    args,
		Depends: depends,
	}
}

func testSchedule(t *testing.T, steps ...dag.Step) (
	*ExecutionGraph, *Scheduler, error,
) {
	t.Helper()
	g, sc := newTestScheduler(t,
		&Config{
			MaxActiveRuns: 2,
			LogDir:        testHomeDir,
		}, steps...)
	return g, sc, sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
}

func newTestScheduler(t *testing.T, cfg *Config, steps ...dag.Step) (
	*ExecutionGraph, *Scheduler,
) {
	t.Helper()
	g, err := NewExecutionGraph(logger.Default, steps...)
	require.NoError(t, err)
	return g, New(cfg)
}
