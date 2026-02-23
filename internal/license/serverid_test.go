package license

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrCreateServerID(t *testing.T) {
	t.Parallel()

	t.Run("creates new UUID when directory does not exist", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "new-license-dir")
		id, err := GetOrCreateServerID(dir)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		_, parseErr := uuid.Parse(id)
		assert.NoError(t, parseErr, "returned ID should be a valid UUID")
	})

	t.Run("returns same ID on subsequent calls", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		first, err := GetOrCreateServerID(dir)
		require.NoError(t, err)

		second, err := GetOrCreateServerID(dir)
		require.NoError(t, err)

		assert.Equal(t, first, second)
	})

	t.Run("reads pre-existing server ID file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		known := "01900000-0000-7000-8000-000000000042"
		idPath := filepath.Join(dir, "server_id")
		require.NoError(t, os.WriteFile(idPath, []byte(known), 0600))

		id, err := GetOrCreateServerID(dir)
		require.NoError(t, err)
		assert.Equal(t, known, id)
	})

	t.Run("trims leading and trailing whitespace from existing file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		raw := " some-id \n"
		idPath := filepath.Join(dir, "server_id")
		require.NoError(t, os.WriteFile(idPath, []byte(raw), 0600))

		id, err := GetOrCreateServerID(dir)
		require.NoError(t, err)
		assert.Equal(t, "some-id", id)
	})

	t.Run("generates new UUID when server_id file is empty", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		idPath := filepath.Join(dir, "server_id")
		require.NoError(t, os.WriteFile(idPath, []byte(""), 0600))

		id, err := GetOrCreateServerID(dir)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		_, parseErr := uuid.Parse(id)
		assert.NoError(t, parseErr, "generated ID should be a valid UUID")
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "a", "b", "c", "d")
		id, err := GetOrCreateServerID(dir)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		info, statErr := os.Stat(dir)
		require.NoError(t, statErr)
		assert.True(t, info.IsDir())
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		t.Parallel()

		// /dev/null is a file, so using it as a directory will fail on MkdirAll.
		dir := "/dev/null/impossible"
		_, err := GetOrCreateServerID(dir)
		assert.Error(t, err)
	})

	t.Run("returned string is a valid UUID", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		id, err := GetOrCreateServerID(dir)
		require.NoError(t, err)

		parsed, parseErr := uuid.Parse(id)
		require.NoError(t, parseErr, "ID must be parseable as a UUID")
		assert.Equal(t, id, parsed.String(), "round-trip through uuid.Parse should be stable")
	})

	t.Run("created server_id file has 0600 permissions", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		_, err := GetOrCreateServerID(dir)
		require.NoError(t, err)

		idPath := filepath.Join(dir, "server_id")
		info, statErr := os.Stat(idPath)
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}
