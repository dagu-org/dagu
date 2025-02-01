package storage

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-storage")
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	f := "test.flag"
	exist := storage.Exists(f)
	require.False(t, exist)

	err := storage.Create(f)
	require.NoError(t, err)

	exist = storage.Exists(f)
	require.True(t, exist)

	err = storage.Delete(f)
	require.NoError(t, err)

	exist = storage.Exists(f)
	require.False(t, exist)
}
