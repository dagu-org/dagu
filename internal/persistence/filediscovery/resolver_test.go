package filediscovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_Members_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := newResolver(tmpDir, "test-service")

	ctx := context.Background()
	members, err := resolver.Members(ctx)
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
			ID:       "instance-1",
			HostPort: "host1:8080",
			PID:      1234,
		},
		{
			ID:       "instance-2",
			HostPort: "host2:8081",
			PID:      1235,
		},
	}

	for _, inst := range instances {
		filename := instanceFilePath(tmpDir, "test-service", inst.ID)
		err := writeInstanceFile(filename, &inst)
		require.NoError(t, err)
	}

	resolver := newResolver(tmpDir, "test-service")
	ctx := context.Background()
	members, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Verify members data
	expectedHosts := map[string]bool{
		"host1:8080": true,
		"host2:8081": true,
	}
	for _, member := range members {
		assert.True(t, expectedHosts[member.HostPort])
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
		ID:       "fresh",
		HostPort: "freshhost:8080",
		PID:      1234,
	}
	staleInstance := instanceInfo{
		ID:       "stale",
		HostPort: "stalehost:8081",
		PID:      1235,
	}

	filename := instanceFilePath(tmpDir, "test-service", freshInstance.ID)
	err = writeInstanceFile(filename, &freshInstance)
	require.NoError(t, err)
	filename = instanceFilePath(tmpDir, "test-service", staleInstance.ID)
	err = writeInstanceFile(filename, &staleInstance)
	require.NoError(t, err)

	// Make stale instance file old by changing its modification time
	staleFile := filepath.Join(tmpDir, "test-service", "stale.json")
	oldTime := time.Now().Add(-time.Minute)
	err = os.Chtimes(staleFile, oldTime, oldTime)
	require.NoError(t, err)

	resolver := newResolver(tmpDir, "test-service")
	resolver.staleTimeout = 30 * time.Second // 30 second timeout

	ctx := context.Background()
	members, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "freshhost:8080", members[0].HostPort)

	// Verify stale file was removed
	assert.NoFileExists(t, staleFile)
}

func TestResolver_Members_IgnoresInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create valid instance
	validInstance := instanceInfo{
		ID:       "valid",
		HostPort: "validhost:8080",
		PID:      1234,
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

	resolver := newResolver(tmpDir, "test-service")
	ctx := context.Background()
	members, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "validhost:8080", members[0].HostPort)
}

func TestResolver_Members_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create many instances
	for i := 0; i < 100; i++ {
		inst := instanceInfo{
			ID:       fmt.Sprintf("instance-%d", i),
			HostPort: fmt.Sprintf("host:%d", 8080+i),
			PID:      1000 + i,
		}
		filename := instanceFilePath(tmpDir, "test-service", inst.ID)
		err := writeInstanceFile(filename, &inst)
		require.NoError(t, err)
	}

	resolver := newResolver(tmpDir, "test-service")

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	members, err := resolver.Members(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	// Should have processed some members before cancellation
	assert.NotNil(t, members)
}

func TestResolver_RemovesStaleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create stale instance
	staleInstance := instanceInfo{
		ID:       "stale-to-remove",
		HostPort: "stalehost:8081",
		PID:      1235,
	}

	staleFile := instanceFilePath(tmpDir, "test-service", staleInstance.ID)
	err = writeInstanceFile(staleFile, &staleInstance)
	require.NoError(t, err)

	// Make instance file old by changing its modification time
	oldTime := time.Now().Add(-time.Minute)
	err = os.Chtimes(staleFile, oldTime, oldTime)
	require.NoError(t, err)

	// Verify file exists before
	assert.FileExists(t, staleFile)

	resolver := newResolver(tmpDir, "test-service")
	resolver.staleTimeout = 30 * time.Second // 30 second timeout

	ctx := context.Background()
	members, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members)

	// Verify stale file was removed
	assert.NoFileExists(t, staleFile)
}

func TestResolver_RealWorldScenario(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate coordinator service discovery
	coordinatorResolver := newResolver(tmpDir, models.ServiceNameCoordinator)
	// Disable caching for this test to ensure we see updates immediately
	coordinatorResolver.cacheDuration = 0

	ctx := context.Background()

	// Initially no coordinators
	members, err := coordinatorResolver.Members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members)

	// Coordinator comes online
	coordinator1 := instanceInfo{
		ID:       "coordinator-primary",
		HostPort: "coord1.example.com:9090",
		PID:      2000,
	}
	filename := instanceFilePath(tmpDir, string(models.ServiceNameCoordinator), coordinator1.ID)
	err = writeInstanceFile(filename, &coordinator1)
	require.NoError(t, err)

	// Now we should see the coordinator
	members, err = coordinatorResolver.Members(ctx)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "coord1.example.com:9090", members[0].HostPort)

	// Second coordinator joins
	coordinator2 := instanceInfo{
		ID:       "coordinator-secondary",
		HostPort: "coord2.example.com:9090",
		PID:      2001,
	}
	filename = instanceFilePath(tmpDir, string(models.ServiceNameCoordinator), coordinator2.ID)
	err = writeInstanceFile(filename, &coordinator2)
	require.NoError(t, err)

	// Should see both coordinators
	members, err = coordinatorResolver.Members(ctx)
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
		ID:       "instance-1",
		HostPort: "host1:8080",
		PID:      1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance1.ID)
	err = writeInstanceFile(filename, &instance1)
	require.NoError(t, err)

	resolver := newResolver(tmpDir, "test-service")
	ctx := context.Background()

	// First call - should read from disk
	members1, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members1, 1)
	assert.Equal(t, "host1:8080", members1[0].HostPort)

	// Add another instance to disk
	instance2 := instanceInfo{
		ID:       "instance-2",
		HostPort: "host2:8081",
		PID:      1235,
	}
	filename = instanceFilePath(tmpDir, "test-service", instance2.ID)
	err = writeInstanceFile(filename, &instance2)
	require.NoError(t, err)

	// Second call immediately - should return cached result
	members2, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members2, 1) // Still only 1 member from cache
	assert.Equal(t, "host1:8080", members2[0].HostPort)

	// Verify cache is being used by checking the same data
	assert.Equal(t, members1[0].ID, members2[0].ID)
	assert.Equal(t, members1[0].HostPort, members2[0].HostPort)
}

func TestResolver_Members_CacheExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create initial instance
	instance1 := instanceInfo{
		ID:       "instance-1",
		HostPort: "host1:8080",
		PID:      1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance1.ID)
	err = writeInstanceFile(filename, &instance1)
	require.NoError(t, err)

	resolver := newResolver(tmpDir, "test-service")
	// Set short cache duration for testing
	resolver.cacheDuration = 100 * time.Millisecond
	ctx := context.Background()

	// First call - should read from disk
	members1, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members1, 1)

	// Add another instance
	instance2 := instanceInfo{
		ID:       "instance-2",
		HostPort: "host2:8081",
		PID:      1235,
	}
	filename = instanceFilePath(tmpDir, "test-service", instance2.ID)
	err = writeInstanceFile(filename, &instance2)
	require.NoError(t, err)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call - cache expired, should read from disk again
	members3, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members3, 2) // Now sees both instances
}

func TestResolver_Members_NoCacheForEmptyMembers(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := newResolver(tmpDir, "test-service")
	ctx := context.Background()

	// First call - no instances
	members1, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members1)

	// Create service directory and add instance
	serviceDir := filepath.Join(tmpDir, "test-service")
	err = os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	instance := instanceInfo{
		ID:       "instance-1",
		HostPort: "host1:8080",
		PID:      1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance.ID)
	err = writeInstanceFile(filename, &instance)
	require.NoError(t, err)

	// Second call immediately - should NOT use cache (since it was empty)
	members2, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members2, 1) // Should see the new instance
	assert.Equal(t, "host1:8080", members2[0].HostPort)
}

func TestResolver_Members_CacheConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "test-service")
	err := os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create instances
	for i := 0; i < 5; i++ {
		inst := instanceInfo{
			ID:       fmt.Sprintf("instance-%d", i),
			HostPort: fmt.Sprintf("host%d:808%d", i, i),
			PID:      1234 + i,
		}
		filename := instanceFilePath(tmpDir, "test-service", inst.ID)
		err := writeInstanceFile(filename, &inst)
		require.NoError(t, err)
	}

	resolver := newResolver(tmpDir, "test-service")
	ctx := context.Background()

	// Run concurrent reads
	const numGoroutines = 10
	results := make(chan []models.HostInfo, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			members, err := resolver.Members(ctx)
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
		ID:       "instance-1",
		HostPort: "host1:8080",
		PID:      1234,
	}
	filename := instanceFilePath(tmpDir, "test-service", instance1.ID)
	err = writeInstanceFile(filename, &instance1)
	require.NoError(t, err)

	resolver := newResolver(tmpDir, "test-service")
	// Use longer cache for this test
	resolver.cacheDuration = 5 * time.Second
	ctx := context.Background()

	// First call - populate cache
	members1, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members1, 1)

	// Remove the instance file
	err = os.Remove(filename)
	require.NoError(t, err)

	// Second call - should still return cached result
	members2, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members2, 1)

	// Manually expire cache
	resolver.mu.Lock()
	resolver.cacheTime = time.Now().Add(-10 * time.Second)
	resolver.mu.Unlock()

	// Third call - cache expired, should see no instances
	members3, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Empty(t, members3)
}
