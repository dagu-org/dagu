package tokensecret_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/auth/tokensecret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileProvider(t *testing.T) {
	t.Run("auto-generates when file missing", func(t *testing.T) {
		dir := t.TempDir()
		authDir := filepath.Join(dir, "auth")

		p := tokensecret.NewFile(authDir)
		ts, err := p.Resolve(context.Background())
		require.NoError(t, err)
		assert.True(t, ts.IsValid())

		// Secret file should exist with correct permissions.
		path := filepath.Join(authDir, "token_secret")
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Generated secret should be 43 chars (32 bytes base64url, no padding).
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Len(t, string(data), 43)
	})

	t.Run("reads existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token_secret")
		require.NoError(t, os.WriteFile(path, []byte("existing-secret"), 0600))

		p := tokensecret.NewFile(dir)
		ts, err := p.Resolve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []byte("existing-secret"), ts.SigningKey())
	})

	t.Run("regenerates on empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token_secret")
		require.NoError(t, os.WriteFile(path, []byte(""), 0600))

		p := tokensecret.NewFile(dir)
		ts, err := p.Resolve(context.Background())
		require.NoError(t, err)
		assert.True(t, ts.IsValid())

		// File should now contain a generated secret.
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Len(t, string(data), 43)
	})

	t.Run("regenerates on whitespace-only file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token_secret")
		require.NoError(t, os.WriteFile(path, []byte("  \n\t  "), 0600))

		p := tokensecret.NewFile(dir)
		ts, err := p.Resolve(context.Background())
		require.NoError(t, err)
		assert.True(t, ts.IsValid())
	})

	t.Run("stable across calls", func(t *testing.T) {
		dir := t.TempDir()
		authDir := filepath.Join(dir, "auth")

		p := tokensecret.NewFile(authDir)

		ts1, err := p.Resolve(context.Background())
		require.NoError(t, err)

		ts2, err := p.Resolve(context.Background())
		require.NoError(t, err)

		assert.Equal(t, ts1.SigningKey(), ts2.SigningKey())
	})

	t.Run("directory permissions", func(t *testing.T) {
		dir := t.TempDir()
		authDir := filepath.Join(dir, "auth")

		p := tokensecret.NewFile(authDir)
		_, err := p.Resolve(context.Background())
		require.NoError(t, err)

		info, err := os.Stat(authDir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
	})
}
