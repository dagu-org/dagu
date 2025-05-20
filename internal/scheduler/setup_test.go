package scheduler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/localdag"
	"github.com/dagu-org/dagu/internal/persistence/localhistory"
	"github.com/dagu-org/dagu/internal/persistence/localproc"
	"github.com/dagu-org/dagu/internal/persistence/localqueue/prototype"
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
	manager        scheduler.JobManager
	historyManager history.Manager
	historyStore   models.HistoryStore
	dagStore       models.DAGStore
	procStore      models.ProcStore
	queueStore     models.QueueStore
	config         *config.Config
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
			HistoryDir:      filepath.Join(tempDir, "."+build.Slug, "data", "history"),
		},
		Global: config.Global{WorkDir: tempDir},
	}

	ds := localdag.New(cfg.Paths.DAGsDir, localdag.WithFlagsBaseDir(cfg.Paths.SuspendFlagsDir))
	hs := localhistory.New(cfg.Paths.HistoryDir)
	ps := localproc.New(cfg.Paths.ProcDir)
	qs := prototype.New(cfg.Paths.QueueDir)

	hm := history.New(hs, "", cfg.Global.WorkDir)
	jm := scheduler.NewDAGJobManager(testdataDir, ds, hm, "", "")

	return testHelper{
		manager:        jm,
		dagStore:       ds,
		historyStore:   hs,
		historyManager: hm,
		config:         cfg,
		procStore:      ps,
		queueStore:     qs,
	}
}
