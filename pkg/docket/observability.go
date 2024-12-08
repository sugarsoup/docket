package docket

import (
	"context"
	"time"
)

// Observer is the interface for observing graph execution events.
// Implementations can emit metrics, logs, or traces to their observability backend.
//
// All Observer methods are called synchronously during execution, so implementations
// should be fast and non-blocking. For expensive operations (e.g., network calls),
// consider buffering events and processing them asynchronously.
//
// Example implementations:
//   - Prometheus metrics collector
//   - OpenTelemetry tracer
//   - Structured logger (zap, zerolog)
//   - Custom metrics aggregator
type Observer interface {
	// OnGraphStart is called when graph execution begins.
	OnGraphStart(ctx context.Context, event *GraphStartEvent)

	// OnGraphEnd is called when graph execution completes (success or failure).
	OnGraphEnd(ctx context.Context, event *GraphEndEvent)

	// OnStepStart is called when a step begins execution.
	OnStepStart(ctx context.Context, event *StepStartEvent)

	// OnStepEnd is called when a step completes (success or failure).
	OnStepEnd(ctx context.Context, event *StepEndEvent)

	// OnCacheCheck is called when checking persistence store for cached result.
	OnCacheCheck(ctx context.Context, event *CacheCheckEvent)

	// OnRetry is called when a step is retried after failure.
	OnRetry(ctx context.Context, event *RetryEvent)
}

// GraphStartEvent is emitted when graph execution begins.
type GraphStartEvent struct {
	ExecutionID string
	OutputType  string // Human-readable type name
	StartTime   time.Time
}

// GraphEndEvent is emitted when graph execution completes.
type GraphEndEvent struct {
	ExecutionID string
	OutputType  string
	Duration    time.Duration
	Error       error // nil if successful
}

// StepStartEvent is emitted when a step begins execution.
type StepStartEvent struct {
	ExecutionID string
	StepName    string
	StepType    string // Output type of the step
	Attempt     int    // 1 for first attempt, 2+ for retries
	StartTime   time.Time
}

// StepEndEvent is emitted when a step completes execution.
type StepEndEvent struct {
	ExecutionID string
	StepName    string
	StepType    string
	Attempt     int
	Duration    time.Duration
	Error       error  // nil if successful
	Panicked    bool   // true if step panicked
	Cached      bool   // true if result was saved to cache
}

// CacheCheckEvent is emitted when checking the persistence store.
type CacheCheckEvent struct {
	ExecutionID string
	StepName    string
	StepType    string
	Hit         bool          // true if cache hit, false if miss
	Latency     time.Duration // Time spent checking cache
	Error       error         // nil if check was successful
}

// RetryEvent is emitted when a step is retried after failure.
type RetryEvent struct {
	ExecutionID string
	StepName    string
	StepType    string
	Attempt     int           // The attempt number that failed (before retry)
	Error       error         // The error that triggered the retry
	Delay       time.Duration // How long we're waiting before retry
}

// NoOpObserver is a no-op implementation of Observer.
// Useful as a base for partial implementations.
type NoOpObserver struct{}

func (NoOpObserver) OnGraphStart(ctx context.Context, event *GraphStartEvent)   {}
func (NoOpObserver) OnGraphEnd(ctx context.Context, event *GraphEndEvent)       {}
func (NoOpObserver) OnStepStart(ctx context.Context, event *StepStartEvent)     {}
func (NoOpObserver) OnStepEnd(ctx context.Context, event *StepEndEvent)         {}
func (NoOpObserver) OnCacheCheck(ctx context.Context, event *CacheCheckEvent)   {}
func (NoOpObserver) OnRetry(ctx context.Context, event *RetryEvent)             {}

// MultiObserver combines multiple observers into one.
// Events are sent to all observers in order.
type MultiObserver struct {
	Observers []Observer
}

func (m *MultiObserver) OnGraphStart(ctx context.Context, event *GraphStartEvent) {
	for _, obs := range m.Observers {
		obs.OnGraphStart(ctx, event)
	}
}

func (m *MultiObserver) OnGraphEnd(ctx context.Context, event *GraphEndEvent) {
	for _, obs := range m.Observers {
		obs.OnGraphEnd(ctx, event)
	}
}

func (m *MultiObserver) OnStepStart(ctx context.Context, event *StepStartEvent) {
	for _, obs := range m.Observers {
		obs.OnStepStart(ctx, event)
	}
}

func (m *MultiObserver) OnStepEnd(ctx context.Context, event *StepEndEvent) {
	for _, obs := range m.Observers {
		obs.OnStepEnd(ctx, event)
	}
}

func (m *MultiObserver) OnCacheCheck(ctx context.Context, event *CacheCheckEvent) {
	for _, obs := range m.Observers {
		obs.OnCacheCheck(ctx, event)
	}
}

func (m *MultiObserver) OnRetry(ctx context.Context, event *RetryEvent) {
	for _, obs := range m.Observers {
		obs.OnRetry(ctx, event)
	}
}
