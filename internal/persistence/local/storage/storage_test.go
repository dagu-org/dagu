package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/utils"
)

func TestStorage(t *testing.T) {
	tmpDir := utils.MustTempDir("test-storage")
	defer os.RemoveAll(tmpDir)

	s := NewStorage(tmpDir)

	f := "test.flag"
	exist := s.Exists(f)
	require.False(t, exist)

	err := s.Create(f)
	require.NoError(t, err)

	exist = s.Exists(f)
	require.True(t, exist)

	err = s.Delete(f)
	require.NoError(t, err)

	exist = s.Exists(f)
	require.False(t, exist)
}
