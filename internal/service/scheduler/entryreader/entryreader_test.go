package entryreader

import (
	"os"
	"path"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-dev/dagu/internal/service/scheduler/scheduler"
	"github.com/dagu-dev/dagu/internal/util"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/config"
)

var (
	testdataDir = path.Join(util.MustGetwd(), "testdata")
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	code := m.Run()
	os.Exit(code)
}

func setupTest(t *testing.T) (string, engine.Engine, *config.Config) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")

	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	cfg := &config.Config{
		DataDir:         path.Join(tmpDir, ".dagu", "data"),
		DAGs:            testdataDir,
		SuspendFlagsDir: tmpDir,
	}

	dataStore := client.NewDataStoreFactory(&client.NewDataStoreFactoryArgs{
		DAGs:            cfg.DAGs,
		DataDir:         cfg.DataDir,
		SuspendFlagsDir: cfg.SuspendFlagsDir,
	})

	return tmpDir, engine.New(&engine.NewEngineArgs{
		DataStore: dataStore,
	}), cfg
}

func TestReadEntries(t *testing.T) {
	tmpDir, eng, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)
	entryReader := New(Params{
		DagsDir:    path.Join(testdataDir, "invalid_directory"),
		JobFactory: &mockJobFactory{},
		Logger:     logger.NewSlogLogger(),
		Engine:     eng,
	})

	entries, err := entryReader.Read(now)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	entryReader = New(Params{
		DagsDir:    testdataDir,
		JobFactory: &mockJobFactory{},
		Logger:     logger.NewSlogLogger(),
		Engine:     eng,
	})

	done := make(chan any)
	defer close(done)
	entryReader.Start(done)

	entries, err = entryReader.Read(now)
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

	err = eng.ToggleSuspend(j.GetDAG().Name, true)
	require.NoError(t, err)

	// check if the job is suspended
	lives, err := entryReader.Read(now)
	require.NoError(t, err)
	require.Equal(t, len(entries)-1, len(lives))
}

type mockJobFactory struct{}

func (f *mockJobFactory) NewJob(dg *dag.DAG, next time.Time) scheduler.Job {
	return &mockJob{DAG: dg}
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
