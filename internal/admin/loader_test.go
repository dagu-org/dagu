package admin_test

import (
	"jobctl/internal/admin"
	"jobctl/internal/settings"
	"jobctl/internal/utils"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	h, err := os.Hostname()
	require.NoError(t, err)
	testConfig(t, cfg, &testWant{
		Host:    h,
		Port:    "8000",
		Jobs:    path.Join(wd),
		Command: "jobctl",
	})
}

func TestHomeAdminConfig(t *testing.T) {
	loader := admin.NewConfigLoader()
	cfg, err := loader.LoadAdminConfig("")
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "localhost",
		Port:    "8081",
		Jobs:    path.Join(testsDir, "/jobctl/jobs"),
		Command: path.Join(testsDir, "/jobctl/bin/jobctl"),
		WorkDir: path.Join(testsDir, "/jobctl/jobs"),
	})
}

func TestLoadAdminConfig(t *testing.T) {
	loader := admin.NewConfigLoader()
	cfg, err := loader.LoadAdminConfig(testsConfig)
	require.NoError(t, err)

	testConfig(t, cfg, &testWant{
		Host:    "localhost",
		Port:    "8082",
		Jobs:    path.Join(testsDir, "/jobctl/jobs"),
		Command: path.Join(testsDir, "/jobctl/bin/jobctl"),
		WorkDir: path.Join(testsDir, "/jobctl/jobs"),
	})
}

func testConfig(t *testing.T, cfg *admin.Config, want *testWant) {
	t.Helper()
	assert.Equal(t, want.Host, cfg.Host)
	assert.Equal(t, want.Port, cfg.Port)
	assert.Equal(t, want.Jobs, cfg.Jobs)
	assert.Equal(t, want.WorkDir, cfg.WorkDir)
	assert.Equal(t, want.Command, cfg.Command)
}

type testWant struct {
	Host    string
	Port    string
	Jobs    string
	Command string
	WorkDir string
}
