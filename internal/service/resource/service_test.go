package resource

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  5 * time.Second,
		},
	}

	svc := NewService(cfg)

	require.NotNil(t, svc)
	assert.NotNil(t, svc.store)
	assert.NotNil(t, svc.done)
	assert.Equal(t, cfg, svc.config)
}

func TestService_StartStop(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  100 * time.Millisecond,
		},
		Paths: config.PathsConfig{
			DataDir: t.TempDir(),
		},
	}

	svc := NewService(cfg)
	ctx := context.Background()

	// Start the service
	err := svc.Start(ctx)
	require.NoError(t, err)

	// Give time for initial collection
	time.Sleep(150 * time.Millisecond)

	// Stop the service
	err = svc.Stop(ctx)
	require.NoError(t, err)
}

func TestService_DoubleStart(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  100 * time.Millisecond,
		},
		Paths: config.PathsConfig{
			DataDir: t.TempDir(),
		},
	}

	svc := NewService(cfg)
	ctx := context.Background()

	// First start
	err := svc.Start(ctx)
	require.NoError(t, err)

	// Second start should be a no-op (guard against double-start)
	err = svc.Start(ctx)
	require.NoError(t, err)

	// Stop the service
	err = svc.Stop(ctx)
	require.NoError(t, err)
}

func TestService_GetHistory(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  50 * time.Millisecond,
		},
		Paths: config.PathsConfig{
			DataDir: t.TempDir(),
		},
	}

	svc := NewService(cfg)
	ctx := context.Background()

	// Start the service
	err := svc.Start(ctx)
	require.NoError(t, err)

	// Wait for at least one collection cycle
	time.Sleep(100 * time.Millisecond)

	// Get history
	history := svc.GetHistory(time.Hour)
	require.NotNil(t, history)

	// Should have at least 1 data point from initial collection
	assert.GreaterOrEqual(t, len(history.CPU), 1)
	assert.GreaterOrEqual(t, len(history.Memory), 1)
	assert.GreaterOrEqual(t, len(history.Disk), 1)
	assert.GreaterOrEqual(t, len(history.Load), 1)

	// Stop the service
	err = svc.Stop(ctx)
	require.NoError(t, err)
}

func TestService_GetHistoryWithoutStart(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  time.Second,
		},
	}

	svc := NewService(cfg)

	// Should return empty history without error
	history := svc.GetHistory(time.Hour)
	require.NotNil(t, history)
	assert.Empty(t, history.CPU)
	assert.Empty(t, history.Memory)
	assert.Empty(t, history.Disk)
	assert.Empty(t, history.Load)
}

func TestService_StopWithoutStart(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  time.Second,
		},
	}

	svc := NewService(cfg)
	ctx := context.Background()

	// Create a new done channel that's already closed to prevent blocking
	close(svc.done)

	// Stop should handle nil cancel gracefully
	err := svc.Stop(ctx)
	require.NoError(t, err)
}

func TestService_CollectsMetricsPeriodically(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Retention: time.Hour,
			Interval:  50 * time.Millisecond,
		},
		Paths: config.PathsConfig{
			DataDir: t.TempDir(),
		},
	}

	svc := NewService(cfg)
	ctx := context.Background()

	err := svc.Start(ctx)
	require.NoError(t, err)

	// Wait for multiple collection cycles
	time.Sleep(200 * time.Millisecond)

	history := svc.GetHistory(time.Hour)

	// Should have multiple data points
	assert.GreaterOrEqual(t, len(history.CPU), 2, "should have collected multiple CPU samples")

	err = svc.Stop(ctx)
	require.NoError(t, err)
}
