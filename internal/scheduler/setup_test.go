package scheduler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"
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
	manager   scheduler.JobManager
	runClient client.RunClient
	dagClient client.DAGClient
	config    *config.Config
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
		},
		Global: config.Global{
			WorkDir: tempDir,
		},
	}

	dagStore := local.NewDAGStore(cfg.Paths.DAGsDir)
	historyStore := jsondb.New(cfg.Paths.DataDir)
	flagStore := local.NewFlagStore(storage.NewStorage(cfg.Paths.SuspendFlagsDir))
	runCli := client.New(historyStore, "", cfg.Global.WorkDir)
	dagCli := client.NewDAGClient(runCli, dagStore, flagStore)
	jobManager := scheduler.NewDAGJobManager(testdataDir, dagCli, runCli, "", "")

	return testHelper{
		manager:   jobManager,
		dagClient: dagCli,
		runClient: runCli,
		config:    cfg,
	}
}
