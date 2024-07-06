package scheduler

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/util"
)

var (
	testHomeDir string
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	tempDir := util.MustTempDir("runner_test")
	err := os.Setenv("HOME", tempDir)
	if err != nil {
		panic(err)
	}
	testHomeDir = tempDir
	code := m.Run()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestRun(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	setFixedTime(now)

	er := &mockEntryReader{
		Entries: []*Entry{
			{
				Job:    &mockJob{},
				Next:   now,
				Logger: logger.NewSlogLogger(),
			},
			{
				Job:    &mockJob{},
				Next:   now.Add(time.Minute),
				Logger: logger.NewSlogLogger(),
			},
		},
	}

	schedulerInstance := NewScheduler(NewSchedulerArgs{
		EntryReader: er,
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})

	go func() {
		_ = schedulerInstance.Start()
	}()

	time.Sleep(time.Second + time.Millisecond*100)
	schedulerInstance.Stop()

	require.Equal(t, int32(1), er.Entries[0].Job.(*mockJob).RunCount.Load())
	require.Equal(t, int32(0), er.Entries[1].Job.(*mockJob).RunCount.Load())
}

func TestRestart(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	setFixedTime(now)

	entryReader := &mockEntryReader{
		Entries: []*Entry{
			{
				EntryType: Restart,
				Job:       &mockJob{},
				Next:      now,
				Logger:    logger.NewSlogLogger(),
			},
		},
	}

	schedulerInstance := NewScheduler(NewSchedulerArgs{
		EntryReader: entryReader,
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})

	go func() {
		_ = schedulerInstance.Start()
	}()
	defer schedulerInstance.Stop()

	time.Sleep(time.Second + time.Millisecond*100)
	require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RestartCount.Load())
}

func TestNextTick(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
	setFixedTime(now)
	schedulerInstance := NewScheduler(NewSchedulerArgs{
		EntryReader: &mockEntryReader{},
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})
	next := schedulerInstance.nextTick(now)
	require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
}

var _ EntryReader = (*mockEntryReader)(nil)

type mockEntryReader struct {
	Entries []*Entry
}

func (er *mockEntryReader) Read(_ time.Time) ([]*Entry, error) {
	return er.Entries, nil
}

func (er *mockEntryReader) Start(chan any) {}

var _ Job = (*mockJob)(nil)

type mockJob struct {
	DAG          *dag.DAG
	Name         string
	RunCount     atomic.Int32
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
}

func newMockJob(dag *dag.DAG) *mockJob {
	return &mockJob{
		DAG:  dag,
		Name: dag.Name,
	}
}

func (j *mockJob) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *mockJob) String() string {
	return j.Name
}

func (j *mockJob) Start() error {
	j.RunCount.Add(1)
	if j.Panic != nil {
		panic(j.Panic)
	}
	return nil
}

func (j *mockJob) Stop() error {
	j.StopCount.Add(1)
	return nil
}

func (j *mockJob) Restart() error {
	j.RestartCount.Add(1)
	return nil
}

func Test_fixedTIme(t *testing.T) {
	t.Run("now", func(t *testing.T) {
		fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		setFixedTime(fixedTime)
		require.Equal(t, fixedTime, now())

		// Reset
		setFixedTime(time.Time{})
		require.NotEqual(t, fixedTime, now())
	})
}
