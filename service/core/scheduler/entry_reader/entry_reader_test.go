package entry_reader

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/dagu-dev/dagu/service/core/scheduler/scheduler"
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

// TODO: fix this tests to use mock
func setupTest(t *testing.T) (string, engine.Factory) {
	t.Helper()

	tmpDir := utils.MustTempDir("dagu_test")
	_ = os.Setenv("HOME", tmpDir)
	_ = config.LoadConfig(tmpDir)

	ds := client.NewDataStoreFactory(&config.Config{
		DataDir:         path.Join(tmpDir, ".dagu", "data"),
		DAGs:            testdataDir,
		SuspendFlagsDir: tmpDir,
	})

	ef := engine.NewFactory(ds, &config.Config{
		Command: path.Join(utils.MustGetwd(), "../../bin/dagu"),
	})

	return tmpDir, ef
}

func TestReadEntries(t *testing.T) {
	tmpDir, ef := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)

	er := New(Params{
		DagsDir:       path.Join(testdataDir, "invalid_directory"),
		JobFactory:    &mockJobFactory{},
		Logger:        logger.NewSlogLogger(),
		EngineFactory: ef,
	})
	entries, err := er.Read(now)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	er = New(Params{
		DagsDir:       testdataDir,
		JobFactory:    &mockJobFactory{},
		Logger:        logger.NewSlogLogger(),
		EngineFactory: ef,
	})
	entries, err = er.Read(now)
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

	e := ef.Create()
	err = e.ToggleSuspend(j.GetDAG().Name, true)
	require.NoError(t, err)

	// check if the job is suspended
	lives, err := er.Read(now)
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
