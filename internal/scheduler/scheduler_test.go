package scheduler

import (
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/dagu-dev/dagu/internal/logger"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/util"
)

var testHomeDir string

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
		Entries: []*entry{
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

	schedulerInstance := newScheduler(newSchedulerArgs{
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
		Entries: []*entry{
			{
				EntryType: Restart,
				Job:       &mockJob{},
				Next:      now,
				Logger:    logger.NewSlogLogger(),
			},
		},
	}

	schedulerInstance := newScheduler(newSchedulerArgs{
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
	schedulerInstance := newScheduler(newSchedulerArgs{
		EntryReader: &mockEntryReader{},
		LogDir:      testHomeDir,
		Logger:      logger.NewSlogLogger(),
	})
	next := schedulerInstance.nextTick(now)
	require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
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
