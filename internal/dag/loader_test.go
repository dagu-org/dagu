package dag

import (
	"path"
	"strings"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadingFile(t *testing.T) {
	l := &Loader{}
	f := path.Join(testdataDir, "loader_test.yaml")
	d, err := l.Load(f, "")
	require.NoError(t, err)
	require.Equal(t, f, d.Location)

	// without .yaml
	d, err = l.Load(path.Join(testdataDir, "loader_test"), "")
	require.NoError(t, err)
	require.Equal(t, f, d.Location)
}

func TestLoaingErrors(t *testing.T) {

	tests := []struct {
		file          string
		expectedError string
	}{
		{
			file:          path.Join(testdataDir, "not_existing_file.yaml"),
			expectedError: "no such file or directory",
		},
		{
			file:          path.Join(testdataDir, "err_decode.yaml"),
			expectedError: "has invalid keys: invalidkey",
		},
		{
			file:          path.Join(testdataDir, "err_parse.yaml"),
			expectedError: "cannot unmarshal",
		},
	}

	for i, tt := range tests {
		l := &Loader{}
		_, err := l.Load(tt.file, "")
		require.Error(t, err)

		if !strings.Contains(err.Error(), tt.expectedError) {
			t.Errorf("test %d: expected error %q, got %q", i, tt.expectedError, err.Error())
		}
	}
}

func TestLoadingHeadlineOnly(t *testing.T) {
	l := &Loader{}

	d, err := l.LoadMetadata(path.Join(testdataDir, "default.yaml"))
	require.NoError(t, err)

	require.Equal(t, d.Name, "default")
	require.True(t, len(d.Steps) == 0)
}

func TestCloning(t *testing.T) {
	l := &Loader{}

	d, err := l.Load(path.Join(testdataDir, "default.yaml"), "")
	require.NoError(t, err)

	cloned := d.Clone()
	require.Equal(t, d, cloned)
}

func TestLoadingBaseConfig(t *testing.T) {
	l := &Loader{}
	d, err := l.loadBaseConfig(config.Get().BaseConfig, &BuildDAGOptions{})
	require.NotNil(t, d)
	require.NoError(t, err)
}

func TestLoadingDeafultValues(t *testing.T) {
	l := &Loader{}
	d, err := l.Load(path.Join(testdataDir, "default.yaml"), "")
	require.NoError(t, err)

	require.Equal(t, time.Second*60, d.MaxCleanUpTime)
	require.Equal(t, 30, d.HistRetentionDays)
}

func TestLoadingFromMemory(t *testing.T) {
	dat := `
name: test DAG
steps:
  - name: "1"
    command: "true"
`
	l := &Loader{}
	ret, err := l.LoadData([]byte(dat))
	require.NoError(t, err)
	require.Equal(t, ret.Name, "test DAG")

	step := ret.Steps[0]
	require.Equal(t, step.Name, "1")
	require.Equal(t, step.Command, "true")

	// error
	dat = `invalidyaml`
	_, err = l.LoadData([]byte(dat))
	require.Error(t, err)

	// error
	dat = `invalidkey: test DAG`
	_, err = l.LoadData([]byte(dat))
	require.Error(t, err)
}
