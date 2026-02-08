package filenamespace_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filenamespace"
)

func newTestStore(t *testing.T) *filenamespace.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := filenamespace.New(dir)
	require.NoError(t, err)
	return s
}

func TestStore_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("basic create", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		ns, err := s.Create(ctx, exec.CreateNamespaceOptions{
			Name:        "team-alpha",
			Description: "Team Alpha's namespace",
		})
		require.NoError(t, err)
		require.Equal(t, "team-alpha", ns.Name)
		require.Len(t, ns.ShortID, 4)
		require.False(t, ns.CreatedAt.IsZero())
		require.Equal(t, "Team Alpha's namespace", ns.Description)
	})

	t.Run("default namespace gets fixed short ID", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		// The "default" namespace is auto-created with the well-known short ID.
		ns, err := s.Get(ctx, "default")
		require.NoError(t, err)
		require.Equal(t, filenamespace.DefaultShortID, ns.ShortID)
	})

	t.Run("with defaults and git sync", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		ns, err := s.Create(ctx, exec.CreateNamespaceOptions{
			Name: "team-beta",
			Defaults: exec.NamespaceDefaults{
				Queue:      "beta-queue",
				WorkingDir: "/work/beta",
			},
			GitSync: exec.NamespaceGitSync{
				RemoteURL: "git@github.com:org/repo.git",
				Branch:    "main",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "beta-queue", ns.Defaults.Queue)
		require.Equal(t, "/work/beta", ns.Defaults.WorkingDir)
		require.Equal(t, "git@github.com:org/repo.git", ns.GitSync.RemoteURL)
	})

	t.Run("duplicate name returns error", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		_, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: "team-alpha"})
		require.NoError(t, err)

		_, err = s.Create(ctx, exec.CreateNamespaceOptions{Name: "team-alpha"})
		require.ErrorIs(t, err, exec.ErrNamespaceAlreadyExists)
	})
}

func TestStore_Get(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("get existing namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		created, err := s.Create(ctx, exec.CreateNamespaceOptions{
			Name:        "team-alpha",
			Description: "Alpha",
		})
		require.NoError(t, err)

		got, err := s.Get(ctx, "team-alpha")
		require.NoError(t, err)
		require.Equal(t, created.Name, got.Name)
		require.Equal(t, created.ShortID, got.ShortID)
		require.Equal(t, created.Description, got.Description)
	})

	t.Run("get non-existent namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		_, err := s.Get(ctx, "does-not-exist")
		require.ErrorIs(t, err, exec.ErrNamespaceNotFound)
	})
}

func TestStore_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("delete existing namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		_, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: "team-alpha"})
		require.NoError(t, err)

		err = s.Delete(ctx, "team-alpha")
		require.NoError(t, err)

		_, err = s.Get(ctx, "team-alpha")
		require.ErrorIs(t, err, exec.ErrNamespaceNotFound)
	})

	t.Run("delete non-existent namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		err := s.Delete(ctx, "does-not-exist")
		require.ErrorIs(t, err, exec.ErrNamespaceNotFound)
	})
}

func TestStore_List(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("list new store has default namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		list, err := s.List(ctx)
		require.NoError(t, err)
		// The "default" namespace is auto-created on store initialization.
		require.Len(t, list, 1)
		require.Equal(t, "default", list[0].Name)
	})

	t.Run("list multiple namespaces", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		_, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: "alpha"})
		require.NoError(t, err)
		_, err = s.Create(ctx, exec.CreateNamespaceOptions{Name: "beta"})
		require.NoError(t, err)

		list, err := s.List(ctx)
		require.NoError(t, err)
		// 3 = default (auto-created) + alpha + beta
		require.Len(t, list, 3)

		names := make(map[string]bool)
		for _, ns := range list {
			names[ns.Name] = true
		}
		require.True(t, names["default"])
		require.True(t, names["alpha"])
		require.True(t, names["beta"])
	})
}

func TestStore_Resolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("resolve existing namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		ns, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: "team-alpha"})
		require.NoError(t, err)

		shortID, err := s.Resolve(ctx, "team-alpha")
		require.NoError(t, err)
		require.Equal(t, ns.ShortID, shortID)
	})

	t.Run("resolve non-existent namespace", func(t *testing.T) {
		t.Parallel()
		s := newTestStore(t)

		_, err := s.Resolve(ctx, "does-not-exist")
		require.ErrorIs(t, err, exec.ErrNamespaceNotFound)
	})
}

func TestStore_NameValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	validNames := []string{
		"a",
		"a1",
		"team-alpha",
		"my-namespace-123",
		"x",
		"abc",
		"a-b",
		"123",
	}

	for _, name := range validNames {
		t.Run("valid: "+name, func(t *testing.T) {
			t.Parallel()
			s := newTestStore(t)
			_, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: name})
			require.NoError(t, err)
		})
	}

	invalidNames := []string{
		"",                      // empty
		"-team",                 // starts with hyphen
		"team-",                 // ends with hyphen
		"Team-Alpha",            // uppercase
		"team_alpha",            // underscore
		"team.alpha",            // dot
		"team alpha",            // space
		strings.Repeat("a", 64), // too long
	}

	for _, name := range invalidNames {
		displayName := name
		if displayName == "" {
			displayName = "(empty)"
		}
		if len(displayName) > 20 {
			displayName = displayName[:20] + "..."
		}
		t.Run("invalid: "+displayName, func(t *testing.T) {
			t.Parallel()
			s := newTestStore(t)
			_, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: name})
			require.Error(t, err)
		})
	}
}

func TestStore_Persistence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()

	// Create a namespace with the first store instance.
	s1, err := filenamespace.New(dir)
	require.NoError(t, err)

	_, err = s1.Create(ctx, exec.CreateNamespaceOptions{
		Name:        "team-alpha",
		Description: "persisted",
	})
	require.NoError(t, err)

	// Create a second store instance from the same directory.
	s2, err := filenamespace.New(dir)
	require.NoError(t, err)

	ns, err := s2.Get(ctx, "team-alpha")
	require.NoError(t, err)
	assert.Equal(t, "team-alpha", ns.Name)
	assert.Equal(t, "persisted", ns.Description)
}

func TestStore_CollisionDetection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// We can't easily produce a real SHA256 collision with 4-char prefixes
	// in a unit test, but we verify the mechanism works by testing that
	// two different names with different short IDs don't collide.
	s := newTestStore(t)

	_, err := s.Create(ctx, exec.CreateNamespaceOptions{Name: "alpha"})
	require.NoError(t, err)

	_, err = s.Create(ctx, exec.CreateNamespaceOptions{Name: "beta"})
	require.NoError(t, err)

	// All should exist without collision: default (auto-created) + alpha + beta.
	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
}

func TestStore_DefaultNamespace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := newTestStore(t)

	// The "default" namespace is auto-created on store initialization.
	ns, err := s.Get(ctx, "default")
	require.NoError(t, err)
	require.Equal(t, "0000", ns.ShortID)

	shortID, err := s.Resolve(ctx, "default")
	require.NoError(t, err)
	require.Equal(t, "0000", shortID)

	// Creating "default" again should fail with ErrNamespaceAlreadyExists.
	_, err = s.Create(ctx, exec.CreateNamespaceOptions{Name: "default"})
	require.ErrorIs(t, err, exec.ErrNamespaceAlreadyExists)
}

func TestStore_WithFileCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cache := fileutil.NewCache[*exec.Namespace]("namespace_test", 100, 15*time.Minute)
	dir := t.TempDir()
	s, err := filenamespace.New(dir, filenamespace.WithFileCache(cache))
	require.NoError(t, err)

	// Create a namespace
	created, err := s.Create(ctx, exec.CreateNamespaceOptions{
		Name:        "cached-ns",
		Description: "original",
	})
	require.NoError(t, err)

	// First Get populates the cache
	got1, err := s.Get(ctx, "cached-ns")
	require.NoError(t, err)
	assert.Equal(t, "original", got1.Description)
	assert.Equal(t, created.ShortID, got1.ShortID)

	// Second Get should hit the cache (returns same data)
	got2, err := s.Get(ctx, "cached-ns")
	require.NoError(t, err)
	assert.Equal(t, "original", got2.Description)

	// Update invalidates the cache
	newDesc := "updated"
	updated, err := s.Update(ctx, "cached-ns", exec.UpdateNamespaceOptions{
		Description: &newDesc,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Description)

	// Get after update should see the new value (cache was invalidated)
	got3, err := s.Get(ctx, "cached-ns")
	require.NoError(t, err)
	assert.Equal(t, "updated", got3.Description)

	// Delete invalidates the cache
	err = s.Delete(ctx, "cached-ns")
	require.NoError(t, err)

	_, err = s.Get(ctx, "cached-ns")
	require.ErrorIs(t, err, exec.ErrNamespaceNotFound)
}
