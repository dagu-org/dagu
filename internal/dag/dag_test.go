package dag

import (
	"os"
	"path"
	"testing"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/stretchr/testify/require"
)

var (
	testdataDir = path.Join(utils.MustGetwd(), "testdata")
	testHomeDir = path.Join(utils.MustGetwd(), "testdata/home")
)

func TestMain(m *testing.M) {
	changeHomeDir(testHomeDir)
	code := m.Run()
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	_ = os.Setenv("HOME", homeDir)
	_ = config.LoadConfig(homeDir)
}

func TestToString(t *testing.T) {
	l := &Loader{}

	d, err := l.Load(path.Join(testdataDir, "default.yaml"), "")
	require.NoError(t, err)

	ret := d.String()
	require.Contains(t, ret, "Name: default")
}

func TestReadingFile(t *testing.T) {
	tmpDir := utils.MustTempDir("read-config-test")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	tmpFile := path.Join(tmpDir, "DAG.yaml")
	input := `
steps:
  - name: step 1
    command: echo test
`
	err := os.WriteFile(tmpFile, []byte(input), 0644)
	require.NoError(t, err)

	ret, err := ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, input, ret)
}
