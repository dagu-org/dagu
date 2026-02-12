package backoff

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExponentialBackoffPolicy_ComputeNextInterval(t *testing.T) {
	t.Run("BasicExponentialBackoff", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     5 * time.Second,
			MaxRetries:      5,
		}

		testCases := []struct {
			retryCount       int
			expectedInterval time.Duration
			expectError      bool
		}{
			{0, 100 * time.Millisecond, false},
			{1, 200 * time.Millisecond, false},
			{2, 400 * time.Millisecond, false},
			{3, 800 * time.Millisecond, false},
			{4, 1600 * time.Millisecond, false},
			{5, 0, true}, // Max retries reached
		}

		for _, tc := range testCases {
			interval, err := policy.ComputeNextInterval(tc.retryCount, 0, nil)
			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrRetriesExhausted, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedInterval, interval)
			}
		}
	})

	t.Run("MaxIntervalCapping", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 1 * time.Second,
			BackoffFactor:   2.0,
			MaxInterval:     3 * time.Second,
			MaxRetries:      10,
		}

		// Test that interval is capped at MaxInterval
		testCases := []struct {
			retryCount       int
			expectedInterval time.Duration
		}{
			{0, 1 * time.Second},
			{1, 2 * time.Second},
			{2, 3 * time.Second}, // Capped at MaxInterval
			{3, 3 * time.Second}, // Still capped
			{4, 3 * time.Second}, // Still capped
		}

		for _, tc := range testCases {
			interval, err := policy.ComputeNextInterval(tc.retryCount, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedInterval, interval)
		}
	})

	t.Run("UnlimitedRetries", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     1 * time.Second,
			MaxRetries:      0, // Unlimited
		}

		// Test that retries continue indefinitely
		for i := range 100 {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.LessOrEqual(t, interval, 1*time.Second)
		}
	})

	t.Run("CustomBackoffFactor", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			BackoffFactor:   1.5,
			MaxInterval:     10 * time.Second,
			MaxRetries:      5,
		}

		testCases := []struct {
			retryCount       int
			expectedInterval time.Duration
		}{
			{0, 100 * time.Millisecond},
			{1, 150 * time.Millisecond},
			{2, 225 * time.Millisecond},
			{3, 337500 * time.Microsecond}, // 337.5ms
		}

		for _, tc := range testCases {
			interval, err := policy.ComputeNextInterval(tc.retryCount, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedInterval, interval)
		}
	})
}

func TestNewExponentialBackoffPolicy(t *testing.T) {
	initialInterval := 500 * time.Millisecond
	policy := NewExponentialBackoffPolicy(initialInterval)

	assert.Equal(t, initialInterval, policy.InitialInterval)
	assert.Equal(t, defaultBackoffFactor, policy.BackoffFactor)
	assert.Equal(t, defaultMaxInterval, policy.MaxInterval)
	assert.Equal(t, defaultMaxRetries, policy.MaxRetries)
}

func TestRetrier_Next(t *testing.T) {
	t.Run("SuccessfulRetry", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 50 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     200 * time.Millisecond,
			MaxRetries:      3,
		}

		retrier := NewRetrier(policy)

		// First retry - should return 50ms interval
		interval, err := retrier.Next(errors.New("test error"))
		require.NoError(t, err)
		assert.Equal(t, 50*time.Millisecond, interval)

		// Second retry - should return 100ms interval
		interval, err = retrier.Next(errors.New("test error"))
		require.NoError(t, err)
		assert.Equal(t, 100*time.Millisecond, interval)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 1 * time.Second,
			BackoffFactor:   2.0,
			MaxInterval:     5 * time.Second,
			MaxRetries:      3,
		}

		retrier := NewRetrier(policy)

		// Context cancellation no longer applies to Next() method
		// Next() just returns the interval, it doesn't wait
		interval, err := retrier.Next(errors.New("test error"))
		require.NoError(t, err)
		assert.Equal(t, 1*time.Second, interval)
	})

	t.Run("RetriesExhausted", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 10 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     100 * time.Millisecond,
			MaxRetries:      2,
		}

		retrier := NewRetrier(policy)

		// First retry - should succeed
		_, err := retrier.Next(errors.New("test error"))
		require.NoError(t, err)

		// Second retry - should succeed
		_, err = retrier.Next(errors.New("test error"))
		require.NoError(t, err)

		// Third retry - should fail (max retries = 2)
		_, err = retrier.Next(errors.New("test error"))
		assert.Equal(t, ErrRetriesExhausted, err)
	})

	t.Run("TimeTracking", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 50 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     200 * time.Millisecond,
			MaxRetries:      5,
		}

		retrier := NewRetrier(policy)

		// Multiple retries should track elapsed time from start
		impl := retrier.(*retrierImpl)

		// First call initializes start time
		_, err := retrier.Next(errors.New("test error"))
		require.NoError(t, err)
		assert.False(t, impl.startTime.IsZero())
		startTime := impl.startTime

		// Subsequent calls should not reset start time
		_, err = retrier.Next(errors.New("test error"))
		require.NoError(t, err)
		assert.Equal(t, startTime, impl.startTime)
	})

	t.Run("ImmediateReturnOnZeroInterval", func(t *testing.T) {
		// Custom policy that returns zero interval
		mockPolicy := &mockRetryPolicy{
			intervals: []time.Duration{0, 0, 0},
		}

		retrier := NewRetrier(mockPolicy)

		// Should return zero interval
		interval, err := retrier.Next(nil)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), interval)
	})
}

var _ RetryPolicy = (*mockRetryPolicy)(nil)

// mockRetryPolicy is a test helper that returns predefined intervals
type mockRetryPolicy struct {
	intervals []time.Duration
	callCount int
}

func (m *mockRetryPolicy) ComputeNextInterval(retryCount int, _ time.Duration, _ error) (time.Duration, error) {
	if retryCount >= len(m.intervals) {
		return 0, ErrRetriesExhausted
	}
	interval := m.intervals[retryCount]
	m.callCount++
	return interval, nil
}

func TestExponentialBackoffPolicy_EdgeCases(t *testing.T) {
	t.Run("ZeroInitialInterval", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 0,
			BackoffFactor:   2.0,
			MaxInterval:     1 * time.Second,
			MaxRetries:      3,
		}

		// Should return 0 for all retries
		for i := range 3 {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), interval)
		}
	})

	t.Run("BackoffFactorOfOne", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			BackoffFactor:   1.0,
			MaxInterval:     1 * time.Second,
			MaxRetries:      5,
		}

		// Should return constant interval
		for i := range 5 {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, 100*time.Millisecond, interval)
		}
	})

	t.Run("VeryLargeBackoffFactor", func(t *testing.T) {
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 1 * time.Millisecond,
			BackoffFactor:   10.0,
			MaxInterval:     1 * time.Second,
			MaxRetries:      10,
		}

		// Should quickly hit max interval
		intervals := []time.Duration{
			1 * time.Millisecond,   // retry 0
			10 * time.Millisecond,  // retry 1
			100 * time.Millisecond, // retry 2
			1 * time.Second,        // retry 3 (capped)
			1 * time.Second,        // retry 4 (capped)
		}

		for i, expected := range intervals {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, expected, interval)
		}
	})
}

func TestRetrier_ConcurrentUse(t *testing.T) {
	// Test that multiple goroutines can safely use the same retrier
	policy := &ExponentialBackoffPolicy{
		InitialInterval: 10 * time.Millisecond,
		BackoffFactor:   2.0,
		MaxInterval:     100 * time.Millisecond,
		MaxRetries:      0, // Unlimited
	}

	retrier := NewRetrier(policy)

	// Run multiple goroutines
	done := make(chan bool, 3)
	for i := range 3 {
		go func(id int) {
			defer func() { done <- true }()

			for range 3 {
				_, err := retrier.Next(nil)
				if err != nil {
					t.Errorf("goroutine %d: unexpected error: %v", id, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for range 3 {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	}
}

func TestConstantBackoffPolicy_ComputeNextInterval(t *testing.T) {
	t.Run("ConstantInterval", func(t *testing.T) {
		policy := &ConstantBackoffPolicy{
			Interval:   100 * time.Millisecond,
			MaxRetries: 5,
		}

		// All retries should have the same interval
		for i := range 5 {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, 100*time.Millisecond, interval)
		}

		// Max retries reached
		interval, err := policy.ComputeNextInterval(5, 0, nil)
		assert.Error(t, err)
		assert.Equal(t, ErrRetriesExhausted, err)
		assert.Equal(t, time.Duration(0), interval)
	})

	t.Run("UnlimitedRetries", func(t *testing.T) {
		policy := &ConstantBackoffPolicy{
			Interval:   50 * time.Millisecond,
			MaxRetries: 0, // Unlimited
		}

		// Test many retries
		for i := range 100 {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, 50*time.Millisecond, interval)
		}
	})
}

func TestNewConstantBackoffPolicy(t *testing.T) {
	interval := 250 * time.Millisecond
	policy := NewConstantBackoffPolicy(interval)

	assert.Equal(t, interval, policy.Interval)
	assert.Equal(t, defaultMaxRetries, policy.MaxRetries)
}

func TestLinearBackoffPolicy_ComputeNextInterval(t *testing.T) {
	t.Run("BasicLinearBackoff", func(t *testing.T) {
		policy := &LinearBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			Increment:       50 * time.Millisecond,
			MaxInterval:     500 * time.Millisecond,
			MaxRetries:      5,
		}

		testCases := []struct {
			retryCount       int
			expectedInterval time.Duration
			expectError      bool
		}{
			{0, 100 * time.Millisecond, false}, // 100ms
			{1, 150 * time.Millisecond, false}, // 100ms + 50ms
			{2, 200 * time.Millisecond, false}, // 100ms + 100ms
			{3, 250 * time.Millisecond, false}, // 100ms + 150ms
			{4, 300 * time.Millisecond, false}, // 100ms + 200ms
			{5, 0, true},                       // Max retries reached
		}

		for _, tc := range testCases {
			interval, err := policy.ComputeNextInterval(tc.retryCount, 0, nil)
			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrRetriesExhausted, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedInterval, interval)
			}
		}
	})

	t.Run("MaxIntervalCapping", func(t *testing.T) {
		policy := &LinearBackoffPolicy{
			InitialInterval: 100 * time.Millisecond,
			Increment:       100 * time.Millisecond,
			MaxInterval:     300 * time.Millisecond,
			MaxRetries:      10,
		}

		testCases := []struct {
			retryCount       int
			expectedInterval time.Duration
		}{
			{0, 100 * time.Millisecond},
			{1, 200 * time.Millisecond},
			{2, 300 * time.Millisecond}, // Capped at MaxInterval
			{3, 300 * time.Millisecond}, // Still capped
			{4, 300 * time.Millisecond}, // Still capped
		}

		for _, tc := range testCases {
			interval, err := policy.ComputeNextInterval(tc.retryCount, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedInterval, interval)
		}
	})

	t.Run("ZeroIncrement", func(t *testing.T) {
		policy := &LinearBackoffPolicy{
			InitialInterval: 200 * time.Millisecond,
			Increment:       0,
			MaxInterval:     1 * time.Second,
			MaxRetries:      5,
		}

		// Should behave like constant backoff
		for i := range 5 {
			interval, err := policy.ComputeNextInterval(i, 0, nil)
			require.NoError(t, err)
			assert.Equal(t, 200*time.Millisecond, interval)
		}
	})
}

func TestNewLinearBackoffPolicy(t *testing.T) {
	initialInterval := 100 * time.Millisecond
	increment := 50 * time.Millisecond
	policy := NewLinearBackoffPolicy(initialInterval, increment)

	assert.Equal(t, initialInterval, policy.InitialInterval)
	assert.Equal(t, increment, policy.Increment)
	assert.Equal(t, defaultMaxInterval, policy.MaxInterval)
	assert.Equal(t, defaultMaxRetries, policy.MaxRetries)
}

func TestRetrier_Reset(t *testing.T) {
	t.Run("ResetAfterRetries", func(t *testing.T) {
		// Create a policy with short intervals for testing
		policy := &ExponentialBackoffPolicy{
			InitialInterval: 10 * time.Millisecond,
			BackoffFactor:   2.0,
			MaxInterval:     100 * time.Millisecond,
			MaxRetries:      0,
		}

		retrier := NewRetrier(policy)

		// Perform a few retries
		for i := range 3 {
			_, err := retrier.Next(errors.New("test error"))
			if err != nil {
				t.Fatalf("unexpected error on retry %d: %v", i, err)
			}
		}

		// Reset the retrier
		retrier.Reset()

		// After reset, the next retry should use the initial interval
		interval, err := retrier.Next(errors.New("test error"))
		if err != nil {
			t.Fatalf("unexpected error after reset: %v", err)
		}

		// Check that the interval is the initial interval
		assert.Equal(t, 10*time.Millisecond, interval)
	})

	t.Run("ResetWithMaxRetries", func(t *testing.T) {
		// Create a policy with max retries
		policy := &ConstantBackoffPolicy{
			Interval:   10 * time.Millisecond,
			MaxRetries: 2,
		}

		retrier := NewRetrier(policy)

		// Exhaust retries
		for i := range 2 {
			_, err := retrier.Next(errors.New("test error"))
			if err != nil {
				t.Fatalf("unexpected error on retry %d: %v", i, err)
			}
		}

		// Next retry should fail
		_, err := retrier.Next(errors.New("test error"))
		if err != ErrRetriesExhausted {
			t.Errorf("expected ErrRetriesExhausted, got %v", err)
		}

		// Reset the retrier
		retrier.Reset()

		// After reset, retries should work again
		for i := range 2 {
			_, err := retrier.Next(errors.New("test error"))
			if err != nil {
				t.Fatalf("unexpected error on retry %d after reset: %v", i, err)
			}
		}
	})
}

func TestRetrier_WithDifferentPolicies(t *testing.T) {
	t.Run("ConstantBackoff", func(t *testing.T) {
		policy := NewConstantBackoffPolicy(50 * time.Millisecond)
		policy.MaxRetries = 3

		retrier := NewRetrier(policy)

		// Each retry should return the same interval
		for range 3 {
			interval, err := retrier.Next(nil)
			assert.NoError(t, err)
			assert.Equal(t, 50*time.Millisecond, interval)
		}

		// Next retry should fail
		_, err := retrier.Next(nil)
		assert.Equal(t, ErrRetriesExhausted, err)
	})

	t.Run("LinearBackoff", func(t *testing.T) {
		policy := NewLinearBackoffPolicy(50*time.Millisecond, 30*time.Millisecond)
		policy.MaxRetries = 3

		retrier := NewRetrier(policy)

		expectedIntervals := []time.Duration{
			50 * time.Millisecond,  // Initial
			80 * time.Millisecond,  // +30ms
			110 * time.Millisecond, // +60ms
		}

		for i, expected := range expectedIntervals {
			interval, err := retrier.Next(nil)
			assert.NoError(t, err, "Retry %d failed", i)
			assert.Equal(t, expected, interval)
		}

		// Next retry should fail
		_, err := retrier.Next(nil)
		assert.Equal(t, ErrRetriesExhausted, err)
	})
}
