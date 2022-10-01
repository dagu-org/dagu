package settings

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/utils"
)

var testHomeDir string

func TestMain(m *testing.M) {
	testHomeDir = utils.MustTempDir("settings_test")
	ChangeHomeDir(testHomeDir)
	exitCode := m.Run()
	os.RemoveAll(testHomeDir)
	os.Exit(exitCode)
}

func TestReadSetting(t *testing.T) {
	load()

	// read default settings
	for _, test := range []struct {
		Name string
		Want string
	}{
		{
			Name: SETTING__DATA_DIR,
			Want: path.Join(testHomeDir, ".dagu/data"),
		},
		{
			Name: SETTING__LOGS_DIR,
			Want: path.Join(testHomeDir, ".dagu/logs"),
		},
	} {
		val, err := Get(test.Name)
		require.NoError(t, err)
		require.Equal(t, val, test.Want)
	}

	// read from environment variables
	_ = os.Setenv(SETTING__HOME, "/tmp/dagu/")
	for _, test := range []struct {
		Name string
		Want string
	}{
		{
			Name: SETTING__DATA_DIR,
			Want: "/tmp/dagu/data",
		},
		{
			Name: SETTING__LOGS_DIR,
			Want: "/tmp/dagu/logs",
		},
	} {
		_ = os.Setenv(test.Name, test.Want)
		load()

		val, err := Get(test.Name)
		require.NoError(t, err)
		require.Equal(t, test.Want, val)

		val = MustGet(test.Name)
		require.Equal(t, test.Want, val)
	}

	_, err := Get("Invalid_Name")
	require.Equal(t, ErrSettingNotFound, err)

	// check $DAGU_HOME
	os.Setenv("DAGU_HOME", "/home/dagu")
	load()

	require.Equal(t, MustGet(SETTING__HOME), "/home/dagu")
	require.Equal(t, MustGet(SETTING__ADMIN_CONFIG), "/home/dagu/admin.yaml")
}
