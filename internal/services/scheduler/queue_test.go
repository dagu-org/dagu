package scheduler

import (
	"testing"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestScheduler_QueueMethods(t *testing.T) {
	t.Parallel()

	t.Run("GetQueueConfigByNameGlobalConfigExists", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config: []config.QueueConfig{
					{Name: "highPriority", MaxActiveRuns: 2},
					{Name: "lowPriority", MaxActiveRuns: 10},
				},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &core.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 5, // Should be ignored since global config exists
		}

		queueCfg := s.getQueueConfig("highPriority", dag)
		assert.Equal(t, 2, queueCfg.MaxConcurrency)
	})

	t.Run("GetQueueConfigByNameFallbackToDAGMaxActiveRuns", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config:  []config.QueueConfig{}, // No global queue configuration
			},
		}

		s := &Scheduler{config: cfg}

		dag := &core.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 7,
		}

		queueCfg := s.getQueueConfig("unknownQueue", dag)
		assert.Equal(t, 7, queueCfg.MaxConcurrency)
	})

	t.Run("GetQueueConfigByNameDefaultToOne", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config:  []config.QueueConfig{}, // No global queue configuration
			},
		}

		s := &Scheduler{config: cfg}

		dag := &core.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 0, // No DAG max active runs
		}

		queueCfg := s.getQueueConfig("unknownQueue", dag)
		assert.Equal(t, 1, queueCfg.MaxConcurrency)
	})

	t.Run("GetQueueConfigByNameGlobalConfigPriority", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config: []config.QueueConfig{
					{Name: "testQueue", MaxActiveRuns: 3},
				},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &core.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 10, // Should be ignored since global config exists for testQueue
		}

		queueCfg := s.getQueueConfig("testQueue", dag)
		assert.Equal(t, 3, queueCfg.MaxConcurrency) // Global config takes priority
	})

	t.Run("GetQueueConfigByNameMinimumOne", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config: []config.QueueConfig{
					{Name: "testQueue", MaxActiveRuns: 0}, // Zero should be converted to 1
				},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &core.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 5,
		}

		queueCfg := s.getQueueConfig("testQueue", dag)
		assert.Equal(t, 1, queueCfg.MaxConcurrency) // Should be at least 1
	})
}
