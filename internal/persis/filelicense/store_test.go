package filelicense

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleActivationData returns a fully-populated ActivationData for use in tests.
func sampleActivationData() *license.ActivationData {
	return &license.ActivationData{
		Token:           "tok-abc123",
		HeartbeatSecret: "hb-secret-xyz",
		LicenseKey:      "LK-0000-1111-2222-3333",
		ServerID:        "srv-deadbeef",
	}
}

// TestSaveLoad_RoundTrip verifies that all fields survive a Save → Load cycle.
func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	original := sampleActivationData()
	require.NoError(t, s.Save(original))

	loaded, err := s.Load()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, original.Token, loaded.Token)
	assert.Equal(t, original.HeartbeatSecret, loaded.HeartbeatSecret)
	assert.Equal(t, original.LicenseKey, loaded.LicenseKey)
	assert.Equal(t, original.ServerID, loaded.ServerID)
}

// TestSave_Overwrite verifies that a second Save replaces the first;
// Load must return the most-recently-saved data.
func TestSave_Overwrite(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	first := &license.ActivationData{
		Token:           "first-token",
		HeartbeatSecret: "first-secret",
		LicenseKey:      "LK-FIRST",
		ServerID:        "srv-first",
	}
	require.NoError(t, s.Save(first))

	second := &license.ActivationData{
		Token:           "second-token",
		HeartbeatSecret: "second-secret",
		LicenseKey:      "LK-SECOND",
		ServerID:        "srv-second",
	}
	require.NoError(t, s.Save(second))

	loaded, err := s.Load()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, second.Token, loaded.Token)
	assert.Equal(t, second.HeartbeatSecret, loaded.HeartbeatSecret)
	assert.Equal(t, second.LicenseKey, loaded.LicenseKey)
	assert.Equal(t, second.ServerID, loaded.ServerID)
}

// TestSave_CreatesNestedDirectories verifies that Save creates the target
// directory (including intermediate directories) when it does not yet exist.
func TestSave_CreatesNestedDirectories(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "a", "b", "c", "license")

	s := New(dir)
	require.NoError(t, s.Save(sampleActivationData()))

	info, err := os.Stat(dir)
	require.NoError(t, err, "directory should have been created by Save")
	assert.True(t, info.IsDir())
}

// TestSave_FilePermissions verifies that the activation file is written with
// 0600 permissions (owner read/write only).
func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	require.NoError(t, s.Save(sampleActivationData()))

	path := filepath.Join(dir, activationFile)
	info, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(filePerm), info.Mode().Perm(),
		"activation file should have 0600 permissions")
}

// TestSave_DirectoryPermissions verifies that Save creates the directory with
// 0700 permissions (owner read/write/execute only).
func TestSave_DirectoryPermissions(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "license_dir")

	s := New(dir)
	require.NoError(t, s.Save(sampleActivationData()))

	info, err := os.Stat(dir)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(dirPerm), info.Mode().Perm(),
		"license directory should have 0700 permissions")
}

// TestSave_PrettyPrintedJSON verifies that the file contains indented JSON
// (i.e. json.MarshalIndent was used rather than json.Marshal).
func TestSave_PrettyPrintedJSON(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	require.NoError(t, s.Save(sampleActivationData()))

	raw, err := os.ReadFile(filepath.Join(dir, activationFile))
	require.NoError(t, err)

	content := string(raw)
	// Pretty-printed JSON will have newlines and leading spaces for indentation.
	assert.True(t, strings.Contains(content, "\n"), "JSON should be newline-separated")
	assert.True(t, strings.Contains(content, "  "), "JSON should be indented with spaces")

	// Also confirm the raw bytes are valid JSON.
	var check license.ActivationData
	require.NoError(t, json.Unmarshal(raw, &check))
}

// TestLoad_NoFile verifies that Load returns (nil, nil) when the activation
// file does not exist yet (fresh installation).
func TestLoad_NoFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	result, err := s.Load()
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestLoad_InvalidJSON verifies that Load returns an error whose message
// contains "unmarshal" when the file contains malformed JSON.
func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	path := filepath.Join(dir, activationFile)
	require.NoError(t, os.WriteFile(path, []byte("{not valid json!!"), filePerm))

	result, err := s.Load()
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, strings.Contains(err.Error(), "unmarshal"),
		"error message should mention unmarshal, got: %s", err.Error())
}

// TestLoad_ReadError verifies that Load returns an error (containing
// "read") when the activation file cannot be read because a plain file
// sits in the path where the directory should be.
func TestLoad_ReadError(t *testing.T) {
	base := t.TempDir()

	// Place a regular file where the license directory would be, so that
	// anything trying to read a file inside it will fail.
	blocker := filepath.Join(base, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("i am a file"), 0600))

	// The store's dir is the blocker file itself; the full path would be
	// <blocker>/activation.json which is impossible to read.
	s := New(blocker)

	result, err := s.Load()
	require.Error(t, err)
	assert.Nil(t, result)
}

// TestRemove_DeletesExistingFile verifies that Remove causes the activation
// file to disappear from disk; a subsequent Load returns (nil, nil).
func TestRemove_DeletesExistingFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	require.NoError(t, s.Save(sampleActivationData()))
	require.NoError(t, s.Remove())

	_, err := os.Stat(filepath.Join(dir, activationFile))
	assert.True(t, os.IsNotExist(err), "activation file should not exist after Remove")
}

// TestRemove_NoFile verifies that Remove does not return an error when the
// activation file does not exist (idempotent operation).
func TestRemove_NoFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	err := s.Remove()
	assert.NoError(t, err, "Remove should not error when file does not exist")
}

// TestRemove_ThenLoad verifies that after Remove the subsequent Load returns
// (nil, nil).
func TestRemove_ThenLoad(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	require.NoError(t, s.Save(sampleActivationData()))
	require.NoError(t, s.Remove())

	result, err := s.Load()
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestConcurrency_SaveAndLoad verifies that concurrent Save and Load
// operations do not trigger a data race (run with -race flag).
func TestConcurrency_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	const numWorkers = 10

	var wg sync.WaitGroup
	errCh := make(chan error, numWorkers*2)

	// Pre-populate so Load never races on an absent file returning nil.
	require.NoError(t, s.Save(sampleActivationData()))

	for i := range numWorkers {
		wg.Add(2)

		// Writer goroutine
		go func(idx int) {
			defer wg.Done()
			ad := &license.ActivationData{
				Token:           "tok-concurrent",
				HeartbeatSecret: "hb-concurrent",
				LicenseKey:      "LK-concurrent",
				ServerID:        "srv-concurrent",
			}
			if err := s.Save(ad); err != nil {
				errCh <- err
			}
		}(i)

		// Reader goroutine
		go func(idx int) {
			defer wg.Done()
			if _, err := s.Load(); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "no errors should occur during concurrent Save/Load")
}

// TestConcurrency_SaveAndRemove verifies that interleaved Save and Remove
// operations do not trigger a data race.
func TestConcurrency_SaveAndRemove(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	const numWorkers = 10

	var wg sync.WaitGroup
	errCh := make(chan error, numWorkers*2)

	for i := range numWorkers {
		wg.Add(2)

		// Saver
		go func(idx int) {
			defer wg.Done()
			ad := &license.ActivationData{
				Token:    "tok-race",
				ServerID: "srv-race",
			}
			if err := s.Save(ad); err != nil {
				errCh <- err
			}
		}(i)

		// Remover
		go func(idx int) {
			defer wg.Done()
			// Remove may legitimately fail-not-exist if a concurrent Remove won
			// the race; that is handled inside Remove already and returns nil.
			if err := s.Remove(); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "no errors should occur during concurrent Save/Remove")
}

// TestSaveLoad_OverwriteReturnsUpdatedData verifies that after saving once
// and then saving again with different values, Load reflects only the
// latest write.
func TestSaveLoad_OverwriteReturnsUpdatedData(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	original := sampleActivationData()
	require.NoError(t, s.Save(original))

	updated := &license.ActivationData{
		Token:           "updated-token",
		HeartbeatSecret: "updated-secret",
		LicenseKey:      "LK-UPDATED",
		ServerID:        "srv-updated",
	}
	require.NoError(t, s.Save(updated))

	loaded, err := s.Load()
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, updated.Token, loaded.Token, "Token should reflect updated value")
	assert.Equal(t, updated.HeartbeatSecret, loaded.HeartbeatSecret, "HeartbeatSecret should reflect updated value")
	assert.Equal(t, updated.LicenseKey, loaded.LicenseKey, "LicenseKey should reflect updated value")
	assert.Equal(t, updated.ServerID, loaded.ServerID, "ServerID should reflect updated value")

	// Ensure none of the old values remain
	assert.NotEqual(t, original.Token, loaded.Token)
	assert.NotEqual(t, original.LicenseKey, loaded.LicenseKey)
}
