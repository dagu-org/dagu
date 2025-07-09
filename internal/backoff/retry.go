package backoff

import (
	"context"
	"time"
)

type (
	// Operation to retry
	Operation func(ctx context.Context) error

	// IsRetriableFunc defines a function that checks if an error is retriable.
	IsRetriableFunc func(err error) bool
)

// Retry executes the operation with retry logic based on the provided policy.
// If isRetriable is nil, all errors are considered retriable.
func Retry(ctx context.Context, op Operation, policy RetryPolicy, isRetriable IsRetriableFunc) error {
	// Default to retrying all errors if no function provided
	if isRetriable == nil {
		isRetriable = func(_ error) bool { return true }
	}

	retrier := NewRetrier(policy)

	for {
		// Check context before operation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Execute the operation
		err := op(ctx)
		if err == nil {
			return nil
		}

		// Check if error is retriable
		if !isRetriable(err) {
			return err
		}

		// Get next retry interval
		interval, retryErr := retrier.Next(err)
		if retryErr != nil {
			// If retries exhausted, return the original operation error
			return err
		}

		// Wait for the interval
		if interval > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-timer.C:
				// Continue to next iteration
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
			timer.Stop()
		}
	}
}
