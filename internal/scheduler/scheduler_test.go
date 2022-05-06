package scheduler_test

import (
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/scheduler"
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
	g, err := scheduler.NewExecutionGraph(
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommandFail, "2"),
		step("4", testCommand, "3"),
	)
	require.NoError(t, err)
	sc := scheduler.New(&scheduler.Config{
		MaxActiveRuns: 1,
	})

	counter := 0
	done := make(chan *scheduler.Node)
	go func() {
		for range done {
			counter += 1
		}
	}()
	require.Error(t, sc.Schedule(g, done))
	assert.Equal(t, counter, 3)
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Error)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[2].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[3].ReadStatus())
}

func TestSchedulerParallel(t *testing.T) {
	g, sc := newTestSchedule(t,
		&scheduler.Config{
			MaxActiveRuns: 1000,
		},
		step("1", testCommand),
		step("2", testCommand),
		step("3", testCommand),
	)
	err := sc.Schedule(g, nil)
	require.NoError(t, err)
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Success)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())
}

func TestSchedulerFailPartially(t *testing.T) {
	g, sc, err := testSchedule(t,
		step("1", testCommand),
		step("2", testCommandFail),
		step("3", testCommand, "1"),
		step("4", testCommand, "3"),
	)
	require.Error(t, err)
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Error)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[3].ReadStatus())
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
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Error)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())
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
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Success)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Skipped, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())
}

func TestSchedulerCancel(t *testing.T) {

	g, _ := scheduler.NewExecutionGraph(
		step("1", testCommand),
		step("2", "sleep 1000", "1"),
		step("3", testCommandFail, "2"),
	)
	sc := scheduler.New(&scheduler.Config{
		MaxActiveRuns: 1,
	})

	done := make(chan bool)
	go func() {
		<-time.After(time.Millisecond * 100)
		sc.Cancel(g)
		done <- true
	}()

	_ = sc.Schedule(g, nil)

	<-done
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Cancel)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].Status)
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[1].Status)
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[2].Status)
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
	assert.True(t, err != nil)
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Error)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[2].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[3].ReadStatus())

	assert.Equal(t, nodes[0].ReadRetryCount(), 1)
	assert.Equal(t, nodes[1].ReadRetryCount(), 1)
}

func TestSchedulerRetrySuccess(t *testing.T) {
	cmd := path.Join(testBinDir, "testfile.sh")
	tmpDir, err := ioutil.TempDir("", "scheduler_test")
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
	assert.NoError(t, err)
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Success)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())

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
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Success)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Skipped, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Skipped, nodes[2].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[3].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[4].ReadStatus())
}

func TestSchedulerOnExit(t *testing.T) {
	g, sc := newTestSchedule(t,
		&scheduler.Config{
			OnExit: step("onExit", testCommand),
		},
		step("1", testCommand),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())

	onExit := sc.HanderNode(constants.OnExit)
	require.NotNil(t, onExit)
	assert.Equal(t, scheduler.NodeStatus_Success, onExit.ReadStatus())
}

func TestSchedulerOnExitOnFail(t *testing.T) {
	g, sc := newTestSchedule(t,
		&scheduler.Config{
			OnExit: step("onExit", testCommand),
		},
		step("1", testCommandFail),
		step("2", testCommand, "1"),
		step("3", testCommand),
	)

	err := sc.Schedule(g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[2].ReadStatus())

	assert.Equal(t, scheduler.NodeStatus_Success, sc.HanderNode(constants.OnExit).ReadStatus())
}

func TestSchedulerOnSignal(t *testing.T) {
	g, _ := scheduler.NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"10"},
		},
	)
	sc := scheduler.New(&scheduler.Config{})

	go func() {
		<-time.After(time.Millisecond * 50)
		sc.Signal(g, syscall.SIGTERM, nil)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Cancel)
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[0].Status)
}

func TestSchedulerOnCancel(t *testing.T) {
	g, sc := newTestSchedule(t,
		&scheduler.Config{
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
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Cancel)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[1].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_None, sc.HanderNode(constants.OnSuccess).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_None, sc.HanderNode(constants.OnFailure).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, sc.HanderNode(constants.OnCancel).ReadStatus())
}

func TestSchedulerOnSuccess(t *testing.T) {
	g, sc := newTestSchedule(t,
		&scheduler.Config{
			OnExit:    step("onExit", testCommand),
			OnSuccess: step("onSuccess", testCommand),
			OnFailure: step("onFailure", testCommand),
		},
		step("1", testCommand),
	)

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, sc.HanderNode(constants.OnExit).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, sc.HanderNode(constants.OnSuccess).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_None, sc.HanderNode(constants.OnFailure).ReadStatus())
}

func TestSchedulerOnFailure(t *testing.T) {
	g, sc := newTestSchedule(t,
		&scheduler.Config{
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
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[0].ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, sc.HanderNode(constants.OnExit).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_None, sc.HanderNode(constants.OnSuccess).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_Success, sc.HanderNode(constants.OnFailure).ReadStatus())
	assert.Equal(t, scheduler.NodeStatus_None, sc.HanderNode(constants.OnCancel).ReadStatus())
}

func TestRepeat(t *testing.T) {
	g, _ := scheduler.NewExecutionGraph(
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
	sc := scheduler.New(&scheduler.Config{})

	go func() {
		<-time.After(time.Millisecond * 3000)
		sc.Cancel(g)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)

	nodes := g.Nodes()

	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Cancel)
	assert.Equal(t, scheduler.NodeStatus_Cancel, nodes[0].Status)
	assert.Equal(t, nodes[0].DoneCount, 2)
}

func TestRepeatFail(t *testing.T) {
	g, _ := scheduler.NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: testCommandFail,
			RepeatPolicy: config.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := scheduler.New(&scheduler.Config{})
	err := sc.Schedule(g, nil)
	require.Error(t, err)

	nodes := g.Nodes()
	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Error)
	assert.Equal(t, scheduler.NodeStatus_Error, nodes[0].Status)
	assert.Equal(t, nodes[0].DoneCount, 1)
}

func TestStopRepetitiveTaskGracefully(t *testing.T) {
	g, _ := scheduler.NewExecutionGraph(
		&config.Step{
			Name:    "1",
			Command: "sleep",
			Args:    []string{"10"},
			RepeatPolicy: config.RepeatPolicy{
				Repeat:   true,
				Interval: time.Millisecond * 300,
			},
		},
	)
	sc := scheduler.New(&scheduler.Config{})

	done := make(chan bool)
	go func() {
		<-time.After(time.Millisecond * 100)
		sc.Signal(g, syscall.SIGTERM, done)
	}()

	err := sc.Schedule(g, nil)
	require.NoError(t, err)
	<-done

	nodes := g.Nodes()

	assert.Equal(t, sc.Status(g), scheduler.SchedulerStatus_Success)
	assert.Equal(t, scheduler.NodeStatus_Success, nodes[0].Status)
	assert.Equal(t, nodes[0].DoneCount, 1)
}

func testSchedule(t *testing.T, steps ...*config.Step) (
	*scheduler.ExecutionGraph, *scheduler.Scheduler, error,
) {
	t.Helper()
	g, sc := newTestSchedule(t,
		&scheduler.Config{MaxActiveRuns: 2}, steps...)
	return g, sc, sc.Schedule(g, nil)
}

func newTestSchedule(t *testing.T, c *scheduler.Config, steps ...*config.Step) (
	*scheduler.ExecutionGraph, *scheduler.Scheduler,
) {
	t.Helper()
	g, err := scheduler.NewExecutionGraph(steps...)
	require.NoError(t, err)
	return g, scheduler.New(c)
}

func TestSchedulerStatusText(t *testing.T) {
	for k, v := range map[scheduler.SchedulerStatus]string{
		scheduler.SchedulerStatus_None:    "not started",
		scheduler.SchedulerStatus_Running: "running",
		scheduler.SchedulerStatus_Error:   "failed",
		scheduler.SchedulerStatus_Cancel:  "canceled",
		scheduler.SchedulerStatus_Success: "finished",
	} {
		assert.Equal(t, k.String(), v)
	}

	for k, v := range map[scheduler.NodeStatus]string{
		scheduler.NodeStatus_None:    "not started",
		scheduler.NodeStatus_Running: "running",
		scheduler.NodeStatus_Error:   "failed",
		scheduler.NodeStatus_Cancel:  "canceled",
		scheduler.NodeStatus_Success: "finished",
		scheduler.NodeStatus_Skipped: "skipped",
	} {
		assert.Equal(t, k.String(), v)
	}
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
