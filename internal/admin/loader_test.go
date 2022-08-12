package admin

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/settings"
)

var testsConfig = path.Join(testsDir, "admin.yaml")

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	require.NoError(t, err)
	testConfig(t, cfg, &testWant{
		Host:    "127.0.0.1",
		Port:    "8080",
		DAGs:    settings.MustGet(settings.SETTING__ADMIN_DAGS_DIR),
		Command: "dagu",
	})
}

func TestHomeAdminConfig(t *testing.T) {
	l := &Loader{}

	_, err := l.LoadAdminConfig("no-existing-file.yaml")
	require.Equal(t, ErrConfigNotFound, err)

	cfg, err := l.LoadAdminConfig(settings.MustGet(settings.SETTING__ADMIN_CONFIG))
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "localhost",
		Port:    "8081",
		DAGs:    path.Join(testsDir, "/dagu/dags"),
		Command: path.Join(testsDir, "/dagu/bin/dagu"),
		WorkDir: path.Join(testsDir, "/dagu/dags"),
	})
}

func TestReadFileError(t *testing.T) {
	l := &Loader{}
	_, err := l.readFile("no-existing-file.yaml")
	require.Error(t, err)
}

func TestLoadAdminConfig(t *testing.T) {
	l := &Loader{}
	cfg, err := l.LoadAdminConfig(testsConfig)
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "localhost",
		Port:    "8082",
		DAGs:    path.Join(testsDir, "/dagu/dags"),
		Command: path.Join(testsDir, "/dagu/bin/dagu"),
		WorkDir: path.Join(testsDir, "/dagu/dags"),
	})
}

func testConfig(t *testing.T, cfg *Config, want *testWant) {
	t.Helper()
	require.Equal(t, want.Host, cfg.Host)
	require.Equal(t, want.Port, cfg.Port)
	require.Equal(t, want.DAGs, cfg.DAGs)
	require.Equal(t, want.WorkDir, cfg.WorkDir)
	require.Equal(t, want.Command, cfg.Command)
}

type testWant struct {
	Host    string
	Port    string
	DAGs    string
	Command string
	WorkDir string
}
