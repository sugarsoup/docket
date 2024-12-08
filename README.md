# Docket

**Docket** is a type-safe, proto-native workflow engine for Go. It turns your Protocol Buffers into a self-assembling execution graph, handling dependency resolution, concurrency, and persistence automatically.

> **Philosophy**: The data types *are* the graph. You define functions that transform Protos, and Docket figures out how to wire them together.

## 🚀 Features

- **Auto-Wiring**: Dependencies are inferred directly from function signatures. No manual DAG construction.
- **Implicit Parallelism**: Independent steps are executed concurrently without extra code.
- **Type Safety**: Built entirely around Protocol Buffers. Compile-time checks for your data structures, runtime checks for graph validity.
- **Batch Processing**: First-class support for "Fan-Out/Fan-In" patterns to process collections efficiently.
- **Resilience**: Configurable **retries** (exponential backoff), **timeouts**, and error handling policies per step.
- **Safety**: Automatic panic recovery prevents crashes from bringing down the app. Unbounded concurrency protection limits goroutines during batch processing.
- **Smart Persistence**: Built-in caching/memoization with **strict input hashing**. Supports SQLite, Postgres, and In-Memory stores.
- **Observability**: Steps are named and traceable.

## 📦 Installation

```bash
go get github.com/sugarsoup/docket
```

## ⚡ Quick Start

### 1. Define your Protos
Imagine you want to fetch a user and enrich their profile.

```protobuf
message UserID { string id = 1; }
message UserProfile { string name = 1; string email = 2; }
message UserPreferences { bool dark_mode = 1; }
message EnrichedUser { UserProfile profile = 1; UserPreferences prefs = 2; }
```

### 2. Register Steps
Create a graph and register functions. Docket inspects the arguments to determine dependencies.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sugarsoup/docket/pkg/docket"
	pb "myapp/proto" // Your generated protos
)

func main() {
	// Create graph with optional default timeout
	g := docket.NewGraph(docket.WithDefaultTimeout(30 * time.Second))

	// Step 1: Fetch Profile (Depends on UserID)
	g.Register(func(ctx context.Context, id *pb.UserID) (*pb.UserProfile, error) {
		return &pb.UserProfile{Name: "Alice", Email: "alice@example.com"}, nil
	})

	// Step 2: Fetch Preferences (Depends on UserID)
	// Runs in PARALLEL with Step 1!
	g.Register(func(ctx context.Context, id *pb.UserID) (*pb.UserPreferences, error) {
		time.Sleep(100 * time.Millisecond) // Simulate IO
		return &pb.UserPreferences{DarkMode: true}, nil
	})

	// Step 3: Combine (Depends on Profile AND Preferences)
	// Waits for both Step 1 and 2 to complete.
	g.Register(func(ctx context.Context, p *pb.UserProfile, prefs *pb.UserPreferences) (*pb.EnrichedUser, error) {
		return &pb.EnrichedUser{Profile: p, Prefs: prefs}, nil
	})

	// Validate the graph structure
	if err := g.Validate(); err != nil {
		panic(err)
	}

	// Execute!
	// We request an *EnrichedUser. The graph works backward to find the path.
	ctx := context.Background()
	input := &pb.UserID{Id: "user-123"}

	result, err := docket.Execute[*pb.EnrichedUser](ctx, g, "request-id-1", input)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Result: %+v\n", result)
}
```

## 🧠 Core Concepts

### Dependency Injection
Dependencies are resolved by type. If a function requires `*pb.UserProfile`, Docket finds the registered step that returns `*pb.UserProfile` and executes it first.

### Parallelism
In the example above, `Fetch Profile` and `Fetch Preferences` both depend only on `UserID`. Since `UserID` is provided as input, both steps execute **simultaneously**. You don't need `go` routines or `WaitGroups`; the graph handles it.

### Batch Processing
Process lists of items efficiently. `ExecuteBatch` runs per-item steps in parallel and supports "Aggregate" steps that see the whole batch.

```go
// Aggregate Step: Takes a slice of inputs, returns a single output
g.RegisterAggregate(func(ctx context.Context, inputs []*pb.Video) (*pb.Summary, error) {
	return &pb.Summary{Count: int32(len(inputs))}, nil
})

// Limit concurrency to avoid overloading downstream services
g.Register(
    EnrichVideoStep,
    docket.WithMaxConcurrency(50), // Process at most 50 items at once
)
```

See `examples/batch` for a complete movie processing pipeline.

### Persistence & Caching
Cache expensive steps automatically using SQLite, Postgres, or Memory.

```go
// Initialize Store
store := docket.NewSQLStore(db, "step_cache", docket.DialectPostgres)
store.InitSchema(ctx)

// Register with Persistence
g.Register(
	HeavyComputationStep,
	docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
)
```
Keys are generated using **SHA-256 hashes of the inputs**, ensuring that if your data changes, the cache invalidates automatically. See `examples/persistence`.

### Retries & Timeouts
Configure robustness policies per-step:

```go
g.Register(
	FlakyNetworkCall,
	docket.WithTimeout(5*time.Second),
	docket.WithRetry(docket.RetryConfig{
		MaxAttempts: 3,
		Backoff:     docket.ExponentialBackoff{InitialDelay: 100*time.Millisecond, Factor: 2},
	}),
)
```

### Error Handling

Docket provides fine-grained control over error semantics:

**Stop a single step** (don't retry this step):
```go
// Define custom error types
type ValidationError struct { Field string }
func (e *ValidationError) Error() string { return "validation failed: " + e.Field }

// Configure step to not retry validation errors
g.Register(
	ProcessOrder,
	docket.WithErrorClassifier(func(err error) bool {
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			return false // Don't retry validation failures
		}
		return true // Retry everything else
	}),
)
```

**Stop the whole graph** (cancel all parallel work):
```go
// Return an AbortError to stop graph execution immediately
func CriticalStep(ctx context.Context, input *pb.Input) (*pb.Output, error) {
	if isCriticalFailure() {
		return nil, docket.NewAbortError(errors.New("critical system failure"))
	}
	return &pb.Output{}, nil
}
```

**Context cancellation** propagates through the entire graph:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := docket.Execute[*pb.Output](ctx, g, "exec-1", input)
if errors.Is(err, context.DeadlineExceeded) {
	// Graph timed out, all in-flight steps were cancelled
}
```

See `examples/error_handling` for complete patterns.

## 🎯 Execution Semantics

Docket provides different execution guarantees depending on your configuration. Understanding these semantics helps you choose the right approach for your use case.

### At Most Once (Default)

**Without persistence**, each step executes **at most once per execution**. If a step fails and retries are exhausted, the execution fails and the step does not complete.

```go
g := docket.NewGraph() // No persistence
g.Register(ExpensiveComputation)

// Step runs once. If it fails, execution fails.
result, err := docket.Execute[*pb.Output](ctx, g, "exec-1", input)
```

**Guarantees**:
- Step executes 0 times (if dependency fails) or 1 time (success or final retry failure)
- No cross-execution deduplication
- Fresh computation on each graph execution

**Use Cases**: Simple workflows, development/testing, stateless operations

### At Least Once (With Retries)

Configure **retry policies** to ensure steps execute until they succeed or max attempts are reached.

```go
g.Register(
    FlakyAPICall,
    docket.WithRetry(docket.RetryConfig{
        MaxAttempts: 5,
        Backoff: docket.ExponentialBackoff{
            InitialDelay: 100*time.Millisecond,
            Factor: 2,
        },
    }),
)

// Step retries on failure up to 5 times
result, err := docket.Execute[*pb.Output](ctx, g, "exec-1", input)
```

**Guarantees**:
- Step executes **at least once** if reachable in the graph
- Continues retrying until success or MaxAttempts reached
- Transient failures are tolerated

**Use Cases**: Network calls, external APIs, operations that can fail transiently

See `examples/retry` for retry patterns.

### Exactly Once (With Global Persistence)

Use `ScopeGlobal` persistence to guarantee a step executes **exactly once per unique input**, even across multiple independent executions.

```go
store := docket.NewMemoryStore()

g.Register(
    ExpensiveComputation,
    docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
)

// First execution: Step runs and result is cached
result1, _ := docket.Execute[*pb.Output](ctx, g, "exec-1", input)

// Second execution with SAME input: Step does NOT run (cache hit)
result2, _ := docket.Execute[*pb.Output](ctx, g, "exec-2", input)

// result1 == result2, but step only executed once
```

**Guarantees**:
- Step executes **exactly once** for each unique `(step_name, hash(inputs))` combination
- Results cached globally until TTL expires
- Provides strong idempotency guarantees

**Use Cases**: Expensive computations, billing operations, idempotent database writes, API calls you can't repeat

See `examples/exactly_once` for a complete demonstration.

### Crash Recovery (With Persistence)

When persistence is enabled, Docket **checkpoints completed steps**. On retry after failure, previously completed steps are restored from checkpoints without re-execution.

```go
store := docket.NewSQLStore(db, "checkpoints", docket.DialectSQLite)

g := docket.NewGraph(
    docket.WithPersistence(store, docket.ScopeGlobal, 10*time.Minute),
)

g.Register(ExpensiveIngestion)  // Step 1
g.Register(ProcessData)          // Step 2 (can fail)
g.Register(EnrichResults)        // Step 3

// First attempt: Step 1 succeeds, Step 2 fails
_, err := docket.Execute[*pb.EnrichedData](ctx, g, "exec-1", input)
// Error: Step 2 failed

// Retry: Step 1 restored from checkpoint (not re-executed)
//        Step 2 retries and succeeds
//        Step 3 executes for first time
result, _ := docket.Execute[*pb.EnrichedData](ctx, g, "exec-1", input)
```

**Guarantees**:
- Completed steps are checkpointed
- On retry, checkpointed steps skip execution
- Only failed steps and downstream dependencies re-execute

**Use Cases**: Long-running pipelines, expensive multi-stage ETL, workflows with unreliable steps

See `examples/crash_recovery` for a working example.

### Persistence Scopes

Docket offers two persistence scopes with different cache lifetime semantics:

#### ScopeWorkflow (Execution-Local)

Cache key includes **execution ID**: `(execution_id, step_name, input_hash)`

```go
g.Register(
    ComputeStep,
    docket.WithPersistence(store, docket.ScopeWorkflow, 1*time.Hour),
)

// Each execution gets its own cache namespace
result1, _ := docket.Execute[*pb.Output](ctx, g, "exec-1", input)  // Executes
result2, _ := docket.Execute[*pb.Output](ctx, g, "exec-2", input)  // Executes again
```

**Behavior**:
- Cache is execution-local
- Results do NOT persist across different execution IDs
- Still provides deduplication within a single execution

**Use Cases**: Temporary results, execution-specific state, when you want fresh computation each time

#### ScopeGlobal (Shared)

Cache key is **global**: `(step_name, input_hash)`

```go
g.Register(
    ComputeStep,
    docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
)

// Same inputs = cache hit, regardless of execution ID
result1, _ := docket.Execute[*pb.Output](ctx, g, "exec-1", input)  // Executes
result2, _ := docket.Execute[*pb.Output](ctx, g, "exec-2", input)  // Cache hit!
```

**Behavior**:
- Cache is shared across all executions
- Same inputs always return cached results (within TTL)
- Provides exactly-once guarantee

**Use Cases**: Expensive computations, API calls, idempotent operations, reusable results

See `examples/scope_comparison` for a side-by-side comparison.

### Choosing the Right Semantics

| Requirement | Solution |
|------------|----------|
| Simple, stateless workflow | Default (no persistence) |
| Handle transient failures | `WithRetry()` |
| Never duplicate expensive operations | `ScopeGlobal` persistence |
| Ensure billing happens exactly once | `ScopeGlobal` + idempotent step |
| Resume long pipelines after crash | `ScopeGlobal` persistence |
| Execution-specific temporary cache | `ScopeWorkflow` persistence |
| Fresh computation every time | No persistence or `ScopeWorkflow` |

## 🤝 Contributing

Contributions are welcome! Please ensure new features are covered by tests.

1. Install Protoc
2. `make proto`
3. `make test`

## License

MIT
