package filens

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates store with valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := New(tmpDir)
		require.NoError(t, err)
		require.NotNil(t, store)
	})

	t.Run("creates directory if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		newDir := filepath.Join(tmpDir, "namespaces")
		store, err := New(newDir)
		require.NoError(t, err)
		require.NotNil(t, store)

		info, err := os.Stat(newDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("fails with empty directory", func(t *testing.T) {
		store, err := New("")
		require.Error(t, err)
		require.Nil(t, store)
	})
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError error
	}{
		{"valid simple", "team", nil},
		{"valid with hyphen", "team-alpha", nil},
		{"valid with underscore", "team_alpha", nil},
		{"valid with numbers", "team1", nil},
		{"valid single char", "a", nil},
		{"valid long name", "abcdefghijklmnopqrstuvwxyz0123456789-_abcdefghijklmnopqrstuv", nil},
		{"empty", "", ErrInvalidNamespaceName},
		{"starts with number", "1team", ErrInvalidNamespaceName},
		{"starts with hyphen", "-team", ErrInvalidNamespaceName},
		{"ends with hyphen", "team-", ErrInvalidNamespaceName},
		{"ends with underscore", "team_", ErrInvalidNamespaceName},
		{"uppercase", "Team", ErrInvalidNamespaceName},
		{"spaces", "team alpha", ErrInvalidNamespaceName},
		{"reserved system", "system", ErrReservedNamespaceName},
		{"reserved admin", "admin", ErrReservedNamespaceName},
		{"reserved api", "api", ErrReservedNamespaceName},
		{"reserved internal", "internal", ErrReservedNamespaceName},
		{"reserved global", "global", ErrReservedNamespaceName},
		{"too long", "abcdefghijklmnopqrstuvwxyz0123456789-_abcdefghijklmnopqrstuvwxyz", ErrInvalidNamespaceName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if tt.wantError != nil {
				assert.ErrorIs(t, err, tt.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStore_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates namespace successfully", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{
			Name:        "team-alpha",
			DisplayName: "Team Alpha",
			Description: "Alpha team namespace",
		}

		err := store.Create(ctx, ns)
		require.NoError(t, err)
		assert.NotEmpty(t, ns.ID)
		assert.False(t, ns.CreatedAt.IsZero())
		assert.False(t, ns.UpdatedAt.IsZero())
	})

	t.Run("fails with nil namespace", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Create(ctx, nil)
		require.Error(t, err)
	})

	t.Run("fails with invalid name", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{Name: "Invalid Name"}
		err := store.Create(ctx, ns)
		require.ErrorIs(t, err, ErrInvalidNamespaceName)
	})

	t.Run("fails with reserved name", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{Name: "system"}
		err := store.Create(ctx, ns)
		require.ErrorIs(t, err, ErrReservedNamespaceName)
	})

	t.Run("fails with duplicate name", func(t *testing.T) {
		store := newTestStore(t)

		ns1 := &Namespace{Name: "team-alpha"}
		err := store.Create(ctx, ns1)
		require.NoError(t, err)

		ns2 := &Namespace{Name: "team-alpha"}
		err = store.Create(ctx, ns2)
		require.ErrorIs(t, err, ErrNamespaceAlreadyExists)
	})
}

func TestStore_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieves namespace by ID", func(t *testing.T) {
		store := newTestStore(t)

		created := &Namespace{
			Name:        "team-alpha",
			DisplayName: "Team Alpha",
		}
		require.NoError(t, store.Create(ctx, created))

		retrieved, err := store.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.Name, retrieved.Name)
		assert.Equal(t, created.DisplayName, retrieved.DisplayName)
	})

	t.Run("fails with empty ID", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.GetByID(ctx, "")
		require.ErrorIs(t, err, ErrInvalidNamespaceID)
	})

	t.Run("fails with non-existent ID", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.GetByID(ctx, "non-existent-id")
		require.ErrorIs(t, err, ErrNamespaceNotFound)
	})
}

func TestStore_GetByName(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieves namespace by name", func(t *testing.T) {
		store := newTestStore(t)

		created := &Namespace{
			Name:        "team-alpha",
			DisplayName: "Team Alpha",
		}
		require.NoError(t, store.Create(ctx, created))

		retrieved, err := store.GetByName(ctx, "team-alpha")
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.Name, retrieved.Name)
	})

	t.Run("fails with empty name", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.GetByName(ctx, "")
		require.ErrorIs(t, err, ErrInvalidNamespaceName)
	})

	t.Run("fails with non-existent name", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.GetByName(ctx, "non-existent")
		require.ErrorIs(t, err, ErrNamespaceNotFound)
	})
}

func TestStore_List(t *testing.T) {
	ctx := context.Background()

	t.Run("lists all namespaces", func(t *testing.T) {
		store := newTestStore(t)

		ns1 := &Namespace{Name: "team-alpha"}
		ns2 := &Namespace{Name: "team-beta"}
		ns3 := &Namespace{Name: "team-gamma"}

		require.NoError(t, store.Create(ctx, ns1))
		require.NoError(t, store.Create(ctx, ns2))
		require.NoError(t, store.Create(ctx, ns3))

		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 3)
	})

	t.Run("returns empty list when no namespaces", func(t *testing.T) {
		store := newTestStore(t)

		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
	})
}

func TestStore_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("updates namespace successfully", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{
			Name:        "team-alpha",
			DisplayName: "Team Alpha",
			Description: "Original description",
		}
		require.NoError(t, store.Create(ctx, ns))

		ns.DisplayName = "Updated Team Alpha"
		ns.Description = "Updated description"
		err := store.Update(ctx, ns)
		require.NoError(t, err)

		retrieved, err := store.GetByID(ctx, ns.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Team Alpha", retrieved.DisplayName)
		assert.Equal(t, "Updated description", retrieved.Description)
		// Name should not be changed by Update
		assert.Equal(t, "team-alpha", retrieved.Name)
	})

	t.Run("fails with nil namespace", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Update(ctx, nil)
		require.Error(t, err)
	})

	t.Run("fails with non-existent namespace", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{ID: "non-existent-id", Name: "test"}
		err := store.Update(ctx, ns)
		require.ErrorIs(t, err, ErrNamespaceNotFound)
	})
}

func TestStore_Rename(t *testing.T) {
	ctx := context.Background()

	t.Run("renames namespace successfully", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{Name: "team-alpha"}
		require.NoError(t, store.Create(ctx, ns))

		err := store.Rename(ctx, ns.ID, "team-omega")
		require.NoError(t, err)

		// Old name should not work
		_, err = store.GetByName(ctx, "team-alpha")
		require.ErrorIs(t, err, ErrNamespaceNotFound)

		// New name should work
		retrieved, err := store.GetByName(ctx, "team-omega")
		require.NoError(t, err)
		assert.Equal(t, ns.ID, retrieved.ID)
		assert.Equal(t, "team-omega", retrieved.Name)
	})

	t.Run("fails with empty ID", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Rename(ctx, "", "new-name")
		require.ErrorIs(t, err, ErrInvalidNamespaceID)
	})

	t.Run("fails with invalid new name", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{Name: "team-alpha"}
		require.NoError(t, store.Create(ctx, ns))

		err := store.Rename(ctx, ns.ID, "Invalid Name")
		require.ErrorIs(t, err, ErrInvalidNamespaceName)
	})

	t.Run("fails when new name already exists", func(t *testing.T) {
		store := newTestStore(t)

		ns1 := &Namespace{Name: "team-alpha"}
		ns2 := &Namespace{Name: "team-beta"}
		require.NoError(t, store.Create(ctx, ns1))
		require.NoError(t, store.Create(ctx, ns2))

		err := store.Rename(ctx, ns1.ID, "team-beta")
		require.ErrorIs(t, err, ErrNamespaceAlreadyExists)
	})

	t.Run("fails with non-existent namespace", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Rename(ctx, "non-existent-id", "new-name")
		require.ErrorIs(t, err, ErrNamespaceNotFound)
	})
}

func TestStore_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes namespace successfully", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{Name: "team-alpha"}
		require.NoError(t, store.Create(ctx, ns))

		err := store.Delete(ctx, ns.ID)
		require.NoError(t, err)

		_, err = store.GetByID(ctx, ns.ID)
		require.ErrorIs(t, err, ErrNamespaceNotFound)

		_, err = store.GetByName(ctx, "team-alpha")
		require.ErrorIs(t, err, ErrNamespaceNotFound)
	})

	t.Run("fails with empty ID", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Delete(ctx, "")
		require.ErrorIs(t, err, ErrInvalidNamespaceID)
	})

	t.Run("fails with non-existent namespace", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Delete(ctx, "non-existent-id")
		require.ErrorIs(t, err, ErrNamespaceNotFound)
	})
}

func TestStore_Count(t *testing.T) {
	ctx := context.Background()

	t.Run("counts namespaces correctly", func(t *testing.T) {
		store := newTestStore(t)

		count, err := store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

		require.NoError(t, store.Create(ctx, &Namespace{Name: "ns1"}))
		require.NoError(t, store.Create(ctx, &Namespace{Name: "ns2"}))
		require.NoError(t, store.Create(ctx, &Namespace{Name: "ns3"}))

		count, err = store.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})
}

func TestStore_Exists(t *testing.T) {
	ctx := context.Background()

	t.Run("returns true for existing namespace", func(t *testing.T) {
		store := newTestStore(t)

		ns := &Namespace{Name: "team-alpha"}
		require.NoError(t, store.Create(ctx, ns))

		assert.True(t, store.Exists(ctx, "team-alpha"))
	})

	t.Run("returns false for non-existing namespace", func(t *testing.T) {
		store := newTestStore(t)

		assert.False(t, store.Exists(ctx, "non-existent"))
	})
}

func TestStore_EnsureDefault(t *testing.T) {
	ctx := context.Background()

	t.Run("creates default namespace if not exists", func(t *testing.T) {
		store := newTestStore(t)

		ns, err := store.EnsureDefault(ctx, "system")
		require.NoError(t, err)
		require.NotNil(t, ns)
		assert.Equal(t, DefaultNamespaceName, ns.Name)
		assert.Equal(t, "Default", ns.DisplayName)
		assert.Equal(t, "system", ns.CreatedBy)
	})

	t.Run("returns existing default namespace", func(t *testing.T) {
		store := newTestStore(t)

		// Create default
		ns1, err := store.EnsureDefault(ctx, "system")
		require.NoError(t, err)

		// Ensure again should return same
		ns2, err := store.EnsureDefault(ctx, "other-user")
		require.NoError(t, err)
		assert.Equal(t, ns1.ID, ns2.ID)
		assert.Equal(t, "system", ns2.CreatedBy) // Original creator
	})
}

func TestStore_IndexPersistence(t *testing.T) {
	ctx := context.Background()

	t.Run("rebuilds index from disk", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create namespaces with first store
		store1, err := New(tmpDir)
		require.NoError(t, err)

		ns := &Namespace{Name: "team-alpha", DisplayName: "Team Alpha"}
		require.NoError(t, store1.Create(ctx, ns))

		// Create second store pointing to same directory
		store2, err := New(tmpDir)
		require.NoError(t, err)

		// Should be able to find namespace by name (proves index rebuilt)
		retrieved, err := store2.GetByName(ctx, "team-alpha")
		require.NoError(t, err)
		assert.Equal(t, ns.ID, retrieved.ID)
	})
}

// newTestStore creates a store with a temporary directory for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	require.NoError(t, err)
	return store
}
