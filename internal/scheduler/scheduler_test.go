package scheduler

import (
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var (
	testCommand     = "true"
	testCommandFail = "false"
	testBinDir      = path.Join(utils.MustGetwd(), "../../tests/bin")
	testDir         string
)

func TestMain(m *testing.M) {
	testDir = utils.MustTempDir("scheduler-test")
	settings.InitTest(testDir)
	code := m.Run()
	os.RemoveAll(testDir)
	os.Exit(code)
}

func TestScheduler(t *testing.T) {
	g, err := NewExecutionGraph(
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommandFail, "2"),
		step("4", testCommand, "3"),
	)
	require.NoError(t, err)
	sc := New(&Config{
		MaxActiveRuns: 1,
	})

	counter := 0
	done := make(chan *Node)
	go func() {
		for range done {
			counter += 1
		}
	}()
	require.Error(t, sc.Schedule(g, done))
	require.Equal(t, counter, 3)
	require.Equal(t, sc.Status(g), SchedulerStatus_Error)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Error, nodes[2].ReadStatus())
	require.Equal(t, NodeStatus_Cancel, nodes[3].ReadStatus())
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
	err := sc.Schedule(g, nil)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Success)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())
}

func TestSchedulerFailPartially(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		step("2", testCommandFail),
		step("3", testCommand, "1"),
		step("4", testCommand, "3"),
	)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Error)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Error, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[3].ReadStatus())
}

func TestSchedulerContinueOnFailure(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		&config.Step{
			Name:    "2",
			Command: testCommandFail,
			Depends: []string{"1"},
			ContinueOn: config.ContinueOn{
				Failure: true,
			},
		},
		step("3", testCommand, "2"),
	)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Error)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Error, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())
}

func TestSchedulerAllowSkipped(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		&config.Step{
			Name:    "2",
			Command: testCommand,
			Depends: []string{"1"},
			Preconditions: []*config.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "0",
				},
			},
			ContinueOn: config.ContinueOn{Skipped: true},
		},
		step("3", testCommand, "2"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Success)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Skipped, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())
}

func TestSchedulerCancel(t *testing.T) {

	g, _ := NewExecutionGraph(
		step("1", testCommand),
		step("2", "sleep 1000", "1"),
		step("3", testCommandFail, "2"),
	)
	sc := New(&Config{
		MaxActiveRuns: 1,
	})

	go func() {
		time.Sleep(time.Millisecond * 300)
		sc.Cancel(g)
	}()

	_ = sc.Schedule(g, nil)

	require.Eventually(t, func() bool {
		return sc.Status(g) == SchedulerStatus_Cancel
	}, time.Second, time.Millisecond*10)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].Status)
	require.Equal(t, NodeStatus_Cancel, nodes[1].Status)
	require.Equal(t, NodeStatus_None, nodes[2].Status)
}

func TestSchedulerRetryFail(t *testing.T) {
	cmd := path.Join(testBinDir, "testfile.sh")
	g, sc, err := testSchedule(t,
		&config.Step{
			Name:        "1",
			Command:     cmd,
			ContinueOn:  config.ContinueOn{Failure: true},
			RetryPolicy: &config.RetryPolicy{Limit: 1},
		},
		&config.Step{
			Name:        "2",
			Command:     cmd,
			Args:        []string{"flag"},
			ContinueOn:  config.ContinueOn{Failure: true},
			RetryPolicy: &config.RetryPolicy{Limit: 1},
			Depends:     []string{"1"},
		},
		&config.Step{
			Name:    "3",
			Command: cmd,
			Depends: []string{"2"},
		},
		step("4", cmd, "3"),
	)
	require.True(t, err != nil)
	require.Equal(t, sc.Status(g), SchedulerStatus_Error)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Error, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Error, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Error, nodes[2].ReadStatus())
	require.Equal(t, NodeStatus_Cancel, nodes[3].ReadStatus())

	require.Equal(t, nodes[0].ReadRetryCount(), 1)
	require.Equal(t, nodes[1].ReadRetryCount(), 1)
}

func TestSchedulerRetrySuccess(t *testing.T) {
	cmd := path.Join(testBinDir, "testfile.sh")
	tmpDir, err := os.MkdirTemp("", "scheduler_test")
	tmpFile := path.Join(tmpDir, "flag")

	require.NoError(t, err)
	defer os.Remove(tmpDir)

	go func() {
		<-time.After(time.Millisecond * 300)
		f, err := os.Create(tmpFile)
		require.NoError(t, err)
		f.Close()
	}()

	g, sc, err := testSchedule(t,
		step("1", testCommand),
		&config.Step{
			Name:        "2",
			Command:     cmd,
			Args:        []string{tmpFile},
			Depends:     []string{"1"},
			RetryPolicy: &config.RetryPolicy{Limit: 10},
		},
		step("3", testCommand, "2"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Success)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())

	if nodes[1].ReadRetryCount() == 0 {
		t.Error("step 2 Should be retried")
	}
}

func TestStepPreCondition(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		&config.Step{
			Name:    "2",
			Command: testCommand,
			Depends: []string{"1"},
			Preconditions: []*config.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "0",
				},
			},
		},
		step("3", testCommand, "2"),
		&config.Step{
			Name:    "4",
			Command: testCommand,
			Preconditions: []*config.Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
			},
		},
		step("5", testCommand, "4"),
	)
	require.NoError(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Success)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Skipped, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Skipped, nodes[2].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[3].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[4].ReadStatus())
}

func TestSchedulerOnExit(t *testing.T) {
	g, sc := newTestSchedule(t,
		&Config{
			OnExit: step("onExit", testCommand),
		},
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())

	onExit := sc.HandlerNode(constants.OnExit)
	require.NotNil(t, onExit)
	require.Equal(t, NodeStatus_Success, onExit.ReadStatus())
}

func TestSchedulerOnExitOnFail(t *testing.T) {
	g, sc := newTestSchedule(t,
		&Config{
			OnExit: step("onExit", testCommand),
		},
		step("1", testCommandFail),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Error, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Cancel, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_Success, nodes[2].ReadStatus())

	require.Equal(t, NodeStatus_Success, sc.HandlerNode(constants.OnExit).ReadStatus())
}

func TestSchedulerOnSignal(t *testing.T) {
	g, _ := NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"10"},
		},
	)
	sc := New(&Config{})

	go func() {
		<-time.After(time.Millisecond * 50)
		sc.Signal(g, syscall.SIGTERM, nil)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), SchedulerStatus_Cancel)
	require.Equal(t, NodeStatus_Cancel, nodes[0].Status)
}

func TestSchedulerOnCancel(t *testing.T) {
	g, sc := newTestSchedule(t,
		&Config{
			OnSuccess: step("onSuccess", testCommand),
			OnFailure: step("onFailure", testCommand),
			OnCancel:  step("onCancel", testCommand),
		},
		step("1", testCommand),
		step("2", "sleep 60", "1"),
	)

	done := make(chan bool)
	go func() {
		<-time.After(time.Millisecond * 500)
		sc.Signal(g, syscall.SIGTERM, done)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)
	<-done // Wait for canceling finished
	require.Equal(t, sc.Status(g), SchedulerStatus_Cancel)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Cancel, nodes[1].ReadStatus())
	require.Equal(t, NodeStatus_None, sc.HandlerNode(constants.OnSuccess).ReadStatus())
	require.Equal(t, NodeStatus_None, sc.HandlerNode(constants.OnFailure).ReadStatus())
	require.Equal(t, NodeStatus_Success, sc.HandlerNode(constants.OnCancel).ReadStatus())
}

func TestSchedulerOnSuccess(t *testing.T) {
	g, sc := newTestSchedule(t,
		&Config{
			OnExit:    step("onExit", testCommand),
			OnSuccess: step("onSuccess", testCommand),
			OnFailure: step("onFailure", testCommand),
		},
		step("1", testCommand),
	)

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Success, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Success, sc.HandlerNode(constants.OnExit).ReadStatus())
	require.Equal(t, NodeStatus_Success, sc.HandlerNode(constants.OnSuccess).ReadStatus())
	require.Equal(t, NodeStatus_None, sc.HandlerNode(constants.OnFailure).ReadStatus())
}

func TestSchedulerOnFailure(t *testing.T) {
	g, sc := newTestSchedule(t,
		&Config{
			OnExit:    step("onExit", testCommand),
			OnSuccess: step("onSuccess", testCommand),
			OnFailure: step("onFailure", testCommand),
			OnCancel:  step("onCancel", testCommand),
		},
		step("1", testCommandFail),
	)

	err := sc.Schedule(g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Error, nodes[0].ReadStatus())
	require.Equal(t, NodeStatus_Success, sc.HandlerNode(constants.OnExit).ReadStatus())
	require.Equal(t, NodeStatus_None, sc.HandlerNode(constants.OnSuccess).ReadStatus())
	require.Equal(t, NodeStatus_Success, sc.HandlerNode(constants.OnFailure).ReadStatus())
	require.Equal(t, NodeStatus_None, sc.HandlerNode(constants.OnCancel).ReadStatus())
}

func TestRepeat(t *testing.T) {
	g, _ := NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			RepeatPolicy: config.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := New(&Config{})

	go func() {
		<-time.After(time.Millisecond * 3000)
		sc.Cancel(g)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), SchedulerStatus_Cancel)
	require.Equal(t, NodeStatus_Cancel, nodes[0].Status)
	require.Equal(t, nodes[0].DoneCount, 2)
}

func TestRepeatFail(t *testing.T) {
	g, _ := NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: testCommandFail,
			RepeatPolicy: config.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := New(&Config{})
	err := sc.Schedule(g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	require.Equal(t, sc.Status(g), SchedulerStatus_Error)
	require.Equal(t, NodeStatus_Error, nodes[0].Status)
	require.Equal(t, nodes[0].DoneCount, 1)
}

func TestStopRepetitiveTaskGracefully(t *testing.T) {
	g, _ := NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			RepeatPolicy: config.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := New(&Config{})

	done := make(chan bool)
	go func() {
		<-time.After(time.Millisecond * 100)
		sc.Signal(g, syscall.SIGTERM, done)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)
	<-done

	nodes := g.Nodes()

	require.Equal(t, sc.Status(g), SchedulerStatus_Success)
	require.Equal(t, NodeStatus_Success, nodes[0].Status)
	require.Equal(t, nodes[0].DoneCount, 1)
}

func testSchedule(t *testing.T, steps ...*config.Step) (
	*ExecutionGraph, *Scheduler, error,
) {
	t.Helper()
	g, sc := newTestSchedule(t,
		&Config{MaxActiveRuns: 2}, steps...)
	return g, sc, sc.Schedule(g, nil)
}

func newTestSchedule(t *testing.T, c *Config, steps ...*config.Step) (
	*ExecutionGraph, *Scheduler,
) {
	t.Helper()
	g, err := NewExecutionGraph(steps...)
	require.NoError(t, err)
	return g, New(c)
}

func TestSchedulerStatusText(t *testing.T) {
	for k, v := range map[SchedulerStatus]string{
		SchedulerStatus_None:    "not started",
		SchedulerStatus_Running: "running",
		SchedulerStatus_Error:   "failed",
		SchedulerStatus_Cancel:  "canceled",
		SchedulerStatus_Success: "finished",
	} {
		require.Equal(t, k.String(), v)
	}

	for k, v := range map[NodeStatus]string{
		NodeStatus_None:    "not started",
		NodeStatus_Running: "running",
		NodeStatus_Error:   "failed",
		NodeStatus_Cancel:  "canceled",
		NodeStatus_Success: "finished",
		NodeStatus_Skipped: "skipped",
	} {
		require.Equal(t, k.String(), v)
	}
}

func TestNodeSetupFailure(t *testing.T) {
	g, _ := NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sh",
			Dir:     "~/",
			Script:  "echo 1",
		},
	)
	sc := New(&Config{})
	err := sc.Schedule(g, nil)
	require.Error(t, err)
	require.Equal(t, sc.Status(g), SchedulerStatus_Error)

	nodes := g.Nodes()
	require.Equal(t, NodeStatus_Error, nodes[0].Status)
	require.Equal(t, nodes[0].DoneCount, 0)
}

func TestNodeTeardownFailure(t *testing.T) {
	g, _ := NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"1"},
			Dir:     "${HOME}",
		},
	)
	sc := New(&Config{})

	nodes := g.Nodes()
	go func() {
		time.Sleep(time.Millisecond * 300)
		nodes[0].logFile.Close()
	}()

	err := sc.Schedule(g, nil)
	require.Error(t, err)

	require.Equal(t, sc.Status(g), SchedulerStatus_Error)
	require.Equal(t, NodeStatus_Error, nodes[0].Status)
	require.Error(t, nodes[0].Error)
}

func step(name, command string, depends ...string) *config.Step {
	cmd, args := utils.SplitCommand(command)
	return &config.Step{
		Name:    name,
		Command: cmd,
		Args:    args,
		Depends: depends,
	}
}
