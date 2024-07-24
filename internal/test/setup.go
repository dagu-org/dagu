package test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/daguflow/dagu/internal/client"
	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/persistence"
	dsclient "github.com/daguflow/dagu/internal/persistence/client"
	"github.com/daguflow/dagu/internal/util"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type Setup struct {
	Config *config.Config
	Logger logger.Logger

	homeDir string
}

func (t Setup) Cleanup() {
	_ = os.RemoveAll(t.homeDir)
}

func (t Setup) DataStore() persistence.DataStores {
	return dsclient.NewDataStores(
		t.Config.DAGs,
		t.Config.DataDir,
		t.Config.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: t.Config.LatestStatusToday,
		},
	)
}

func (t Setup) Client() client.Client {
	return client.New(
		t.DataStore(), t.Config.Executable, t.Config.WorkDir, logger.Default,
	)
}

var (
	lock sync.Mutex
)

func SetupTest(t *testing.T) Setup {
	lock.Lock()
	defer lock.Unlock()

	tmpDir := util.MustTempDir("dagu_test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	viper.AddConfigPath(config.ConfigDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	config.ConfigDir = filepath.Join(tmpDir, "config")
	config.DataDir = filepath.Join(tmpDir, "data")
	config.LogsDir = filepath.Join(tmpDir, "log")

	cfg, err := config.Load()
	require.NoError(t, err)

	// Set the executable path to the test binary.
	cfg.Executable = filepath.Join(util.MustGetwd(), "../../bin/dagu")

	// Set environment variables.
	// This is required for some tests that run the executable
	_ = os.Setenv("DAGU_DAGS_DIR", cfg.DAGs)
	_ = os.Setenv("DAGU_WORK_DIR", cfg.WorkDir)
	_ = os.Setenv("DAGU_BASE_CONFIG", cfg.BaseConfig)
	_ = os.Setenv("DAGU_LOG_DIR", cfg.LogDir)
	_ = os.Setenv("DAGU_DATA_DIR", cfg.DataDir)
	_ = os.Setenv("DAGU_SUSPEND_FLAGS_DIR", cfg.SuspendFlagsDir)
	_ = os.Setenv("DAGU_ADMIN_LOG_DIR", cfg.AdminLogsDir)

	return Setup{
		Config: cfg,
		Logger: NewLogger(),

		homeDir: tmpDir,
	}
}

func SetupForDir(t *testing.T, dir string) Setup {
	lock.Lock()
	defer lock.Unlock()

	tmpDir := util.MustTempDir("dagu_test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	viper.AddConfigPath(dir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	cfg, err := config.Load()
	require.NoError(t, err)

	return Setup{
		Config: cfg,
		Logger: NewLogger(),

		homeDir: tmpDir,
	}
}

func NewLogger() logger.Logger {
	return logger.NewLogger(logger.NewLoggerArgs{
		LogLevel:  "debug",
		LogFormat: "text",
	})
}
