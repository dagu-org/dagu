package scheduler

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigSet(t *testing.T) {
	t.Run("NewConfigSet", func(t *testing.T) {
		configSet := NewConfigSet()
		assert.NotNil(t, configSet)
		assert.NotNil(t, configSet.configs)
		assert.Empty(t, configSet.configs)
	})

	t.Run("Get_NonExistentConfig", func(t *testing.T) {
		configSet := NewConfigSet()
		config := configSet.Get("non-existent")
		// Should return zero value of Config
		assert.Equal(t, 0, config.ConcurrencyLimit)
	})

	t.Run("Set_And_Get", func(t *testing.T) {
		configSet := NewConfigSet()
		dagName := "test-dag"
		expectedConfig := Config{
			ConcurrencyLimit: 5,
		}

		configSet.Set(dagName, expectedConfig)
		actualConfig := configSet.Get(dagName)

		assert.Equal(t, expectedConfig, actualConfig)
	})

	t.Run("DefaultDAGConfig", func(t *testing.T) {
		assert.Equal(t, 1, DefaultConfig.ConcurrencyLimit)
	})

	t.Run("Concurrent_Access", func(t *testing.T) {
		configSet := NewConfigSet()
		dagName := "concurrent-dag"

		// Test concurrent reads and writes
		var wg sync.WaitGroup
		wg.Add(10)

		// 5 goroutines setting the config
		for i := 0; i < 5; i++ {
			go func(val int) {
				defer wg.Done()
				configSet.Set(dagName, Config{ConcurrencyLimit: val})
			}(i)
		}

		// 5 goroutines getting the config
		for i := 0; i < 5; i++ {
			go func() {
				defer wg.Done()
				_ = configSet.Get(dagName)
			}()
		}

		wg.Wait()

		// We can't assert on the exact value since the order of execution is non-deterministic,
		// but we can verify that the operation completed without deadlocks or panics
		_ = configSet.Get(dagName)
	})

	t.Run("Multiple_DAGs", func(t *testing.T) {
		configSet := NewConfigSet()

		dag1 := "dag-1"
		config1 := Config{ConcurrencyLimit: 3}

		dag2 := "dag-2"
		config2 := Config{ConcurrencyLimit: 7}

		configSet.Set(dag1, config1)
		configSet.Set(dag2, config2)

		assert.Equal(t, config1, configSet.Get(dag1))
		assert.Equal(t, config2, configSet.Get(dag2))
	})
}
