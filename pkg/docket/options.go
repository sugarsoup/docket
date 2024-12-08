package docket

import "time"

// StepOption is a functional option for configuring a graph step.
type StepOption interface {
	apply(*StepConfig)
}

type optionFunc func(*StepConfig)

func (f optionFunc) apply(c *StepConfig) {
	f(c)
}

// WithName sets a custom name for the step (useful for debugging and introspection).
func WithName(name string) StepOption {
	return optionFunc(func(c *StepConfig) {
		c.Name = name
	})
}

// WithTimeout sets a timeout for the step execution.
func WithTimeout(timeout time.Duration) StepOption {
	return optionFunc(func(c *StepConfig) {
		c.Timeout = timeout
	})
}

// WithMaxConcurrency limits the number of concurrent executions for this step.
// Useful for rate-limiting calls to external services during batch processing.
// Default is 100. Set to -1 for unlimited.
func WithMaxConcurrency(n int) StepOption {
	return optionFunc(func(c *StepConfig) {
		c.MaxConcurrency = n
	})
}

// WithRetry sets a retry policy for the step.
func WithRetry(config RetryConfig) StepOption {
	return optionFunc(func(c *StepConfig) {
		c.RetryConfig = &config
	})
}

// WithErrorClassifier sets a function to determine which errors should trigger a retry.
// If not provided, all errors are considered retriable.
func WithErrorClassifier(classifier func(error) bool) StepOption {
	return optionFunc(func(c *StepConfig) {
		c.ErrorClassifier = classifier
	})
}

// WithPersistence configures persistence for the step.
//
//	store: The persistence layer implementation (e.g. SQLiteStore, InMemoryStore)
//	scope: ScopeWorkflow (local to execution) or ScopeGlobal (shared)
//	ttl: How long the result should be valid (0 = forever)
//
// Example:
//
//	g.Register(fn, WithPersistence(myStore, ScopeGlobal, 24*time.Hour))
func WithPersistence(store Store, scope PersistenceScope, ttl time.Duration) StepOption {
	return optionFunc(func(c *StepConfig) {
		c.Store = store
		c.PersistenceScope = scope
		c.TTL = ttl
	})
}
