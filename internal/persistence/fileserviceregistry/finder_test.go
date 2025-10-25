package fileserviceregistry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_Members_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	finder := newFinder(tmpDir, "test-service")

	ctx := context.Background()
	members, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members)
}

func TestResolver_Members_WithInstances(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create test instances
	instances := []instanceInfo{
		{
			ID:     "instance-1",
			Host:   "host1",
			Port:   8080,
			PID:    1234,
			Status: execution.ServiceStatusActive,
		},
		{
			ID:     "instance-2",
			Host:   "host2",
			Port:   8081,
			PID:    1235,
			Status: execution.ServiceStatusActive,
		},
	}

	for _, inst := range instances {
		filename := instanceFilePath(tmpDir, "test-service", inst.ID)
		err := writeInstanceFile(filename, &inst)
		require.NoError(t, err)
	}

	finder := newFinder(tmpDir, "test-service")
	ctx := context.Background()
	members, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Verify members data
	expectedHosts := map[string]int{
		"host1": 8080,
		"host2": 8081,
	}
	for _, member := range members {
		expectedPort, ok := expectedHosts[member.Host]
		assert.True(t, ok)
		assert.Equal(t, expectedPort, member.Port)
		assert.NotEmpty(t, member.ID)
	}
}

func TestResolver_Members_FiltersStaleInstances(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create fresh and stale instances
	freshInstance := instanceInfo{
		ID:     "fresh",
		Host:   "freshhost",
		Port:   8080,
		PID:    1234,
		Status: execution.ServiceStatusActive,
	}
	staleInstance := instanceInfo{
		ID:     "stale",
		Host:   "stalehost",
		Port:   8081,
		PID:    1235,
		Status: execution.ServiceStatusInactive,
	}

	filename := instanceFilePath(tmpDir, "test-service", freshInstance.ID)
	err = writeInstanceFile(filename, &freshInstance)
	require.NoError(t, err)
	filename = instanceFilePath(tmpDir, "test-service", staleInstance.ID)
	err = writeInstanceFile(filename, &staleInstance)
	require.NoError(t, err)

	// Make stale instance file old by changing its modification time
	staleFile := filepath.Join(tmpDir, "test-service", "stale.json")
	oldTime := time.Now().Add(-300 * time.Second)
	err = os.Chtimes(staleFile, oldTime, oldTime)
	require.NoError(t, err)

	finder := newFinder(tmpDir, "test-service")
	finder.staleTimeout = 30 * time.Second // 30 second timeout

	ctx := context.Background()
	members, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "freshhost", members[0].Host)
	assert.Equal(t, 8080, members[0].Port)

	// Verify stale file was removed
	assert.NoFileExists(t, staleFile)

	quarantineFile := staleFile + ".gc"
	require.Eventually(t, func() bool {
		_, err := os.Stat(quarantineFile)
		return os.IsNotExist(err)
	}, 5*time.Second, 100*time.Millisecond, "expected quarantined file to be cleaned up")
}

func TestResolver_Members_IgnoresInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create valid instance
	validInstance := instanceInfo{
		ID:     "valid",
		Host:   "validhost",
		Port:   8080,
		PID:    1234,
		Status: execution.ServiceStatusActive,
	}
	filename := instanceFilePath(tmpDir, "test-service", validInstance.ID)
	err = writeInstanceFile(filename, &validInstance)
	require.NoError(t, err)

	// Create invalid files
	invalidFiles := []struct {
		name    string
		content string
	}{
		{"invalid.json", "not json"},
		{"textfile.txt", "some text"},
		{"empty.json", ""},
	}

	for _, f := range invalidFiles {
		err := os.WriteFile(filepath.Join(serviceDir, f.name), []byte(f.content), 0644)
		require.NoError(t, err)
	}

	// Create a directory (should be ignored)
	err = os.Mkdir(filepath.Join(serviceDir, "subdir"), 0755)
	require.NoError(t, err)

	finder := newFinder(tmpDir, "test-service")
	ctx := context.Background()
	members, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "validhost", members[0].Host)
	assert.Equal(t, 8080, members[0].Port)
}

func TestResolver_Members_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create many instances
	for i := 0; i < 100; i++ {
		inst := instanceInfo{
			ID:     fmt.Sprintf("instance-%d", i),
			Host:   "host",
			Port:   8080 + i,
			PID:    1000 + i,
			Status: execution.ServiceStatusActive,
		}
		filename := instanceFilePath(tmpDir, "test-service", inst.ID)
		err := writeInstanceFile(filename, &inst)
		require.NoError(t, err)
	}

	finder := newFinder(tmpDir, "test-service")

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	members, err := finder.members(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	// Should have processed some members before cancellation
	assert.NotNil(t, members)
}

func TestResolver_RealWorldScenario(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate coordinator service registry
	coordinatorFinder := newFinder(tmpDir, execution.ServiceNameCoordinator)
	// Disable caching for this test to ensure we see updates immediately
	coordinatorFinder.cacheDuration = 0

	ctx := context.Background()

	// Initially no coordinators
	members, err := coordinatorFinder.members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members)

	// Coordinator comes online
	coordinator1 := instanceInfo{
		ID:     "coordinator-primary",
		Host:   "coord1.example.com",
		Port:   9090,
		PID:    2000,
		Status: execution.ServiceStatusActive,
	}
	filename := instanceFilePath(tmpDir, string(execution.ServiceNameCoordinator), coordinator1.ID)
	err = writeInstanceFile(filename, &coordinator1)
	require.NoError(t, err)

	// Now we should see the coordinator
	members, err = coordinatorFinder.members(ctx)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "coord1.example.com", members[0].Host)
	assert.Equal(t, 9090, members[0].Port)

	// Second coordinator joins
	coordinator2 := instanceInfo{
		ID:     "coordinator-secondary",
		Host:   "coord2.example.com",
		Port:   9090,
		Status: execution.ServiceStatusActive,
		PID:    2001,
	}
	filename = instanceFilePath(tmpDir, string(execution.ServiceNameCoordinator), coordinator2.ID)
	err = writeInstanceFile(filename, &coordinator2)
	require.NoError(t, err)

	// Should see both coordinators
	members, err = coordinatorFinder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 2)
}

func TestResolver_Members_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create initial instances
	instance1 := instanceInfo{
		ID:     "instance-1",
		Host:   "host1",
		Port:   8080,
		Status: execution.ServiceStatusActive,
		PID:    1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance1.ID)
	err = writeInstanceFile(filename, &instance1)
	require.NoError(t, err)

	finder := newFinder(tmpDir, "test-service")
	ctx := context.Background()

	// First call - should read from disk
	members1, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members1, 1)
	assert.Equal(t, "host1:8080", fmt.Sprintf("%s:%d", members1[0].Host, members1[0].Port))

	// Add another instance to disk
	instance2 := instanceInfo{
		ID:     "instance-2",
		Host:   "host2",
		Port:   8081,
		Status: execution.ServiceStatusActive,
		PID:    1235,
	}
	filename = instanceFilePath(tmpDir, "test-service", instance2.ID)
	err = writeInstanceFile(filename, &instance2)
	require.NoError(t, err)

	// Second call immediately - should return cached result
	members2, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members2, 1) // Still only 1 member from cache
	assert.Equal(t, "host1:8080", fmt.Sprintf("%s:%d", members2[0].Host, members2[0].Port))

	// Verify cache is being used by checking the same data
	assert.Equal(t, members1[0].ID, members2[0].ID)
	assert.Equal(t, fmt.Sprintf("%s:%d", members1[0].Host, members1[0].Port), fmt.Sprintf("%s:%d", members2[0].Host, members2[0].Port))
}

func TestResolver_Members_CacheExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create initial instance
	instance1 := instanceInfo{
		ID:     "instance-1",
		Host:   "host1",
		Port:   8080,
		Status: execution.ServiceStatusActive,
		PID:    1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance1.ID)
	err = writeInstanceFile(filename, &instance1)
	require.NoError(t, err)

	finder := newFinder(tmpDir, "test-service")
	// Set short cache duration for testing
	finder.cacheDuration = 100 * time.Millisecond
	ctx := context.Background()

	// First call - should read from disk
	members1, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members1, 1)

	// Add another instance
	instance2 := instanceInfo{
		ID:     "instance-2",
		Host:   "host2",
		Port:   8081,
		Status: execution.ServiceStatusActive,
		PID:    1235,
	}
	filename = instanceFilePath(tmpDir, "test-service", instance2.ID)
	err = writeInstanceFile(filename, &instance2)
	require.NoError(t, err)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call - cache expired, should read from disk again
	members3, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members3, 2) // Now sees both instances
}

func TestResolver_Members_NoCacheForEmptyMembers(t *testing.T) {
	tmpDir := t.TempDir()
	finder := newFinder(tmpDir, "test-service")
	ctx := context.Background()

	// First call - no instances
	members1, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members1)

	// Create service directory and add instance
	serviceDir := filepath.Join(tmpDir, "test-service")
	err = os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	instance := instanceInfo{
		ID:     "instance-1",
		Host:   "host1",
		Port:   8080,
		Status: execution.ServiceStatusActive,
		PID:    1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance.ID)
	err = writeInstanceFile(filename, &instance)
	require.NoError(t, err)

	// Second call immediately - should NOT use cache (since it was empty)
	members2, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members2, 1) // Should see the new instance
	assert.Equal(t, "host1:8080", fmt.Sprintf("%s:%d", members2[0].Host, members2[0].Port))
}

func TestResolver_Members_CacheConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create instances
	for i := 0; i < 5; i++ {
		inst := instanceInfo{
			ID:     fmt.Sprintf("instance-%d", i),
			Host:   fmt.Sprintf("host%d", i),
			Port:   8080 + i,
			PID:    1234 + i,
			Status: execution.ServiceStatusActive,
		}
		filename := instanceFilePath(tmpDir, "test-service", inst.ID)
		err := writeInstanceFile(filename, &inst)
		require.NoError(t, err)
	}

	finder := newFinder(tmpDir, "test-service")
	ctx := context.Background()

	// Run concurrent reads
	const numGoroutines = 10
	results := make(chan []execution.HostInfo, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			members, err := finder.members(ctx)
			if err != nil {
				errors <- err
				return
			}
			results <- members
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errors:
			t.Fatalf("Unexpected error: %v", err)
		case members := <-results:
			assert.Len(t, members, 5)
		}
	}
}

func TestResolver_Members_CacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create initial instance
	instance1 := instanceInfo{
		ID:     "instance-1",
		Host:   "host1",
		Port:   8080,
		Status: execution.ServiceStatusActive,
		PID:    1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance1.ID)
	err = writeInstanceFile(filename, &instance1)
	require.NoError(t, err)

	finder := newFinder(tmpDir, "test-service")
	// Use longer cache for this test
	finder.cacheDuration = 5 * time.Second
	ctx := context.Background()

	// First call - populate cache
	members1, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members1, 1)

	// Remove the instance file
	err = os.Remove(filename)
	require.NoError(t, err)

	// Second call - should still return cached result
	members2, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Len(t, members2, 1)

	// Manually expire cache
	finder.mu.Lock()
	finder.cacheTime = time.Now().Add(-10 * time.Second)
	finder.mu.Unlock()

	// Third call - cache expired, should see no instances
	members3, err := finder.members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members3)
}
