package scheduler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/localdag"
	"github.com/dagu-org/dagu/internal/persistence/localdagrun"
	"github.com/dagu-org/dagu/internal/persistence/localproc"
	"github.com/dagu-org/dagu/internal/persistence/localqueue"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var testHomeDir string

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	tempDir := fileutil.MustTempDir("runner_test")
	err := os.Setenv("HOME", tempDir)
	if err != nil {
		panic(err)
	}

	testHomeDir = tempDir
	code := m.Run()

	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}

type testHelper struct {
	EntryReader scheduler.EntryReader
	DAGRunMgr   dagrun.Manager
	DAGRunStore models.DAGRunStore
	DAGStore    models.DAGStore
	ProcStore   models.ProcStore
	QueueStore  models.QueueStore
	Config      *config.Config
}

func setupTest(t *testing.T) testHelper {
	t.Helper()

	tempDir := fileutil.MustTempDir("test")
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	err := os.Setenv("HOME", tempDir)
	require.NoError(t, err)

	testdataDir := test.TestdataPath(t, filepath.Join("scheduler"))

	cfg := &config.Config{
		Paths: config.PathsConfig{
			DataDir:         filepath.Join(tempDir, "."+build.Slug, "data"),
			DAGsDir:         testdataDir,
			SuspendFlagsDir: tempDir,
			DAGRunsDir:      filepath.Join(tempDir, "."+build.Slug, "data", "dag-runs"),
		},
		Global: config.Global{WorkDir: tempDir},
	}

	ds := localdag.New(cfg.Paths.DAGsDir, localdag.WithFlagsBaseDir(cfg.Paths.SuspendFlagsDir))
	drs := localdagrun.New(cfg.Paths.DAGRunsDir)
	ps := localproc.New(cfg.Paths.ProcDir)
	qs := localqueue.New(cfg.Paths.QueueDir)

	drm := dagrun.New(drs, cfg.Paths.Executable, cfg.Global.WorkDir)
	em := scheduler.NewEntryReader(testdataDir, ds, drm, "", "")

	return testHelper{
		EntryReader: em,
		DAGStore:    ds,
		DAGRunStore: drs,
		DAGRunMgr:   drm,
		Config:      cfg,
		ProcStore:   ps,
		QueueStore:  qs,
	}
}
