package entry_reader

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/dagu-dev/dagu/service/scheduler/scheduler"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/config"
)

var (
	testdataDir = path.Join(utils.MustGetwd(), "testdata")
)

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("runner_test")
	changeHomeDir(tempDir)
	code := m.Run()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	_ = os.Setenv("HOME", homeDir)
	_ = config.LoadConfig(homeDir)
}

func TestReadEntries(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)

	r := NewEntryReader(Params{
		DagsDir:    path.Join(testdataDir, "invalid_directory"),
		JobFactory: &mockJobFactory{},
		Logger:     logger.NewSlogLogger(),
	})
	entries, err := r.Read(now)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	r = NewEntryReader(Params{
		DagsDir:    testdataDir,
		JobFactory: &mockJobFactory{},
		Logger:     logger.NewSlogLogger(),
	})
	entries, err = r.Read(now)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(entries), 1)

	next := entries[0].Next
	require.Equal(t, now.Add(time.Second), next)

	// suspend
	var j scheduler.Job
	for _, e := range entries {
		jj := e.Job
		if jj.GetDAG().Name == "scheduled_job" {
			j = jj
			break
		}
	}
	sc := suspend.NewSuspendChecker(storage.NewStorage(config.Get().SuspendFlagsDir))
	err = sc.ToggleSuspend(j.GetDAG(), true)
	require.NoError(t, err)

	// check if the job is suspended
	lives, err := r.Read(now)
	require.NoError(t, err)
	require.Equal(t, len(entries)-1, len(lives))
}

// TODO: fix to use mock library
type mockJobFactory struct {
}

func (f *mockJobFactory) NewJob(d *dag.DAG, next time.Time) scheduler.Job {
	return &mockJob{
		DAG: d,
	}
}

// TODO: fix to use mock library
type mockJob struct {
	DAG          *dag.DAG
	Name         string
	RunCount     int
	StopCount    int
	RestartCount int
	Panic        error
}

var _ scheduler.Job = (*mockJob)(nil)

func (j *mockJob) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *mockJob) String() string {
	return j.Name
}

func (j *mockJob) Start() error {
	j.RunCount++
	if j.Panic != nil {
		panic(j.Panic)
	}
	return nil
}

func (j *mockJob) Stop() error {
	j.StopCount++
	return nil
}

func (j *mockJob) Restart() error {
	j.RestartCount++
	return nil
}
