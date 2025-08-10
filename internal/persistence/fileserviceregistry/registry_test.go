package fileserviceregistry

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

func TestRegistry_RegisterUnregister(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:     "test-instance",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err := registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)

	// Check that service registry directory was created
	assert.DirExists(t, tmpDir)

	// Stop should not error
	registry.Unregister(ctx)
}

func TestRegistry_GetServiceMembers(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)

	ctx := context.Background()

	// Test getting members for empty service
	members, err := registry.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	assert.Empty(t, members)

	// Register a service
	hostInfo := models.HostInfo{
		ID:     "test-instance",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err = registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer registry.Unregister(ctx)

	// Now should find the registered member
	members, err = registry.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "localhost:8080", fmt.Sprintf("%s:%d", members[0].Host, members[0].Port))
}

func TestRegistry_RegisterInstance(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:     "test-coordinator",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err := registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer registry.Unregister(ctx)

	// Check that instance file was created
	serviceDir := filepath.Join(tmpDir, string(models.ServiceNameCoordinator))
	entries, err := os.ReadDir(serviceDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Verify registry can find the registered instance
	members, err := registry.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "localhost:8080", fmt.Sprintf("%s:%d", members[0].Host, members[0].Port))
}

func TestRegistry_Heartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)
	registry.heartbeatInterval = 100 * time.Millisecond // Short interval for testing

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:     "test-heartbeat",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err := registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer registry.Unregister(ctx)

	// Get initial heartbeat time
	members1, err := registry.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	require.Len(t, members1, 1)

	// Heartbeat already started by Register method

	// Wait for heartbeat to update
	time.Sleep(200 * time.Millisecond)

	// Verify heartbeat was updated
	members2, err := registry.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	require.Len(t, members2, 1)
	assert.Equal(t, members1[0].ID, members2[0].ID)
}

func TestRegistry_UnregisterRemovesInstance(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:     "test-stop",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err := registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)

	// Verify it exists
	members, err := registry.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	assert.Len(t, members, 1)

	// Unregister from registry
	registry.Unregister(ctx)

	// Verify instance file was removed
	serviceDir := filepath.Join(tmpDir, string(models.ServiceNameCoordinator))
	entries, err := os.ReadDir(serviceDir)
	if err == nil {
		assert.Empty(t, entries)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:     "test-concurrent",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err := registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer registry.Unregister(ctx)

	// Concurrent finder access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			serviceName := models.ServiceName(fmt.Sprintf("%s-%d", models.ServiceNameCoordinator, i))
			// Just verify we can get members without error
			_, err := registry.GetServiceMembers(context.Background(), serviceName)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRegistry_HeartbeatRecreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	registry := New(tmpDir)
	registry.heartbeatInterval = 100 * time.Millisecond // Short interval for testing

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:     "test-recreate",
		Host:   "localhost",
		Port:   8080,
		Status: models.ServiceStatusActive,
	}
	err := registry.Register(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer registry.Unregister(ctx)

	// Verify file exists
	instanceFile := filepath.Join(tmpDir, string(models.ServiceNameCoordinator), "test-recreate.json")
	assert.FileExists(t, instanceFile)

	// Delete the file to simulate accidental deletion
	err = os.Remove(instanceFile)
	require.NoError(t, err)
	assert.NoFileExists(t, instanceFile)

	// Wait for heartbeat to recreate it
	time.Sleep(200 * time.Millisecond)

	// Verify file was recreated
	assert.FileExists(t, instanceFile)

	// Verify content is correct
	info, err := readInstanceFile(instanceFile)
	require.NoError(t, err)
	assert.Equal(t, "test-recreate", info.ID)
	assert.Equal(t, "localhost:8080", fmt.Sprintf("%s:%d", info.Host, info.Port))
}

func TestRegistry_MultipleInstances(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create multiple monitors for different instances
	instances := []struct {
		serviceName models.ServiceName
		hostInfo    models.HostInfo
	}{
		{
			serviceName: models.ServiceNameCoordinator,
			hostInfo: models.HostInfo{
				ID:     "coord-1",
				Host:   "coord1.example.com",
				Port:   9090,
				Status: models.ServiceStatusActive,
			},
		},
		{
			serviceName: models.ServiceNameCoordinator,
			hostInfo: models.HostInfo{
				ID:     "coord-2",
				Host:   "coord2.example.com",
				Port:   9090,
				Status: models.ServiceStatusActive,
			},
		},
		{
			serviceName: "worker",
			hostInfo: models.HostInfo{
				ID:     "worker-1",
				Host:   "worker1.example.com",
				Port:   8080,
				Status: models.ServiceStatusActive,
			},
		},
	}

	// Register all instances
	registries := make([]*registry, len(instances))
	for i, inst := range instances {
		registry := New(tmpDir)
		err := registry.Register(ctx, inst.serviceName, inst.hostInfo)
		require.NoError(t, err)
		registries[i] = registry
		defer registry.Unregister(ctx)
	}

	// Use any registry to resolve services
	resolver := registries[0]

	// Check coordinator service has 2 instances
	coordMembers, err := resolver.GetServiceMembers(ctx, models.ServiceNameCoordinator)
	require.NoError(t, err)
	assert.Len(t, coordMembers, 2)

	// Check worker service has 1 instance
	workerMembers, err := resolver.GetServiceMembers(ctx, "worker")
	require.NoError(t, err)
	assert.Len(t, workerMembers, 1)
	assert.Equal(t, "worker1.example.com:8080", fmt.Sprintf("%s:%d", workerMembers[0].Host, workerMembers[0].Port))
}
