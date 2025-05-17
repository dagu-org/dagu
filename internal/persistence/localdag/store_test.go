package localdag

import (
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/fileutil"

	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	dr := New(tmpDir)
	ctx := context.Background()

	require.False(t, dr.IsSuspended(ctx, "test"))

	err := dr.ToggleSuspend(ctx, "test", true)
	require.NoError(t, err)

	require.True(t, dr.IsSuspended(ctx, "test"))
}
