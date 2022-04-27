package admin_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var (
	testsDir    = path.Join(utils.MustGetwd(), "../../tests/admin/")
	testsConfig = path.Join(testsDir, "admin.yaml")
)

func TestMain(m *testing.M) {
	os.Setenv("HOST", "localhost")
	settings.InitTest(testsDir)
	code := m.Run()
	os.Exit(code)
}

func TestDefaultConfig(t *testing.T) {
	cfg, err := admin.DefaultConfig()
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "127.0.0.1",
		Port:    "8000",
		DAGs:    path.Join(wd),
		Command: "dagu",
	})
}

func TestHomeAdminConfig(t *testing.T) {
	loader := admin.NewConfigLoader()
	cfg, err := loader.LoadAdminConfig("")
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "localhost",
		Port:    "8081",
		DAGs:    path.Join(testsDir, "/dagu/dags"),
		Command: path.Join(testsDir, "/dagu/bin/dagu"),
		WorkDir: path.Join(testsDir, "/dagu/dags"),
	})
}

func TestLoadAdminConfig(t *testing.T) {
	loader := admin.NewConfigLoader()
	cfg, err := loader.LoadAdminConfig(testsConfig)
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "localhost",
		Port:    "8082",
		DAGs:    path.Join(testsDir, "/dagu/dags"),
		Command: path.Join(testsDir, "/dagu/bin/dagu"),
		WorkDir: path.Join(testsDir, "/dagu/dags"),
	})
}

func testConfig(t *testing.T, cfg *admin.Config, want *testWant) {
	t.Helper()
	assert.Equal(t, want.Host, cfg.Host)
	assert.Equal(t, want.Port, cfg.Port)
	assert.Equal(t, want.DAGs, cfg.DAGs)
	assert.Equal(t, want.WorkDir, cfg.WorkDir)
	assert.Equal(t, want.Command, cfg.Command)
}

type testWant struct {
	Host    string
	Port    string
	DAGs    string
	Command string
	WorkDir string
}
