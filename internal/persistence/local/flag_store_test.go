package local

import (
	"github.com/dagu-dev/dagu/internal/persistence/local/storage"
	"os"
	"testing"

	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestFlagStore(t *testing.T) {
	tmpDir := utils.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	fs := NewFlagStore(storage.NewStorage(tmpDir))

	require.False(t, fs.IsSuspended("test"))

	err := fs.ToggleSuspend("test", true)
	require.NoError(t, err)

	require.True(t, fs.IsSuspended("test"))
}
