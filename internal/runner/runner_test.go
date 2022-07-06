package runner

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var (
	testsDir    = path.Join(utils.MustGetwd(), "../../tests")
	testBin     = path.Join(utils.MustGetwd(), "../../bin/dagu")
	testConfig  = &admin.Config{Command: testBin}
	testHomeDir string
)

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("runner_test")
	settings.ChangeHomeDir(tempDir)
	testHomeDir = tempDir
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestReadEntries(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)

	r := New(&Config{
		Admin: &admin.Config{
			DAGs: path.Join(testsDir, "runner/invalid"),
		}})
	_, err := r.readEntries(now)
	require.Error(t, err)

	r = New(&Config{
		Admin: &admin.Config{
			DAGs: path.Join(testsDir, "runner"),
		}})
	entries, err := r.readEntries(now)
	require.NoError(t, err)

	require.Len(t, entries, 1)

	j := entries[0].Job.(*job)
	require.Equal(t, "scheduled_job", j.DAG.Name)

	next := entries[0].Next
	require.Equal(t, now.Add(time.Second), next)
}

func TestRun(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	utils.FixedTime = now

	r := New(&Config{
		Admin: &admin.Config{
			Command: testBin,
			DAGs:    testHomeDir,
		},
	})

	tests := []struct {
		Config *config.Config
		Want   scheduler.SchedulerStatus
	}{
		{
			Config: temporaryDAG(t, "job1",
				testDAGConfig(t, "0 1 * * *", "true")),
			Want: scheduler.SchedulerStatus_Success,
		},
		{
			Config: temporaryDAG(t, "job2",
				testDAGConfig(t, "10 1 * * *", "true")),
			Want: scheduler.SchedulerStatus_None,
		},
		{
			Config: temporaryDAG(t, "job3",
				testDAGConfig(t, "30 1 * * *", "true")),
			Want: scheduler.SchedulerStatus_None,
		},
	}

	go func() {
		r.Start()
	}()

	time.Sleep(time.Second + time.Millisecond*500)
	r.Stop()

	for _, tt := range tests {
		c := controller.New(tt.Config)
		s, err := c.GetLastStatus()
		require.NoError(t, err)
		require.Equal(t, tt.Want, s.Status)
	}
}

func TestRunOnlyOnce(t *testing.T) {
	cfg := temporaryDAG(t, "job1",
		testDAGConfig(t, "* * * * *", "true"))
	cont := controller.New(cfg)
	// now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	utils.FixedTime = time.Time{}

	startRunner := func() *Runner {
		r := New(&Config{
			Admin: &admin.Config{
				Command: testBin,
				DAGs:    testHomeDir,
			},
		})
		go func() {
			r.Start()
		}()
		return r
	}

	r := startRunner()
	time.Sleep(time.Second + time.Millisecond*100)
	r.Stop()

	s, _ := cont.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Success, s.Status)
	s.Status = scheduler.SchedulerStatus_Error
	cont.UpdateStatus(s)

	r = startRunner()
	time.Sleep(time.Second + time.Millisecond*100)
	r.Stop()

	s, _ = cont.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Error, s.Status)
}

func TestNextTick(t *testing.T) {
	n := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
	utils.FixedTime = n
	r := New(&Config{})
	next := r.nextTick(n)
	require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
}

func TestRunWithRecovery(t *testing.T) {
	dag := temporaryDAG(t, "failure",
		testDAGConfig(t, "* * * * *", "false"))
	job := &job{
		DAG: dag,
		Config: &admin.Config{
			Command: testBin,
			WorkDir: "",
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	go runWithRecovery(job)

	c := controller.New(dag)
	require.Eventually(t, func() bool {
		status, _ := c.GetLastStatus()
		return status.Status == scheduler.SchedulerStatus_Error
	}, time.Millisecond*1500, time.Millisecond*100)

	os.Stdout = origStdout
	w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, "exit status 1")
}

func testDAGConfig(t *testing.T, schedule, command string) string {
	t.Helper()
	return fmt.Sprintf(`schedule: "%s"
steps:
  - name: step1
    command: "%s"`, schedule, command)
}

func temporaryDAG(t *testing.T, name, cfg string) *config.Config {
	t.Helper()
	f := path.Join(testHomeDir, fmt.Sprintf("%s.yaml", name))
	err := os.WriteFile(f, []byte(cfg), 0644)
	require.NoError(t, err)
	cl := &config.Loader{}
	dag, err := cl.LoadHeadOnly(f)
	require.NoError(t, err)
	return dag
}
