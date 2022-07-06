package runner

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yohamta/dagu/internal/admin"
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

func TestRun(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	utils.FixedTime = now

	er := &mockEntryReader{
		Entries: []*Entry{
			{
				Job:  &mockJob{},
				Next: now,
			},
			{
				Job:  &mockJob{},
				Next: now.Add(time.Minute),
			},
		},
	}

	r := New(er)

	go func() {
		r.Start()
	}()

	time.Sleep(time.Second + time.Millisecond*100)
	r.Stop()

	require.Equal(t, 1, er.Entries[0].Job.(*mockJob).RunCount)
	require.Equal(t, 0, er.Entries[1].Job.(*mockJob).RunCount)
}

func TestNextTick(t *testing.T) {
	n := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
	utils.FixedTime = n
	r := New(&entryReader{})
	next := r.nextTick(n)
	require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
}

type mockEntryReader struct {
	Entries []*Entry
}

var _ EntryReader = (*mockEntryReader)(nil)

func (er *mockEntryReader) Read(now time.Time) ([]*Entry, error) {
	return er.Entries, nil
}

type mockJob struct {
	Name     string
	RunCount int
	Panic    error
}

var _ Job = (*mockJob)(nil)

func (j *mockJob) String() string {
	return j.Name
}

func (j *mockJob) Run() error {
	j.RunCount++
	if j.Panic != nil {
		panic(j.Panic)
	}
	return nil
}
