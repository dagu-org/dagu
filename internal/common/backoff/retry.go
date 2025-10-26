package backoff

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
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
	if isRetriable == nil {
		isRetriable = func(_ error) bool { return true }
	}

	retrier := NewRetrier(policy)
	attempt := 0
	var lastDebugLog time.Time

	for {
		attempt++

		if err := ctx.Err(); err != nil {
			logger.Warn(ctx, "Retry aborted due to context error", "attempt", attempt, "err", err)
			return err
		}

		err := op(ctx)
		if err == nil {
			logSuccessIfNeeded(ctx, attempt, &lastDebugLog)
			return nil
		}

		if !isRetriable(err) {
			logger.Warn(ctx, "Retryable operation failed with non-retriable error", "attempt", attempt, "err", err)
			return err
		}

		interval, retryErr := retrier.Next(err)
		if retryErr != nil {
			logger.Warn(ctx, "Retry attempts exhausted", "attempt", attempt, "err", err)
			return err
		}

		if interval <= 0 {
			interval = time.Millisecond * 100 // Default small delay
		}

		logRetryIfNeeded(ctx, attempt, interval, err, &lastDebugLog)

		if err := waitWithContext(ctx, interval, attempt); err != nil {
			return err
		}
	}
}

func logSuccessIfNeeded(ctx context.Context, attempt int, lastDebugLog *time.Time) {
	if attempt > 1 && time.Since(*lastDebugLog) >= 30*time.Second {
		logger.Debug(ctx, "Retryable operation succeeded", "attempt", attempt)
		*lastDebugLog = time.Now()
	}
}

func logRetryIfNeeded(ctx context.Context, attempt int, interval time.Duration, err error, lastDebugLog *time.Time) {
	if time.Since(*lastDebugLog) >= 30*time.Second {
		logger.Debug(ctx, "Retryable operation failed; scheduling retry",
			"attempt", attempt,
			"next_attempt_in", interval,
			"err", err,
		)
		*lastDebugLog = time.Now()
	}
}

func waitWithContext(ctx context.Context, interval time.Duration, attempt int) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		logger.Warn(ctx, "Retry aborted during backoff wait", "attempt", attempt, "err", ctx.Err())
		return ctx.Err()
	}
}
