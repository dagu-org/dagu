package fileupgradecheck

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/upgrade"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if store == nil {
		t.Fatal("New() returned nil")
	}

	upgradeDir := filepath.Join(tmpDir, upgradeDirName)
	info, err := os.Stat(upgradeDir)
	if err != nil {
		t.Fatalf("upgrade directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("upgrade path should be a directory")
	}
}

func TestNew_EmptyDir(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Error("New() should error on empty dataDir")
	}
}

func TestLoadNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	cache, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cache != nil {
		t.Error("Load() should return nil when no cache file exists")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	cache := &upgrade.UpgradeCheckCache{
		LastCheck:       time.Now().Truncate(time.Second),
		LatestVersion:   "v1.30.3",
		CurrentVersion:  "v1.30.0",
		UpdateAvailable: true,
	}

	if err := store.Save(cache); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil after save")
	}
	if loaded.LatestVersion != cache.LatestVersion {
		t.Errorf("Load().LatestVersion = %q, want %q", loaded.LatestVersion, cache.LatestVersion)
	}
	if loaded.CurrentVersion != cache.CurrentVersion {
		t.Errorf("Load().CurrentVersion = %q, want %q", loaded.CurrentVersion, cache.CurrentVersion)
	}
	if loaded.UpdateAvailable != cache.UpdateAvailable {
		t.Errorf("Load().UpdateAvailable = %v, want %v", loaded.UpdateAvailable, cache.UpdateAvailable)
	}
	if !loaded.LastCheck.Equal(cache.LastCheck) {
		t.Errorf("Load().LastCheck = %v, want %v", loaded.LastCheck, cache.LastCheck)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	cachePath := filepath.Join(tmpDir, upgradeDirName, cacheFileName)
	if err := os.WriteFile(cachePath, []byte("invalid json{"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cache, err := store.Load()
	if err != nil {
		t.Errorf("Load() should not error on invalid JSON: %v", err)
	}
	if cache != nil {
		t.Error("Load() should return nil for invalid JSON")
	}
}

func TestSaveAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	cache := &upgrade.UpgradeCheckCache{
		LastCheck:       time.Now().Truncate(time.Second),
		LatestVersion:   "v2.0.0",
		CurrentVersion:  "v1.0.0",
		UpdateAvailable: true,
	}

	if err := store.Save(cache); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cachePath := filepath.Join(tmpDir, upgradeDirName, cacheFileName)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Save() did not create cache file")
	}
}
