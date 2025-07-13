package backoff

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJitterFunc(t *testing.T) {
	t.Run("NoJitter", func(t *testing.T) {
		jitterFunc := NewJitterFunc(NoJitter)
		interval := 100 * time.Millisecond

		// Should always return the same interval
		for i := 0; i < 10; i++ {
			result := jitterFunc(interval)
			assert.Equal(t, interval, result)
		}
	})

	t.Run("FullJitter", func(t *testing.T) {
		jitterFunc := NewJitterFunc(FullJitter)
		interval := 1000 * time.Millisecond

		// Collect multiple samples
		hasVariation := false
		var results []time.Duration
		for i := 0; i < 100; i++ {
			result := jitterFunc(interval)
			results = append(results, result)

			// Check bounds: should be between 0 and interval
			assert.GreaterOrEqual(t, result, time.Duration(0))
			assert.LessOrEqual(t, result, interval)

			// Check for variation
			if i > 0 && result != results[0] {
				hasVariation = true
			}
		}

		// Should have some variation
		assert.True(t, hasVariation, "FullJitter should produce varying results")
	})

	t.Run("Jitter", func(t *testing.T) {
		jitterFunc := NewJitterFunc(Jitter)
		interval := 1000 * time.Millisecond

		// Collect multiple samples
		hasVariation := false
		minSeen := interval
		maxSeen := time.Duration(0)

		for i := 0; i < 100; i++ {
			result := jitterFunc(interval)

			// Check bounds: should be between 0.5x and 1.5x interval
			assert.GreaterOrEqual(t, result, interval/2)
			assert.LessOrEqual(t, result, interval+interval/2)

			// Track min/max
			if result < minSeen {
				minSeen = result
			}
			if result > maxSeen {
				maxSeen = result
			}

			// Check for variation
			if i > 0 && result != interval {
				hasVariation = true
			}
		}

		// Should have variation and good spread
		assert.True(t, hasVariation, "Jitter should produce varying results")
		assert.Less(t, minSeen, 600*time.Millisecond, "Should see values near lower bound")
		assert.Greater(t, maxSeen, 1400*time.Millisecond, "Should see values near upper bound")
	})

	t.Run("ZeroInterval", func(t *testing.T) {
		// All jitter types should return 0 for 0 interval
		jitterTypes := []JitterType{NoJitter, FullJitter, Jitter}

		for _, jt := range jitterTypes {
			jitterFunc := NewJitterFunc(jt)
			result := jitterFunc(0)
			assert.Equal(t, time.Duration(0), result, "JitterType %d should return 0 for 0 interval", jt)
		}
	})

	t.Run("NegativeInterval", func(t *testing.T) {
		// All jitter types should return 0 for negative interval
		jitterTypes := []JitterType{NoJitter, FullJitter, Jitter}

		for _, jt := range jitterTypes {
			jitterFunc := NewJitterFunc(jt)
			result := jitterFunc(-100 * time.Millisecond)
			assert.Equal(t, time.Duration(0), result, "JitterType %d should return 0 for negative interval", jt)
		}
	})
}

func TestWithJitter(t *testing.T) {
	t.Run("ExponentialWithFullJitter", func(t *testing.T) {
		basePolicy := &ExponentialBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     1 * time.Second,
			MaxRetries:      5,
		}

		policy := WithJitter(basePolicy, FullJitter)

		// Test multiple retries
		for i := 0; i < 5; i++ {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)

			// Calculate expected base interval
			multiplier := 1
			for j := 0; j < i; j++ {
				multiplier *= 2
			}
			expectedBase := time.Duration(100*time.Millisecond) * time.Duration(multiplier)
			if expectedBase > 1*time.Second {
				expectedBase = 1 * time.Second
			}

			// Jittered interval should be between 0 and base
			assert.GreaterOrEqual(t, interval, time.Duration(0))
			assert.LessOrEqual(t, interval, expectedBase)
		}

		// Max retries should still be enforced
		_, err := policy.ComputeNextInterval(5, 0, nil)
		assert.Equal(t, ErrRetriesExhausted, err)
	})

	t.Run("ConstantWithJitter", func(t *testing.T) {
		basePolicy := &ConstantBackoffPolicy{
			Interval:   200 * time.Millisecond,
			MaxRetries: 3,
		}

		policy := WithJitter(basePolicy, Jitter)

		// Collect intervals to check for jitter
		hasVariation := false
		var firstInterval time.Duration

		for i := 0; i < 3; i++ {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)

			// Should be within jitter bounds (0.5x to 1.5x)
			assert.GreaterOrEqual(t, interval, 100*time.Millisecond)
			assert.LessOrEqual(t, interval, 300*time.Millisecond)

			if i == 0 {
				firstInterval = interval
			} else if interval != firstInterval {
				hasVariation = true
			}
		}

		assert.True(t, hasVariation, "Should have variation with jitter")
	})

	t.Run("LinearWithNoJitter", func(t *testing.T) {
		basePolicy := &LinearBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			Increment:       50 * time.Millisecond,
			MaxInterval:     500 * time.Millisecond,
			MaxRetries:      3,
		}

		policy := WithJitter(basePolicy, NoJitter)

		// With NoJitter, should return exact linear intervals
		expectedIntervals := []time.Duration{
			100 * time.Millisecond,
			150 * time.Millisecond,
			200 * time.Millisecond,
		}

		for i, expected := range expectedIntervals {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, expected, interval)
		}
	})
}

func TestJitterFunc_ConcurrentUse(t *testing.T) {
	// Test that jitter is thread-safe
	jitterFunc := NewJitterFunc(FullJitter)
	interval := 100 * time.Millisecond

	// Run multiple goroutines applying jitter
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				result := jitterFunc(interval)
				// Just verify bounds
				if result < 0 || result > interval {
					t.Errorf("Invalid jitter result: %v", result)
					return
				}
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	}
}
