package filestore

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/fileutil"

	"github.com/stretchr/testify/require"
)

func TestDAGStore(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	dagStore := New(tmpDir)

	require.False(t, dagStore.IsSuspended("test"))

	err := dagStore.ToggleSuspend("test", true)
	require.NoError(t, err)

	require.True(t, dagStore.IsSuspended("test"))
}
