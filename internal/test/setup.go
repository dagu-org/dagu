package test

import (
	"os"
	"testing"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type Setup struct {
	Config *config.Config

	homeDir string
}

func (t Setup) Cleanup() {
	_ = os.RemoveAll(t.homeDir)
}

func (t Setup) DataStore() persistence.DataStores {
	return client.NewDataStores(&client.NewDataStoresArgs{
		DAGs:              t.Config.DAGs,
		DataDir:           t.Config.DataDir,
		SuspendFlagsDir:   t.Config.SuspendFlagsDir,
		LatestStatusToday: t.Config.LatestStatusToday,
	})
}

func (t Setup) Engine() engine.Engine {
	return engine.New(&engine.NewEngineArgs{
		DataStore:  t.DataStore(),
		Executable: t.Config.Executable,
		WorkDir:    t.Config.WorkDir,
	})

}

func SetupTest(t *testing.T) Setup {
	tmpDir := util.MustTempDir("dagu_test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	viper.AddConfigPath(config.ConfigDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	cfg, err := config.Load()
	require.NoError(t, err)

	return Setup{
		Config:  cfg,
		homeDir: tmpDir,
	}
}

func SetupForDir(t *testing.T, dir string) Setup {
	tmpDir := util.MustTempDir("dagu_test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	viper.AddConfigPath(dir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	cfg, err := config.Load()
	require.NoError(t, err)

	return Setup{
		Config:  cfg,
		homeDir: tmpDir,
	}
}
