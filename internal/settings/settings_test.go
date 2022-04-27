package settings

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yohamta/dagu/internal/utils"
)

var testHomeDir string

func TestMain(m *testing.M) {
	testHomeDir = utils.MustTempDir("settings_test")
	InitTest(testHomeDir)
	os.Exit(m.Run())
}

func TestReadSetting(t *testing.T) {
	load()

	// read default configs
	for _, test := range []struct {
		Name string
		Want string
	}{
		{
			Name: CONFIG__DATA_DIR,
			Want: path.Join(testHomeDir, ".dagu/data"),
		},
		{
			Name: CONFIG__LOGS_DIR,
			Want: path.Join(testHomeDir, ".dagu/logs"),
		},
	} {
		val, err := Get(test.Name)
		assert.NoError(t, err)
		assert.Equal(t, val, test.Want)
	}

	// read from env variables
	for _, test := range []struct {
		Name string
		Want string
	}{
		{
			Name: CONFIG__DATA_DIR,
			Want: "/home/dagu/data",
		},
		{
			Name: CONFIG__LOGS_DIR,
			Want: "/home/dagu/logs",
		},
	} {
		os.Setenv(test.Name, test.Want)
		load()

		val, err := Get(test.Name)
		assert.NoError(t, err)
		assert.Equal(t, val, test.Want)
	}
}
