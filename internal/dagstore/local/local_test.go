package local

import (
	"context"
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
	ctx := context.Background()

	require.False(t, dagStore.IsSuspended(ctx, "test"))

	err := dagStore.ToggleSuspend(ctx, "test", true)
	require.NoError(t, err)

	require.True(t, dagStore.IsSuspended(ctx, "test"))
}
