package scheduler

import (
	"testing"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
)

func TestScheduler_QueueMethods(t *testing.T) {
	t.Parallel()

	t.Run("getQueueNameForDAG_WithExplicitQueue", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config:  []config.QueueConfig{},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &digraph.DAG{
			Name:  "test-dag",
			Queue: "customQueue",
		}

		queueName := s.getQueueNameForDAG(dag)
		assert.Equal(t, "customQueue", queueName)
	})

	t.Run("getQueueNameForDAG_DefaultsToDAGName", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config:  []config.QueueConfig{},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &digraph.DAG{
			Name:  "my-pipeline",
			Queue: "", // Empty queue should default to DAG name
		}

		queueName := s.getQueueNameForDAG(dag)
		assert.Equal(t, "my-pipeline", queueName)
	})

	t.Run("getQueueConfigByName_GlobalConfigExists", func(t *testing.T) {
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

		dag := &digraph.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 5, // Should be ignored since global config exists
		}

		queueCfg := s.getQueueConfigByName("highPriority", dag)
		assert.Equal(t, 2, queueCfg.MaxConcurrency)
	})

	t.Run("getQueueConfigByName_FallbackToDAGMaxActiveRuns", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config:  []config.QueueConfig{}, // No global queue configuration
			},
		}

		s := &Scheduler{config: cfg}

		dag := &digraph.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 7,
		}

		queueCfg := s.getQueueConfigByName("unknownQueue", dag)
		assert.Equal(t, 7, queueCfg.MaxConcurrency)
	})

	t.Run("getQueueConfigByName_DefaultToOne", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config:  []config.QueueConfig{}, // No global queue configuration
			},
		}

		s := &Scheduler{config: cfg}

		dag := &digraph.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 0, // No DAG max active runs
		}

		queueCfg := s.getQueueConfigByName("unknownQueue", dag)
		assert.Equal(t, 1, queueCfg.MaxConcurrency)
	})

	t.Run("getQueueConfigByName_GlobalConfigPriority", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config: []config.QueueConfig{
					{Name: "testQueue", MaxActiveRuns: 3},
				},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &digraph.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 10, // Should be ignored since global config exists for testQueue
		}

		queueCfg := s.getQueueConfigByName("testQueue", dag)
		assert.Equal(t, 3, queueCfg.MaxConcurrency) // Global config takes priority
	})

	t.Run("getQueueConfigByName_MinimumOne", func(t *testing.T) {
		cfg := &config.Config{
			Queues: config.Queues{
				Enabled: true,
				Config: []config.QueueConfig{
					{Name: "testQueue", MaxActiveRuns: 0}, // Zero should be converted to 1
				},
			},
		}

		s := &Scheduler{config: cfg}

		dag := &digraph.DAG{
			Name:          "test-dag",
			MaxActiveRuns: 5,
		}

		queueCfg := s.getQueueConfigByName("testQueue", dag)
		assert.Equal(t, 1, queueCfg.MaxConcurrency) // Should be at least 1
	})
}