package scheduler

import (
	"context"
	"os"
	"path"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var (
	testCommand     = "true"
	testCommandFail = "false"
	testHomeDir     string
)

func TestMain(m *testing.M) {
	testHomeDir = util.MustTempDir("scheduler-test")
	changeHomeDir(testHomeDir)
	code := m.Run()
	_ = os.RemoveAll(testHomeDir)
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	_ = os.Setenv("HOME", homeDir)
	_ = config.LoadConfig()
}

func TestScheduler(t *testing.T) {
	g, err := NewExecutionGraph(
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommandFail, "2"),
		step("4", testCommand, "3"),
	)
	require.NoError(t, err)
	sc := &Scheduler{Config: &Config{MaxActiveRuns: 1}}

	var counter atomic.Int64
	done := make(chan *Node)
	go func() {
		for range done {
			counter.Add(1)
		}
	}()
	require.Error(t, sc.Schedule(context.Background(), g, done))
	require.Equal(t, counter.Load(), int64(3))
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[2].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[3].State().Status)
}

func TestSchedulerParallel(t *testing.T) {
	g, sc := newTestSchedule(t,
		&Config{
			MaxActiveRuns: 1000,
		},
		step("1", testCommand),
		step("2", testCommand),
		step("3", testCommand),
	)
	err := sc.Schedule(context.Background(), g, nil)
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
			Preconditions: []*dag.Condition{
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
		step("1", testCommand),
		step("2", "sleep 1000", "1"),
		step("3", testCommandFail, "2"),
	)
	sc := &Scheduler{Config: &Config{MaxActiveRuns: 1}}

	go func() {
		time.Sleep(time.Millisecond * 300)
		sc.Cancel(g)
	}()

	_ = sc.Schedule(context.Background(), g, nil)

	require.Eventually(t, func() bool {
		return sc.Status(g) == StatusCancel
	}, time.Second, time.Millisecond*10)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, NodeStatusNone, nodes[2].State().Status)
}

func TestSchedulerRetryFail(t *testing.T) {
	cmd := path.Join(util.MustGetwd(), "testdata/testfile.sh")
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
	cmd := path.Join(util.MustGetwd(), "testdata/testfile.sh")
	tmpDir, err := os.MkdirTemp("", "scheduler_test")
	tmpFile := path.Join(tmpDir, "flag")

	require.NoError(t, err)
	defer os.Remove(tmpDir)

	g, sc := newTestSchedule(
		t, &Config{MaxActiveRuns: 2},
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

	err = sc.Schedule(context.Background(), g, nil)

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
			Preconditions: []*dag.Condition{
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
			Preconditions: []*dag.Condition{
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
	g, sc := newTestSchedule(t,
		&Config{OnExit: &onExitStep},
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)

	onExitNode := sc.HandlerNode(constants.OnExit)
	require.NotNil(t, onExitNode)
	require.Equal(t, NodeStatusSuccess, onExitNode.State().Status)
}

func TestSchedulerOnExitOnFail(t *testing.T) {
	onExitStep := step("onExit", testCommand)
	g, sc := newTestSchedule(t,
		&Config{
			OnExit: &onExitStep,
		},
		step("1", testCommandFail),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(context.Background(), g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[2].State().Status)

	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(constants.OnExit).State().Status)
}

func TestSchedulerOnSignal(t *testing.T) {
	g, _ := NewExecutionGraph(dag.Step{
		Name:    "1",
		Command: "sleep",
		Args:    []string{"10"},
	})
	sc := &Scheduler{Config: &Config{}}

	go func() {
		timer := time.NewTimer(time.Millisecond * 50)
		defer timer.Stop()
		<-timer.C

		sc.Signal(g, syscall.SIGTERM, nil, false)
	}()

	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), StatusCancel)
	require.Equal(t, NodeStatusCancel, nodes[0].State().Status)
}

func TestSchedulerOnCancel(t *testing.T) {
	onSuccessStep := step("onSuccess", testCommand)
	onFailureStep := step("onFailure", testCommand)
	onCancelStep := step("onCancel", testCommand)
	g, sc := newTestSchedule(t,
		&Config{
			OnSuccess: &onSuccessStep,
			OnFailure: &onFailureStep,
			OnCancel:  &onCancelStep,
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

	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)
	<-done // Wait for canceling finished
	require.Equal(t, sc.Status(g), StatusCancel)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusCancel, nodes[1].State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(constants.OnSuccess).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(constants.OnFailure).State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(constants.OnCancel).State().Status)
}

func TestSchedulerOnSuccess(t *testing.T) {
	onExit := step("onExit", testCommand)
	onSuccess := step("onSuccess", testCommand)
	onFailure := step("onFailure", testCommand)
	g, sc := newTestSchedule(t,
		&Config{
			OnExit:    &onExit,
			OnSuccess: &onSuccess,
			OnFailure: &onFailure,
		},
		step("1", testCommand),
	)

	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(constants.OnExit).State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(constants.OnSuccess).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(constants.OnFailure).State().Status)
}

func TestSchedulerOnFailure(t *testing.T) {
	onExit := step("onExit", testCommand)
	onSuccess := step("onSuccess", testCommand)
	onFailure := step("onFailure", testCommand)
	onCancel := step("onCancel", testCommand)
	g, sc := newTestSchedule(t,
		&Config{
			OnExit:    &onExit,
			OnSuccess: &onSuccess,
			OnFailure: &onFailure,
			OnCancel:  &onCancel,
		},
		step("1", testCommandFail),
	)

	err := sc.Schedule(context.Background(), g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(constants.OnExit).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(constants.OnSuccess).State().Status)
	require.Equal(t, NodeStatusSuccess, sc.HandlerNode(constants.OnFailure).State().Status)
	require.Equal(t, NodeStatusNone, sc.HandlerNode(constants.OnCancel).State().Status)
}

func TestRepeat(t *testing.T) {
	g, _ := NewExecutionGraph(
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
	sc := &Scheduler{Config: &Config{}}

	go func() {
		timer := time.NewTimer(time.Millisecond * 3000)
		defer timer.Stop()
		<-timer.C
		sc.Cancel(g)
	}()

	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), StatusCancel)
	require.Equal(t, NodeStatusCancel, nodes[0].State().Status)
	require.Equal(t, nodes[0].DoneCount, 2)
}

func TestRepeatFail(t *testing.T) {
	g, _ := NewExecutionGraph(
		dag.Step{
			Name:    "1",
			Command: testCommandFail,
			RepeatPolicy: dag.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := &Scheduler{Config: &Config{}}
	err := sc.Schedule(context.Background(), g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, sc.Status(g), StatusError)
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, nodes[0].DoneCount, 1)
}

func TestStopRepetitiveTaskGracefully(t *testing.T) {
	g, _ := NewExecutionGraph(
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
	sc := &Scheduler{Config: &Config{}}

	done := make(chan bool)
	go func() {
		timer := time.NewTimer(time.Millisecond * 100)
		defer timer.Stop()
		<-timer.C
		sc.Signal(g, syscall.SIGTERM, done, false)
	}()

	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)
	<-done

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), StatusSuccess)
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, nodes[0].DoneCount, 1)
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
		dag.Step{
			Name:    "1",
			Command: "sh",
			Dir:     "~/",
			Script:  "echo 1",
		},
	)
	sc := &Scheduler{Config: &Config{}}
	err := sc.Schedule(context.Background(), g, nil)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), StatusError)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusError, nodes[0].State().Status)
	require.Equal(t, nodes[0].DoneCount, 0)
}

func TestNodeTeardownFailure(t *testing.T) {
	g, _ := NewExecutionGraph(
		dag.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
		},
	)
	sc := &Scheduler{Config: &Config{}}

	nodes := g.Nodes()
	go func() {
		time.Sleep(time.Millisecond * 300)
		nodes[0].mu.Lock()
		_ = nodes[0].logFile.Close()
		nodes[0].mu.Unlock()
	}()

	err := sc.Schedule(context.Background(), g, nil)
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

	g, sc := newTestSchedule(t, &Config{}, s1, s2)
	err := sc.Schedule(context.Background(), g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusSuccess, nodes[1].State().Status)

	require.Equal(t, "take-output", os.ExpandEnv("$TOOK_PREV_OUT"))
}

func step(name, command string, depends ...string) dag.Step {
	cmd, args := util.SplitCommand(command, false)
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
	g, sc := newTestSchedule(t,
		&Config{MaxActiveRuns: 2}, steps...)
	return g, sc, sc.Schedule(context.Background(), g, nil)
}

func newTestSchedule(t *testing.T, cfg *Config, steps ...dag.Step) (
	*ExecutionGraph, *Scheduler,
) {
	t.Helper()
	g, err := NewExecutionGraph(steps...)
	require.NoError(t, err)
	return g, &Scheduler{Config: cfg}
}
