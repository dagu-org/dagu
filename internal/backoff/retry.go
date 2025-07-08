package backoff

import (
	"context"
	"time"
)

// Heavily inspired by the code from Temporal's retry policy implementation.
// https://github.com/temporalio/temporal/blob/2a1044994085bffbeeee789cad52ecf2650c501c/common/backoff/retrypolicy.go

type (
	RetryPolicy interface {
		// ComputeNextInterval computes the next interval based on the retry policy.
		ComputeNextInterval(retryCount, numAttempts int, err error) error
	}

	// Retrier manages the state of retry operations.
	Retrier interface {
		Next(ctx context.Context, err error)
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
	// MaxRetries is the maximum number of retries allowed.
	MaxRetries int `json:"maxRetries,omitempty"`
}

// NewRetrier creates a new Retrier instance with the specified retry policy.
func NewRetrier(retryPolicy RetryPolicy) Retrier {
	return &retrierImpl{
		retryPolicy: retryPolicy,
		retryCount:  0,
		numAttempts: 0,
	}
}

type retrierImpl struct {
	retryPolicy RetryPolicy
	retryCount  int
	numAttempts int
}

// Next implements Retrier.
func (r *retrierImpl) Next(ctx context.Context, err error) {
	// TODO: Implement the logic to compute the next retry interval
}
