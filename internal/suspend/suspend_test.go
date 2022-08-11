package suspend

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/utils"
)

func TestSuspendChecker(t *testing.T) {
	tmpDir := utils.MustTempDir("test-suspend-checker")
	defer os.RemoveAll(tmpDir)

	s := storage.NewStorage(tmpDir)

	sc := NewSuspendChecker(s)

	d := &dag.DAG{
		Name: "test",
	}

	suspend := sc.IsSuspended(d)
	require.False(t, suspend)

	err := sc.ToggleSuspend(d, true)
	require.NoError(t, err)

	suspend = sc.IsSuspended(d)
	require.True(t, suspend)

	err = sc.ToggleSuspend(d, false)
	require.NoError(t, err)

	suspend = sc.IsSuspended(d)
	require.False(t, suspend)
}
