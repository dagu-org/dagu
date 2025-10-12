package metrics

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestStartUptime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Reset package variables before test
		startTime = time.Time{}
		uptime.Store(0)

		// Create a context with cancel to stop the goroutine
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start uptime tracking
		StartUptime(ctx)

		// Verify that startTime was set
		if startTime.IsZero() {
			t.Error("startTime was not initialized")
		}

		// Wait for at least 2 seconds to ensure multiple updates
		time.Sleep(2 * time.Second)
		synctest.Wait()

		// Get the uptime value
		currentUptime := GetUptime()

		// Check if uptime is in the expected range (between 1 and 3 seconds)
		if currentUptime < 1 || currentUptime > 3 {
			t.Errorf("unexpected uptime value: got %d, want between 1 and 3", currentUptime)
		}

		// Test goroutine cleanup
		cancel()
		previousUptime := GetUptime()
		time.Sleep(2 * time.Second)
		synctest.Wait()

		// After cancellation, the uptime should remain relatively unchanged
		currentUptime = GetUptime()
		if currentUptime < previousUptime || currentUptime > previousUptime+1 {
			t.Errorf("uptime continued to increase after context cancellation: previous=%d, current=%d",
				previousUptime, currentUptime)
		}
	})
}

func TestGetUptime(t *testing.T) {
	// Reset package variables
	startTime = time.Time{}
	uptime.Store(42)

	// Test getting the stored value
	if got := GetUptime(); got != 42 {
		t.Errorf("GetUptime() = %d; want 42", got)
	}
}

func TestUptimeAccuracy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Reset package variables
		startTime = time.Time{}
		uptime.Store(0)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start uptime tracking
		StartUptime(ctx)

		// Wait for 3.5 seconds
		time.Sleep(3500 * time.Millisecond)
		synctest.Wait()

		// Get uptime
		currentUptime := GetUptime()

		// Check if uptime is approximately 3 seconds (allowing for some execution time variance)
		if currentUptime < 3 || currentUptime > 4 {
			t.Errorf("uptime accuracy test failed: got %d seconds, want approximately 3", currentUptime)
		}
	})
}
