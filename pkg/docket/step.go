package docket

import (
	"math"
	"time"
)

// StepConfig holds configuration applied at registration time.
type StepConfig struct {
	// Name is a human-readable identifier (defaults to function name)
	Name string

	// RetryConfig specifies retry behavior
	RetryConfig *RetryConfig

	// Timeout is the per-execution timeout for this step
	Timeout time.Duration

	// MaxConcurrency limits the number of concurrent executions for this step
	// (Relevant for batch processing / fan-out)
	// 0 means unlimited (bounded by batch size)
	MaxConcurrency int

	// ErrorClassifier determines if an error should trigger a retry
	// Returns true if the error is retriable
	// If nil, all errors are considered retriable
	ErrorClassifier func(error) bool

	// Persistence options

	// Store is the persistence layer for this step.
	// If nil, no persistence/caching is performed.
	Store Store

	// PersistenceScope defines whether to cache globally or per-workflow.
	PersistenceScope PersistenceScope

	// TTL defines how long the result is valid. 0 means forever.
	TTL time.Duration
}

// RetryConfig specifies retry behavior for a step.
type RetryConfig struct {
	// MaxAttempts is the maximum number of execution attempts (must be >= 1)
	MaxAttempts int

	// Backoff determines wait time between retries
	Backoff BackoffStrategy
}

// BackoffStrategy determines wait time between retry attempts.
type BackoffStrategy interface {
	// NextDelay returns the duration to wait before the next attempt
	// attempt: The attempt number (1 for first retry, 2 for second, etc.)
	NextDelay(attempt int) time.Duration
}

// FixedBackoff implements a constant wait time between retries.
type FixedBackoff struct {
	Delay time.Duration
}

func (b FixedBackoff) NextDelay(attempt int) time.Duration {
	return b.Delay
}

// ExponentialBackoff implements an exponentially increasing wait time.
type ExponentialBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Factor       float64
}

func (b ExponentialBackoff) NextDelay(attempt int) time.Duration {
	delay := float64(b.InitialDelay) * math.Pow(b.Factor, float64(attempt-1))
	if delay > float64(b.MaxDelay) {
		return b.MaxDelay
	}
	return time.Duration(delay)
}
