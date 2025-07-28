package filediscovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitor_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:       "test-instance",
		HostPort: "localhost:8080",
	}
	err := monitor.Start(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)

	// Check that discovery directory was created
	assert.DirExists(t, tmpDir)

	// Stop should not error
	monitor.Stop(ctx)
}

func TestMonitor_Resolver(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)

	// Get resolver for coordinator service
	resolver1 := monitor.Resolver(context.Background(), models.ServiceNameCoordinator)
	assert.NotNil(t, resolver1)

	// Getting the same service should return the same resolver
	resolver2 := monitor.Resolver(context.Background(), models.ServiceNameCoordinator)
	assert.Equal(t, resolver1, resolver2)

	// Different service should return different resolver
	resolver3 := monitor.Resolver(context.Background(), "other-service")
	assert.NotEqual(t, resolver1, resolver3)
}

func TestMonitor_RegisterInstance(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:       "test-coordinator",
		HostPort: "localhost:8080",
	}
	err := monitor.Start(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer monitor.Stop(ctx)

	// Check that instance file was created
	serviceDir := filepath.Join(tmpDir, string(models.ServiceNameCoordinator))
	entries, err := os.ReadDir(serviceDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Verify resolver can find the registered instance
	resolver := monitor.Resolver(ctx, models.ServiceNameCoordinator)
	members, err := resolver.Members(ctx)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "localhost:8080", members[0].HostPort)
}

func TestMonitor_Heartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)
	monitor.heartbeatInterval = 100 * time.Millisecond // Short interval for testing

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:       "test-heartbeat",
		HostPort: "localhost:8080",
	}
	err := monitor.Start(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer monitor.Stop(ctx)

	// Get initial heartbeat time
	resolver := monitor.Resolver(ctx, models.ServiceNameCoordinator)
	members1, err := resolver.Members(ctx)
	require.NoError(t, err)
	require.Len(t, members1, 1)

	// Heartbeat already started by Start method

	// Wait for heartbeat to update
	time.Sleep(200 * time.Millisecond)

	// Verify heartbeat was updated
	members2, err := resolver.Members(ctx)
	require.NoError(t, err)
	require.Len(t, members2, 1)
	assert.Equal(t, members1[0].ID, members2[0].ID)
}

func TestMonitor_StopRemovesInstance(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:       "test-stop",
		HostPort: "localhost:8080",
	}
	err := monitor.Start(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)

	// Verify it exists
	resolver := monitor.Resolver(ctx, models.ServiceNameCoordinator)
	members, err := resolver.Members(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 1)

	// Stop monitor
	monitor.Stop(ctx)

	// Verify instance file was removed
	serviceDir := filepath.Join(tmpDir, string(models.ServiceNameCoordinator))
	entries, err := os.ReadDir(serviceDir)
	if err == nil {
		assert.Empty(t, entries)
	}
}

func TestMonitor_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:       "test-concurrent",
		HostPort: "localhost:8080",
	}
	err := monitor.Start(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer monitor.Stop(ctx)

	// Concurrent resolver access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			serviceName := models.ServiceName(string(models.ServiceNameCoordinator) + string(rune(i)))
			resolver := monitor.Resolver(context.Background(), serviceName)
			assert.NotNil(t, resolver)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMonitor_HeartbeatRecreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	monitor := New(tmpDir)
	monitor.heartbeatInterval = 100 * time.Millisecond // Short interval for testing

	ctx := context.Background()
	hostInfo := models.HostInfo{
		ID:       "test-recreate",
		HostPort: "localhost:8080",
	}
	err := monitor.Start(ctx, models.ServiceNameCoordinator, hostInfo)
	require.NoError(t, err)
	defer monitor.Stop(ctx)

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
	assert.Equal(t, "localhost:8080", info.HostPort)
}

func TestMonitor_MultipleInstances(t *testing.T) {
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
				ID:       "coord-1",
				HostPort: "coord1.example.com:9090",
			},
		},
		{
			serviceName: models.ServiceNameCoordinator,
			hostInfo: models.HostInfo{
				ID:       "coord-2",
				HostPort: "coord2.example.com:9090",
			},
		},
		{
			serviceName: "worker",
			hostInfo: models.HostInfo{
				ID:       "worker-1",
				HostPort: "worker1.example.com:8080",
			},
		},
	}

	// Start all monitors
	monitors := make([]*Monitor, len(instances))
	for i, inst := range instances {
		monitor := New(tmpDir)
		err := monitor.Start(ctx, inst.serviceName, inst.hostInfo)
		require.NoError(t, err)
		monitors[i] = monitor
		defer monitor.Stop(ctx)
	}

	// Use any monitor to resolve services
	resolver := monitors[0]

	// Check coordinator service has 2 instances
	coordMembers, err := resolver.Resolver(ctx, models.ServiceNameCoordinator).Members(ctx)
	require.NoError(t, err)
	assert.Len(t, coordMembers, 2)

	// Check worker service has 1 instance
	workerMembers, err := resolver.Resolver(ctx, "worker").Members(ctx)
	require.NoError(t, err)
	assert.Len(t, workerMembers, 1)
	assert.Equal(t, "worker1.example.com:8080", workerMembers[0].HostPort)
}
