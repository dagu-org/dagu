// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type testHelper struct {
	test.Helper

	Scheduler *scheduler.Scheduler
	Config    *scheduler.Config
}

type schedulerOption func(*scheduler.Config)

func withMaxActiveRuns(n int) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.MaxActiveRuns = n
	}
}

func setup(t *testing.T, opts ...schedulerOption) testHelper {
	t.Helper()

	th := test.Setup(t)

	cfg := &scheduler.Config{
		LogDir: th.Config.Paths.LogDir,
		ReqID:  uuid.Must(uuid.NewRandom()).String(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	sc := scheduler.New(cfg)

	return testHelper{
		Helper:    test.Setup(t),
		Scheduler: sc,
		Config:    cfg,
	}
}

func (th testHelper) newGraph(t *testing.T, steps ...digraph.Step) graphHelper {
	t.Helper()

	graph, err := scheduler.NewExecutionGraph(steps...)
	require.NoError(t, err)

	return graphHelper{
		testHelper:     th,
		ExecutionGraph: graph,
	}
}

type graphHelper struct {
	testHelper
	*scheduler.ExecutionGraph
}

func (gh graphHelper) Schedule(t *testing.T, expectedStatus scheduler.Status) scheduleResult {
	ctx := digraph.NewContext(gh.Context, &digraph.DAG{}, nil, nil, gh.Config.ReqID, "logFile")

	var doneNodes []*scheduler.Node
	nodeCompletedChan := make(chan *scheduler.Node)

	go func() {
		for node := range nodeCompletedChan {
			doneNodes = append(doneNodes, node)
		}
	}()

	err := gh.Scheduler.Schedule(ctx, gh.ExecutionGraph, nodeCompletedChan)

	switch expectedStatus {
	case scheduler.StatusSuccess, scheduler.StatusCancel:
		require.NoError(t, err)

	case scheduler.StatusError:
		require.Error(t, err)

	case scheduler.StatusRunning, scheduler.StatusNone:
		t.Errorf("unexpected status %s", expectedStatus)

	}

	require.Equal(t, gh.Scheduler.Status(gh.ExecutionGraph), expectedStatus,
		"expected status %s, got %s", expectedStatus, gh.Scheduler.Status(gh.ExecutionGraph))

	return scheduleResult{
		graphHelper: gh,
		Done:        doneNodes,
	}
}

type scheduleResult struct {
	graphHelper
	Done []*scheduler.Node
}

func (sr scheduleResult) AssertDoneCount(t *testing.T, expected int) {
	t.Helper()

	require.Len(t, sr.Done, expected, "expected %d done nodes, got %d", expected, len(sr.Done))
}

func (sr scheduleResult) AssertNodeStatus(t *testing.T, stepName string, expected scheduler.NodeStatus) {
	t.Helper()

	nodes := sr.ExecutionGraph.Nodes()
	var found bool
	for _, node := range nodes {
		if node.Data().Step.Name == stepName {
			found = true
			require.Equal(t, expected, node.State().Status, "expected status %s, got %s", expected, node.State().Status)
		}
	}

	require.True(t, found, "step %s not found", stepName)
}

var (
	testCommand     = "true"
	testCommandFail = "false"
	testHomeDir     string
)

func TestMain(m *testing.M) {
	testHomeDir = fileutil.MustTempDir("scheduler-test")
	err := os.Setenv("HOME", testHomeDir)
	if err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.RemoveAll(testHomeDir)
	os.Exit(code)
}

func schedulerTextCtxWithDagContext() context.Context {
	return digraph.NewContext(context.Background(), nil, nil, nil, "", "")
}

func TestScheduler(t *testing.T) {
	sc := setup(t, withMaxActiveRuns(1))

	// 1 -> 2 -> 3 -> 4
	graph := sc.newGraph(t,
		successStep("1"),
		successStep("2", "1"),
		failStep("3", "2"),
		successStep("4", "3"),
	)

	result := graph.Schedule(t, scheduler.StatusError)

	// 1, 2, 3 should be executed and 4 should be canceled because 3 failed
	result.AssertDoneCount(t, 3)
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "3", scheduler.NodeStatusError)
	result.AssertNodeStatus(t, "4", scheduler.NodeStatusCancel)
}

func TestSchedulerParallel(t *testing.T) {
	graph, sc := newTestScheduler(t,
		&scheduler.Config{MaxActiveRuns: 1000, LogDir: testHomeDir},
		step("1", testCommand),
		step("2", testCommand),
		step("3", testCommand),
	)
	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusSuccess)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)
}

func TestSchedulerFailPartially(t *testing.T) {
	graph, sc, err := testSchedule(t,
		step("1", testCommand),
		step("2", testCommandFail),
		step("3", testCommand, "1"),
		step("4", testCommand, "3"),
	)
	require.Error(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusError)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusError, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[3].State().Status)
}

func TestSchedulerContinueOnFailure(t *testing.T) {
	graph, sc, err := testSchedule(t,
		step("1", testCommand),
		digraph.Step{
			Name:    "2",
			Command: testCommandFail,
			Depends: []string{"1"},
			ContinueOn: digraph.ContinueOn{
				Failure: true,
			},
		},
		step("3", testCommand, "2"),
	)
	require.Error(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusError)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusError, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)
}

func TestSchedulerAllowSkipped(t *testing.T) {
	graph, sc, err := testSchedule(t,
		step("1", testCommand),
		digraph.Step{
			Name:    "2",
			Command: testCommand,
			Depends: []string{"1"},
			Preconditions: []digraph.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "0",
				},
			},
			ContinueOn: digraph.ContinueOn{Skipped: true},
		},
		step("3", testCommand, "2"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusSuccess)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSkipped, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)
}

func TestSchedulerCancel(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		step("1", testCommand),
		step("2", "sleep 100", "1"),
		step("3", testCommandFail, "2"),
	)
	cfg := &scheduler.Config{MaxActiveRuns: 1, LogDir: testHomeDir}
	sc := scheduler.New(cfg)

	go func() {
		time.Sleep(time.Millisecond * 300)
		sc.Cancel(graph)
	}()

	_ = sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)

	require.Eventually(t, func() bool {
		return sc.Status(graph) == scheduler.StatusCancel
	}, time.Second, time.Millisecond*10)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, nodes[2].State().Status)
}

func TestSchedulerTimeout(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		step("1", "sleep 1"),
		step("2", "sleep 1"),
		step("3", "sleep 3"),
		step("4", "sleep 10"),
		step("5", "sleep 1", "2"),
		step("6", "sleep 1", "5"),
	)
	cfg := &scheduler.Config{Timeout: time.Second * 2, LogDir: testHomeDir}
	sc := scheduler.New(cfg)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.Error(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusError)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[2].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[3].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[4].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[5].State().Status)
}

func TestSchedulerRetryFail(t *testing.T) {
	cmd := filepath.Join(fileutil.MustGetwd(), "testdata/testfile.sh")
	graph, sc, err := testSchedule(t,
		digraph.Step{
			Name:        "1",
			Command:     cmd,
			ContinueOn:  digraph.ContinueOn{Failure: true},
			RetryPolicy: digraph.RetryPolicy{Limit: 1},
		},
		digraph.Step{
			Name:        "2",
			Command:     cmd,
			Args:        []string{"flag"},
			ContinueOn:  digraph.ContinueOn{Failure: true},
			RetryPolicy: digraph.RetryPolicy{Limit: 1},
			Depends:     []string{"1"},
		},
		digraph.Step{
			Name:    "3",
			Command: cmd,
			Depends: []string{"2"},
		},
		step("4", cmd, "3"),
	)
	require.True(t, err != nil)
	require.Equal(t, sc.Status(graph), scheduler.StatusError)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusError, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusError, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusError, nodes[2].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[3].State().Status)

	require.Equal(t, nodes[0].State().RetryCount, 1)
	require.Equal(t, nodes[1].State().RetryCount, 1)
}

func TestSchedulerRetrySuccess(t *testing.T) {
	cmd := filepath.Join(fileutil.MustGetwd(), "testdata/testfile.sh")
	tmpDir, err := os.MkdirTemp("", "scheduler_test")
	tmpFile := filepath.Join(tmpDir, "flag")

	require.NoError(t, err)
	defer os.Remove(tmpDir)

	graph, sc := newTestScheduler(
		t, &scheduler.Config{MaxActiveRuns: 2, LogDir: tmpDir},
		step("1", testCommand),
		digraph.Step{
			Name:    "2",
			Command: cmd,
			Args:    []string{tmpFile},
			Depends: []string{"1"},
			RetryPolicy: digraph.RetryPolicy{
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

		nodes := graph.Nodes()

		// scheduled for retry
		require.Equal(t, 1, nodes[1].State().RetryCount)
		require.Equal(t, scheduler.NodeStatusRunning, nodes[1].State().Status)
		startedAt := nodes[1].State().StartedAt

		// wait for retry
		timer3 := time.NewTimer(time.Millisecond * 500)
		defer timer3.Stop()
		<-timer3.C

		// check time difference
		retriedAt := nodes[1].State().RetriedAt
		require.Greater(t, retriedAt.Sub(startedAt), time.Millisecond*500)
	}()

	err = sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)

	require.NoError(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusSuccess)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)

	if nodes[1].State().RetryCount == 0 {
		t.Error("step 2 Should be retried")
	}
}

func TestStepPreCondition(t *testing.T) {
	graph, sc, err := testSchedule(t,
		step("1", testCommand),
		digraph.Step{
			Name:    "2",
			Command: testCommand,
			Depends: []string{"1"},
			Preconditions: []digraph.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "0",
				},
			},
		},
		step("3", testCommand, "2"),
		digraph.Step{
			Name:    "4",
			Command: testCommand,
			Preconditions: []digraph.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
			},
		},
		step("5", testCommand, "4"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusSuccess)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSkipped, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSkipped, nodes[2].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[3].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[4].State().Status)
}

func TestSchedulerOnExit(t *testing.T) {
	onExitStep := step("onExit", testCommand)
	g, sc := newTestScheduler(t,
		&scheduler.Config{OnExit: &onExitStep, LogDir: testHomeDir},
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)

	onExitNode := sc.HandlerNode(digraph.HandlerOnExit)
	require.NotNil(t, onExitNode)
	require.Equal(t, scheduler.NodeStatusSuccess, onExitNode.State().Status)
}

func TestSchedulerOnExitOnFail(t *testing.T) {
	onExitStep := step("onExit", testCommand)
	graph, sc := newTestScheduler(t,
		&scheduler.Config{OnExit: &onExitStep, LogDir: testHomeDir},
		step("1", testCommandFail),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.Error(t, err)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusError, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[2].State().Status)

	require.Equal(t, scheduler.NodeStatusSuccess, sc.HandlerNode(digraph.HandlerOnExit).State().Status)
}

func TestSchedulerOnSignal(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(digraph.Step{
		Name:    "1",
		Command: "sleep",
		Args:    []string{"10"},
	})
	cfg := &scheduler.Config{LogDir: testHomeDir}
	sc := scheduler.New(cfg)

	go func() {
		timer := time.NewTimer(time.Millisecond * 50)
		defer timer.Stop()
		<-timer.C

		sc.Signal(context.Background(), graph, syscall.SIGTERM, nil, false)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)

	nodes := graph.Nodes()

	require.Equal(t, sc.Status(graph), scheduler.StatusCancel)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[0].State().Status)
}

func TestSchedulerOnCancel(t *testing.T) {
	onSuccessStep := step("onSuccess", testCommand)
	onFailureStep := step("onFailure", testCommand)
	onCancelStep := step("onCancel", testCommand)
	graph, sc := newTestScheduler(t,
		&scheduler.Config{
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
		sc.Signal(context.Background(), graph, syscall.SIGTERM, done, false)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)
	<-done // Wait for canceling finished
	require.Equal(t, sc.Status(graph), scheduler.StatusCancel)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, sc.HandlerNode(digraph.HandlerOnSuccess).State().Status)
	require.Equal(t, scheduler.NodeStatusNone, sc.HandlerNode(digraph.HandlerOnFailure).State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, sc.HandlerNode(digraph.HandlerOnCancel).State().Status)
}

func TestSchedulerOnSuccess(t *testing.T) {
	onExit := step("onExit", testCommand)
	onSuccess := step("onSuccess", testCommand)
	onFailure := step("onFailure", testCommand)
	graph, sc := newTestScheduler(t,
		&scheduler.Config{
			OnExit:    &onExit,
			OnSuccess: &onSuccess,
			OnFailure: &onFailure,
			LogDir:    testHomeDir,
		},
		step("1", testCommand),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, sc.HandlerNode(digraph.HandlerOnExit).State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, sc.HandlerNode(digraph.HandlerOnSuccess).State().Status)
	require.Equal(t, scheduler.NodeStatusNone, sc.HandlerNode(digraph.HandlerOnFailure).State().Status)
}

func TestSchedulerOnFailure(t *testing.T) {
	onExit := step("onExit", testCommand)
	onSuccess := step("onSuccess", testCommand)
	onFailure := step("onFailure", testCommand)
	onCancel := step("onCancel", testCommand)
	graph, sc := newTestScheduler(t,
		&scheduler.Config{
			OnExit:    &onExit,
			OnSuccess: &onSuccess,
			OnFailure: &onFailure,
			OnCancel:  &onCancel,
			LogDir:    testHomeDir,
		},
		step("1", testCommandFail),
	)

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.Error(t, err)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusError, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, sc.HandlerNode(digraph.HandlerOnExit).State().Status)
	require.Equal(t, scheduler.NodeStatusNone, sc.HandlerNode(digraph.HandlerOnSuccess).State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, sc.HandlerNode(digraph.HandlerOnFailure).State().Status)
	require.Equal(t, scheduler.NodeStatusNone, sc.HandlerNode(digraph.HandlerOnCancel).State().Status)
}

func TestRepeat(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		digraph.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			RepeatPolicy: digraph.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	cfg := &scheduler.Config{LogDir: testHomeDir}
	sc := scheduler.New(cfg)

	go func() {
		timer := time.NewTimer(time.Millisecond * 3000)
		defer timer.Stop()
		<-timer.C
		sc.Cancel(graph)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)

	nodes := graph.Nodes()

	require.Equal(t, sc.Status(graph), scheduler.StatusCancel)
	require.Equal(t, scheduler.NodeStatusCancel, nodes[0].State().Status)
	require.Equal(t, nodes[0].Data().State.DoneCount, 2)
}

func TestRepeatFail(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		digraph.Step{
			Name:    "1",
			Command: testCommandFail,
			RepeatPolicy: digraph.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	cfg := &scheduler.Config{LogDir: testHomeDir}
	sc := scheduler.New(cfg)
	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.Error(t, err)

	nodes := graph.Nodes()
	require.Equal(t, sc.Status(graph), scheduler.StatusError)
	require.Equal(t, scheduler.NodeStatusError, nodes[0].State().Status)
	require.Equal(t, nodes[0].Data().State.DoneCount, 1)
}

func TestStopRepetitiveTaskGracefully(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		digraph.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			RepeatPolicy: digraph.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	cfg := &scheduler.Config{LogDir: testHomeDir}
	sc := scheduler.New(cfg)

	done := make(chan bool)
	go func() {
		timer := time.NewTimer(time.Millisecond * 100)
		defer timer.Stop()
		<-timer.C
		sc.Signal(context.Background(), graph, syscall.SIGTERM, done, false)
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)
	<-done

	nodes := graph.Nodes()

	require.Equal(t, sc.Status(graph), scheduler.StatusSuccess)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, nodes[0].Data().State.DoneCount, 1)
}

func TestSchedulerStatusText(t *testing.T) {
	for k, v := range map[scheduler.Status]string{
		scheduler.StatusNone:    "not started",
		scheduler.StatusRunning: "running",
		scheduler.StatusError:   "failed",
		scheduler.StatusCancel:  "canceled",
		scheduler.StatusSuccess: "finished",
	} {
		require.Equal(t, k.String(), v)
	}

	for k, v := range map[scheduler.NodeStatus]string{
		scheduler.NodeStatusNone:    "not started",
		scheduler.NodeStatusRunning: "running",
		scheduler.NodeStatusError:   "failed",
		scheduler.NodeStatusCancel:  "canceled",
		scheduler.NodeStatusSuccess: "finished",
		scheduler.NodeStatusSkipped: "skipped",
	} {
		require.Equal(t, k.String(), v)
	}
}

func TestNodeSetupFailure(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		digraph.Step{
			Name:    "1",
			Command: "sh",
			Dir:     "~/",
			Script:  "echo 1",
		},
	)
	cfg := &scheduler.Config{LogDir: testHomeDir}
	sc := scheduler.New(cfg)
	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.Error(t, err)
	require.Equal(t, sc.Status(graph), scheduler.StatusError)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusError, nodes[0].State().Status)
	require.Equal(t, nodes[0].Data().State.DoneCount, 0)
}

func TestNodeTeardownFailure(t *testing.T) {
	graph, _ := scheduler.NewExecutionGraph(
		digraph.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
		},
	)
	cfg := &scheduler.Config{LogDir: testHomeDir}
	sc := scheduler.New(cfg)

	nodes := graph.Nodes()
	go func() {
		time.Sleep(time.Millisecond * 300)
		_ = nodes[0].CloseLog()
	}()

	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	// file already closed
	require.Error(t, err)

	require.Equal(t, sc.Status(graph), scheduler.StatusError)
	require.Equal(t, scheduler.NodeStatusError, nodes[0].State().Status)
	require.Error(t, nodes[0].State().Error)
}

func TestTakeOutputFromPrevStep(t *testing.T) {
	s1 := step("1", "echo take-output")
	s1.Output = "PREV_OUT"

	s2 := step("2", "sh", "1")
	s2.Script = "echo $PREV_OUT"
	s2.Output = "TOOK_PREV_OUT"

	graph, sc := newTestScheduler(t, &scheduler.Config{LogDir: testHomeDir}, s1, s2)
	err := sc.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
	require.NoError(t, err)

	nodes := graph.Nodes()
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[1].State().Status)

	require.Equal(t, "take-output", os.ExpandEnv("$TOOK_PREV_OUT"))
}

func successStep(name string, depends ...string) digraph.Step {
	return step(name, testCommand, depends...)
}

func failStep(name string, depends ...string) digraph.Step {
	return step(name, testCommandFail, depends...)
}

func step(name, command string, depends ...string) digraph.Step {
	splits := strings.SplitN(command, " ", 2)
	cmd := splits[0]
	var args []string
	if len(splits) == 2 {
		args = strings.Fields(splits[1])
	}
	return digraph.Step{
		Name:    name,
		Command: cmd,
		Args:    args,
		Depends: depends,
	}
}

func testSchedule(t *testing.T, steps ...digraph.Step) (
	*scheduler.ExecutionGraph, *scheduler.Scheduler, error,
) {
	t.Helper()
	graph, scheduler := newTestScheduler(t,
		&scheduler.Config{
			MaxActiveRuns: 2,
			LogDir:        testHomeDir,
		}, steps...)
	return graph, scheduler, scheduler.Schedule(schedulerTextCtxWithDagContext(), graph, nil)
}

func newTestScheduler(t *testing.T, cfg *scheduler.Config, steps ...digraph.Step) (
	*scheduler.ExecutionGraph, *scheduler.Scheduler,
) {
	t.Helper()
	graph, err := scheduler.NewExecutionGraph(steps...)
	require.NoError(t, err)
	return graph, scheduler.New(cfg)
}
