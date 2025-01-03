package local

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"

	"github.com/stretchr/testify/require"
)

func TestFlagStore(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	flagStore := NewFlagStore(storage.NewStorage(tmpDir))

	require.False(t, flagStore.IsSuspended("test"))

	err := flagStore.ToggleSuspend("test", true)
	require.NoError(t, err)

	require.True(t, flagStore.IsSuspended("test"))
}
