package scheduler

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowConfigSet(t *testing.T) {
	t.Run("NewWorkflowConfigSet", func(t *testing.T) {
		configSet := NewConfigSet()
		assert.NotNil(t, configSet)
		assert.NotNil(t, configSet.configs)
		assert.Empty(t, configSet.configs)
	})

	t.Run("Get_NonExistentConfig", func(t *testing.T) {
		configSet := NewConfigSet()
		config := configSet.Get("non-existent")
		// Should return zero value of WorkflowConfig
		assert.Equal(t, 0, config.ConcurrencyLimit)
	})

	t.Run("Set_And_Get", func(t *testing.T) {
		configSet := NewConfigSet()
		workflowName := "test-workflow"
		expectedConfig := Config{
			ConcurrencyLimit: 5,
		}

		configSet.Set(workflowName, expectedConfig)
		actualConfig := configSet.Get(workflowName)

		assert.Equal(t, expectedConfig, actualConfig)
	})

	t.Run("DefaultWorkflowConfig", func(t *testing.T) {
		assert.Equal(t, 1, DefaultConfig.ConcurrencyLimit)
	})

	t.Run("Concurrent_Access", func(t *testing.T) {
		configSet := NewConfigSet()
		workflowName := "concurrent-workflow"

		// Test concurrent reads and writes
		var wg sync.WaitGroup
		wg.Add(10)

		// 5 goroutines setting the config
		for i := 0; i < 5; i++ {
			go func(val int) {
				defer wg.Done()
				configSet.Set(workflowName, Config{ConcurrencyLimit: val})
			}(i)
		}

		// 5 goroutines getting the config
		for i := 0; i < 5; i++ {
			go func() {
				defer wg.Done()
				_ = configSet.Get(workflowName)
			}()
		}

		wg.Wait()

		// We can't assert on the exact value since the order of execution is non-deterministic,
		// but we can verify that the operation completed without deadlocks or panics
		_ = configSet.Get(workflowName)
	})

	t.Run("Multiple_Workflows", func(t *testing.T) {
		configSet := NewConfigSet()

		workflow1 := "workflow-1"
		config1 := Config{ConcurrencyLimit: 3}

		workflow2 := "workflow-2"
		config2 := Config{ConcurrencyLimit: 7}

		configSet.Set(workflow1, config1)
		configSet.Set(workflow2, config2)

		assert.Equal(t, config1, configSet.Get(workflow1))
		assert.Equal(t, config2, configSet.Get(workflow2))
	})
}
