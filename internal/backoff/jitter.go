package backoff

import (
	"math/rand"
	"sync"
	"time"
)

// JitterType defines the type of jitter to apply to backoff intervals.
type JitterType int

const (
	// NoJitter applies no jitter to the interval.
	NoJitter JitterType = iota
	// FullJitter applies jitter between 0 and the interval.
	FullJitter
	// Jitter applies jitter of ±50% of the interval.
	Jitter
)

// JitterFunc defines a function that applies jitter to a backoff interval.
type JitterFunc func(interval time.Duration) time.Duration

// jitterImpl implements jitter strategies.
type jitterImpl struct {
	jitterType JitterType
	randSource *rand.Rand
	mu         sync.Mutex
}

// NewJitterFunc creates a new jitter function with the specified type.
func NewJitterFunc(jitterType JitterType) JitterFunc {
	j := &jitterImpl{
		jitterType: jitterType,
		// nolint:gosec // G404: Use of weak RNG is acceptable for jitter calculation
		randSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return j.Apply
}

// Apply adds jitter to the given interval based on the jitter type.
func (j *jitterImpl) Apply(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	switch j.jitterType {
	case NoJitter:
		return interval
	case FullJitter:
		// Return a random value between 0 and interval
		return time.Duration(j.randSource.Int63n(int64(interval) + 1))
	case Jitter:
		// Return interval ± 50%
		// This gives a range of [0.5 * interval, 1.5 * interval]
		half := interval / 2
		jitter := time.Duration(j.randSource.Int63n(int64(interval) + 1))
		return half + jitter
	default:
		return interval
	}
}

// WithJitter returns a new RetryPolicy that applies jitter to the base policy's intervals.
func WithJitter(basePolicy RetryPolicy, jitterType JitterType) RetryPolicy {
	jitterFunc := NewJitterFunc(jitterType)
	return &jitteredPolicy{
		basePolicy: basePolicy,
		jitterFunc: jitterFunc,
	}
}

// jitteredPolicy wraps a RetryPolicy and applies jitter to its intervals.
type jitteredPolicy struct {
	basePolicy RetryPolicy
	jitterFunc JitterFunc
}

// ComputeNextInterval computes the next interval with jitter applied.
func (p *jitteredPolicy) ComputeNextInterval(retryCount int, elapsedTime time.Duration, err error) (time.Duration, error) {
	// Get the base interval from the underlying policy
	baseInterval, baseErr := p.basePolicy.ComputeNextInterval(retryCount, elapsedTime, err)
	if baseErr != nil {
		return 0, baseErr
	}

	// Apply jitter to the base interval
	return p.jitterFunc(baseInterval), nil
}
