package scheduler

import (
	"go.uber.org/goleak"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/util"
)

var (
	testHomeDir string
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	tempDir := util.MustTempDir("runner_test")
	changeHomeDir(tempDir)
	testHomeDir = tempDir
	code := m.Run()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	_ = os.Setenv("HOME", homeDir)
	_ = config.LoadConfig()
}

func TestRun(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	util.SetFixedTime(now)

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

	r := New(Params{
		EntryReader: er,
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})

	go func() {
		_ = r.Start()
	}()

	time.Sleep(time.Second + time.Millisecond*100)
	r.Stop()

	require.Equal(t, int32(1), er.Entries[0].Job.(*mockJob).RunCount.Load())
	require.Equal(t, int32(0), er.Entries[1].Job.(*mockJob).RunCount.Load())
}

func TestRestart(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	util.SetFixedTime(now)

	er := &mockEntryReader{
		Entries: []*Entry{
			{
				EntryType: Restart,
				Job:       &mockJob{},
				Next:      now,
				Logger:    logger.NewSlogLogger(),
			},
		},
	}

	r := New(Params{
		EntryReader: er,
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})

	go func() {
		_ = r.Start()
	}()
	defer r.Stop()

	time.Sleep(time.Second + time.Millisecond*100)
	require.Equal(t, int32(1), er.Entries[0].Job.(*mockJob).RestartCount.Load())
}

func TestNextTick(t *testing.T) {
	n := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
	util.SetFixedTime(n)
	r := New(Params{
		EntryReader: &mockEntryReader{},
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})
	next := r.nextTick(n)
	require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
}

type mockEntryReader struct {
	Entries []*Entry
}

var _ EntryReader = (*mockEntryReader)(nil)

func (er *mockEntryReader) Read(_ time.Time) ([]*Entry, error) {
	return er.Entries, nil
}

func (er *mockEntryReader) Start(chan any) {}

// TODO: fix to use mock library
type mockJob struct {
	Name         string
	RunCount     atomic.Int32
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
}

var _ Job = (*mockJob)(nil)

func (j *mockJob) GetDAG() *dag.DAG {
	return nil
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
