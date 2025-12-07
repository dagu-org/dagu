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

func TestService_Lifecycle(t *testing.T) {
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

	// Start
	err := svc.Start(ctx)
	require.NoError(t, err)

	// Double start should be no-op
	err = svc.Start(ctx)
	require.NoError(t, err)

	// Wait for collection
	time.Sleep(150 * time.Millisecond)

	// Verify data collected
	history := svc.GetHistory(time.Hour)
	require.NotNil(t, history)
	assert.GreaterOrEqual(t, len(history.CPU), 1)

	// Stop
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

	history := svc.GetHistory(time.Hour)
	require.NotNil(t, history)
	assert.Empty(t, history.CPU)
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

	// Pre-close done channel to prevent blocking
	close(svc.done)

	err := svc.Stop(ctx)
	require.NoError(t, err)
}
