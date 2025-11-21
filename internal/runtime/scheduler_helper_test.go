package runtime_test

import (
	"fmt"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func successStep(name string, depends ...string) core.Step {
	return newStep(name, withDepends(depends...), withCommand("true"))
}

func failStep(name string, depends ...string) core.Step {
	return newStep(name, withDepends(depends...), withCommand("false"))
}

type stepOption func(*core.Step)

func withDepends(depends ...string) stepOption {
	return func(step *core.Step) {
		step.Depends = depends
	}
}

func withContinueOn(c core.ContinueOn) stepOption {
	return func(step *core.Step) {
		step.ContinueOn = c
	}
}

func withRetryPolicy(limit int, interval time.Duration) stepOption {
	return func(step *core.Step) {
		step.RetryPolicy.Limit = limit
		step.RetryPolicy.Interval = interval
	}
}

func withRepeatPolicy(repeat bool, interval time.Duration) stepOption {
	return func(step *core.Step) {
		if repeat {
			step.RepeatPolicy.RepeatMode = core.RepeatModeWhile
		}
		step.RepeatPolicy.Interval = interval
	}
}

func withPrecondition(condition *core.Condition) stepOption {
	return func(step *core.Step) {
		step.Preconditions = []*core.Condition{condition}
	}
}

func withScript(script string) stepOption {
	return func(step *core.Step) {
		step.Script = script
	}
}

func withWorkingDir(dir string) stepOption {
	return func(step *core.Step) {
		step.Dir = dir
	}
}

func withOutput(output string) stepOption {
	return func(step *core.Step) {
		step.Output = output
	}
}

func withCommand(command string) stepOption {
	return func(step *core.Step) {
		cmd, args, err := cmdutil.SplitCommand(command)
		if err != nil {
			panic(fmt.Errorf("unexpected: %w", err))
		}
		step.CmdWithArgs = command
		step.Command = cmd
		step.Args = args
	}
}

func withID(id string) stepOption {
	return func(step *core.Step) {
		step.ID = id
	}
}

func withStepTimeout(d time.Duration) stepOption {
	return func(step *core.Step) {
		step.Timeout = d
	}
}

func newStep(name string, opts ...stepOption) core.Step {
	step := core.Step{Name: name}
	for _, opt := range opts {
		opt(&step)
	}

	return step
}

type testHelper struct {
	test.Helper

	scheduler *runtime.Scheduler
	cfg       *runtime.Config
}

type schedulerOption func(*runtime.Config)

func withTimeout(d time.Duration) schedulerOption {
	return func(cfg *runtime.Config) {
		cfg.Timeout = d
	}
}

func withMaxActiveRuns(n int) schedulerOption {
	return func(cfg *runtime.Config) {
		cfg.MaxActiveSteps = n
	}
}

func withOnExit(step core.Step) schedulerOption {
	return func(cfg *runtime.Config) {
		cfg.OnExit = &step
	}
}

func withOnCancel(step core.Step) schedulerOption {
	return func(cfg *runtime.Config) {
		cfg.OnCancel = &step
	}
}

func withOnSuccess(step core.Step) schedulerOption {
	return func(cfg *runtime.Config) {
		cfg.OnSuccess = &step
	}
}

func withOnFailure(step core.Step) schedulerOption {
	return func(cfg *runtime.Config) {
		cfg.OnFailure = &step
	}
}

func setupScheduler(t *testing.T, opts ...schedulerOption) testHelper {
	t.Helper()

	th := test.Setup(t)

	cfg := &runtime.Config{
		LogDir:   th.Config.Paths.LogDir,
		DAGRunID: uuid.Must(uuid.NewV7()).String(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	sc := runtime.New(cfg)

	return testHelper{
		Helper:    test.Setup(t),
		scheduler: sc,
		cfg:       cfg,
	}
}

func (th testHelper) newPlan(t *testing.T, steps ...core.Step) planHelper {
	t.Helper()

	plan, err := runtime.NewPlan(steps...)
	require.NoError(t, err)

	return planHelper{
		testHelper: th,
		Plan:       plan,
	}
}

type planHelper struct {
	testHelper
	*runtime.Plan
}

func (ph planHelper) assertSchedule(t *testing.T, expectedStatus core.Status) scheduleResult {
	t.Helper()

	dag := &core.DAG{Name: "test_dag"}
	logFilename := fmt.Sprintf("%s_%s.log", dag.Name, ph.cfg.DAGRunID)
	logFilePath := path.Join(ph.cfg.LogDir, logFilename)

	ctx := execution.SetupDAGContext(ph.Context, dag, nil, execution.DAGRunRef{}, ph.cfg.DAGRunID, logFilePath, nil, nil, nil)

	var doneNodes []*runtime.Node
	progressCh := make(chan *runtime.Node)

	done := make(chan struct{})
	go func() {
		for node := range progressCh {
			doneNodes = append(doneNodes, node)
		}
		done <- struct{}{}
	}()

	err := ph.scheduler.Schedule(ctx, ph.Plan, progressCh)

	close(progressCh)

	switch expectedStatus {
	case core.Succeeded, core.Aborted:
		require.NoError(t, err)

	case core.Failed, core.PartiallySucceeded:
		require.Error(t, err)

	case core.Running, core.NotStarted, core.Queued:
		t.Errorf("unexpected status %s", expectedStatus)

	}

	require.Equal(t, expectedStatus.String(), ph.scheduler.Status(ctx, ph.Plan).String(),
		"expected status %s, got %s", expectedStatus, ph.scheduler.Status(ctx, ph.Plan))

	// wait for items of nodeCompletedChan to be processed
	<-done
	close(done)

	return scheduleResult{
		planHelper: ph,
		Done:       doneNodes,
		Error:      err,
	}
}

func (ph planHelper) signal(sig syscall.Signal) {
	ph.scheduler.Signal(ph.Context, ph.Plan, sig, nil, false)
}

func (ph planHelper) cancelSchedule(t *testing.T) {
	t.Helper()

	ph.scheduler.Cancel(ph.Plan)
}

type scheduleResult struct {
	planHelper
	Done  []*runtime.Node
	Error error
}

func (sr scheduleResult) assertNodeStatus(t *testing.T, stepName string, expected core.NodeStatus) {
	t.Helper()

	target := sr.GetNodeByName(stepName)
	if target == nil {
		if sr.cfg.OnExit != nil && sr.cfg.OnExit.Name == stepName {
			target = sr.scheduler.HandlerNode(core.HandlerOnExit)
		}
		if sr.cfg.OnSuccess != nil && sr.cfg.OnSuccess.Name == stepName {
			target = sr.scheduler.HandlerNode(core.HandlerOnSuccess)
		}
		if sr.cfg.OnFailure != nil && sr.cfg.OnFailure.Name == stepName {
			target = sr.scheduler.HandlerNode(core.HandlerOnFailure)
		}
		if sr.cfg.OnCancel != nil && sr.cfg.OnCancel.Name == stepName {
			target = sr.scheduler.HandlerNode(core.HandlerOnCancel)
		}
	}

	if target == nil {
		t.Fatalf("step %s not found", stepName)
	}

	require.Equal(t, expected.String(), target.State().Status.String(), "expected status %q, got %q", expected.String(), target.State().Status.String())
}

func (sr scheduleResult) nodeByName(t *testing.T, stepName string) *runtime.Node {
	t.Helper()

	if node := sr.GetNodeByName(stepName); node != nil {
		return node
	}

	if sr.cfg.OnExit != nil && sr.cfg.OnExit.Name == stepName {
		return sr.scheduler.HandlerNode(core.HandlerOnExit)
	}
	if sr.cfg.OnSuccess != nil && sr.cfg.OnSuccess.Name == stepName {
		return sr.scheduler.HandlerNode(core.HandlerOnSuccess)
	}
	if sr.cfg.OnFailure != nil && sr.cfg.OnFailure.Name == stepName {
		return sr.scheduler.HandlerNode(core.HandlerOnFailure)
	}
	if sr.cfg.OnCancel != nil && sr.cfg.OnCancel.Name == stepName {
		return sr.scheduler.HandlerNode(core.HandlerOnCancel)
	}

	t.Fatalf("step %s not found", stepName)
	return nil
}
