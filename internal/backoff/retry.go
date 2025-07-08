package backoff

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// Inspired by the code from Temporal's retry policy implementation (License: MIT License).
// https://github.com/temporalio/temporal/blob/2a1044994085bffbeeee789cad52ecf2650c501c/common/backoff/retrypolicy.go

var (
	// ErrRetriesExhausted is returned when the maximum number of retries has been reached.
	ErrRetriesExhausted = errors.New("retries exhausted")
	// ErrOperationCanceled is returned when the retry operation is canceled via context.
	ErrOperationCanceled = errors.New("operation canceled")
)

type (
	// RetryPolicy defines the interface for retry policies.
	RetryPolicy interface {
		// ComputeNextInterval computes the next interval based on the retry policy.
		// Returns the duration to wait before the next retry, or an error if no more retries should be attempted.
		ComputeNextInterval(retryCount int, elapsedTime time.Duration, err error) (time.Duration, error)
	}

	// Retrier manages the state of retry operations.
	Retrier interface {
		// Next waits for the next retry interval or returns an error if retries are exhausted.
		// It blocks until the interval has passed or the context is canceled.
		Next(ctx context.Context, err error) error
		// Reset resets the retrier to its initial state.
		Reset()
	}
)

var (
	noMaximumAttempts = 0 // Special value indicating no maximum attempts

	defaultBackoffFactor = 2.0
	defaultMaxInterval   = 10 * time.Second
	defaultMaxRetries    = noMaximumAttempts
)

// NewExponentialBackoffPolicy creates a new ExponentialBackoffPolicy with the specified parameters.
func NewExponentialBackoffPolicy(initialInterval time.Duration) *ExponentialBackoffPolicy {
	return &ExponentialBackoffPolicy{
		InitialInterval: initialInterval,
		BackoffFactor:   defaultBackoffFactor,
		MaxInterval:     defaultMaxInterval,
		MaxRetries:      defaultMaxRetries,
	}
}

// ExponentialBackoffPolicy is a retry policy that implements exponential backoff.
type ExponentialBackoffPolicy struct {
	// InitialInterval is the initial interval before the first retry.
	InitialInterval time.Duration `json:"initialInterval,omitempty"`
	// BackoffFactor is the factor by which the interval increases after each retry.
	BackoffFactor float64 `json:"backoffFactor,omitempty"`
	// MaxInterval is the maximum interval cap for exponential backoff.
	MaxInterval time.Duration `json:"maxInterval,omitempty"`
	// MaxRetries is the maximum number of retries allowed. 0 means unlimited retries.
	MaxRetries int `json:"maxRetries,omitempty"`
}

// ComputeNextInterval computes the next retry interval using exponential backoff.
func (p *ExponentialBackoffPolicy) ComputeNextInterval(retryCount int, elapsedTime time.Duration, err error) (time.Duration, error) {
	// Check if max retries is reached
	if p.MaxRetries > 0 && retryCount >= p.MaxRetries {
		return 0, ErrRetriesExhausted
	}

	// Calculate the interval using exponential backoff
	interval := float64(p.InitialInterval) * math.Pow(p.BackoffFactor, float64(retryCount))

	// Cap the interval at MaxInterval
	if interval > float64(p.MaxInterval) {
		interval = float64(p.MaxInterval)
	}

	return time.Duration(interval), nil
}

// ConstantBackoffPolicy is a retry policy that uses a constant interval between retries.
type ConstantBackoffPolicy struct {
	// Interval is the constant interval between retries.
	Interval time.Duration `json:"interval,omitempty"`
	// MaxRetries is the maximum number of retries allowed. 0 means unlimited retries.
	MaxRetries int `json:"maxRetries,omitempty"`
}

// NewConstantBackoffPolicy creates a new ConstantBackoffPolicy with the specified interval.
func NewConstantBackoffPolicy(interval time.Duration) *ConstantBackoffPolicy {
	return &ConstantBackoffPolicy{
		Interval:   interval,
		MaxRetries: defaultMaxRetries,
	}
}

// ComputeNextInterval returns a constant interval for each retry.
func (p *ConstantBackoffPolicy) ComputeNextInterval(retryCount int, elapsedTime time.Duration, err error) (time.Duration, error) {
	// Check if max retries is reached
	if p.MaxRetries > 0 && retryCount >= p.MaxRetries {
		return 0, ErrRetriesExhausted
	}

	return p.Interval, nil
}

// LinearBackoffPolicy is a retry policy that increases the interval linearly.
type LinearBackoffPolicy struct {
	// InitialInterval is the initial interval before the first retry.
	InitialInterval time.Duration `json:"initialInterval,omitempty"`
	// Increment is the amount by which the interval increases after each retry.
	Increment time.Duration `json:"increment,omitempty"`
	// MaxInterval is the maximum interval cap.
	MaxInterval time.Duration `json:"maxInterval,omitempty"`
	// MaxRetries is the maximum number of retries allowed. 0 means unlimited retries.
	MaxRetries int `json:"maxRetries,omitempty"`
}

// NewLinearBackoffPolicy creates a new LinearBackoffPolicy with the specified parameters.
func NewLinearBackoffPolicy(initialInterval, increment time.Duration) *LinearBackoffPolicy {
	return &LinearBackoffPolicy{
		InitialInterval: initialInterval,
		Increment:       increment,
		MaxInterval:     defaultMaxInterval,
		MaxRetries:      defaultMaxRetries,
	}
}

// ComputeNextInterval computes the next retry interval using linear backoff.
func (p *LinearBackoffPolicy) ComputeNextInterval(retryCount int, elapsedTime time.Duration, err error) (time.Duration, error) {
	// Check if max retries is reached
	if p.MaxRetries > 0 && retryCount >= p.MaxRetries {
		return 0, ErrRetriesExhausted
	}

	// Calculate the interval using linear backoff
	interval := p.InitialInterval + (time.Duration(retryCount) * p.Increment)

	// Cap the interval at MaxInterval
	if interval > p.MaxInterval {
		interval = p.MaxInterval
	}

	return interval, nil
}

// NewRetrier creates a new Retrier instance with the specified retry policy.
func NewRetrier(retryPolicy RetryPolicy) Retrier {
	return &retrierImpl{
		retryPolicy: retryPolicy,
		retryCount:  0,
	}
}

type retrierImpl struct {
	retryPolicy RetryPolicy
	retryCount  int
	startTime   time.Time
	mu          sync.Mutex
}

// Next implements Retrier interface.
// It waits for the next retry interval or returns an error if retries are exhausted.
func (r *retrierImpl) Next(ctx context.Context, err error) error {
	r.mu.Lock()
	// Initialize start time on first call
	if r.startTime.IsZero() {
		r.startTime = time.Now()
	}

	// Compute elapsed time
	elapsedTime := time.Since(r.startTime)

	// Compute next interval
	interval, computeErr := r.retryPolicy.ComputeNextInterval(r.retryCount, elapsedTime, err)
	if computeErr != nil {
		r.mu.Unlock()
		return computeErr
	}

	// Increment retry count for next iteration
	r.retryCount++
	r.mu.Unlock()

	// Create a timer for the interval
	timer := time.NewTimer(interval)
	defer timer.Stop()

	// Wait for either the timer to expire or context to be canceled
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ErrOperationCanceled
	}
}

// Reset resets the retrier to its initial state.
func (r *retrierImpl) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retryCount = 0
	r.startTime = time.Time{}
}
