package local

import (
	"os"
	"testing"

	"github.com/daguflow/dagu/internal/persistence/local/storage"

	"github.com/daguflow/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestFlagStore(t *testing.T) {
	tmpDir := util.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	flagStore := NewFlagStore(storage.NewStorage(tmpDir))

	require.False(t, flagStore.IsSuspended("test"))

	err := flagStore.ToggleSuspend("test", true)
	require.NoError(t, err)

	require.True(t, flagStore.IsSuspended("test"))
}
