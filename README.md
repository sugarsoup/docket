# Docket

**Docket** is a typed execution graph for Go. You write functions with typed inputs and outputs—Docket infers the dependency graph, runs independent steps in parallel, and caches what you tell it to.

```go
func FetchProfile(ctx context.Context, id *pb.UserID) (*pb.UserProfile, error)
func FetchPrefs(ctx context.Context, id *pb.UserID) (*pb.UserPreferences, error)
func Enrich(ctx context.Context, p *pb.UserProfile, prefs *pb.UserPreferences) (*pb.EnrichedUser, error)
```

Register these three functions. Ask for `*EnrichedUser`. Docket figures out the rest—`FetchProfile` and `FetchPrefs` run in parallel (both only need `UserID`), then `Enrich` runs when both complete.

No manual DAG construction. No goroutines. No WaitGroups. The types encode the structure.

## Installation

```bash
go get github.com/sugarsoup/docket
```

## Quick Start

### 1. Define your Protos

```protobuf
message UserID { string id = 1; }
message UserProfile { string name = 1; string email = 2; }
message UserPreferences { bool dark_mode = 1; }
message EnrichedUser { UserProfile profile = 1; UserPreferences prefs = 2; }
```

### 2. Register Steps

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sugarsoup/docket/pkg/docket"
	pb "myapp/proto"
)

func FetchUserProfile(ctx context.Context, id *pb.UserID) (*pb.UserProfile, error) {
	// Fetch from database, API, etc.
	return &pb.UserProfile{Name: "Alice", Email: "alice@example.com"}, nil
}

func FetchUserPreferences(ctx context.Context, id *pb.UserID) (*pb.UserPreferences, error) {
	time.Sleep(100 * time.Millisecond) // Simulate IO
	return &pb.UserPreferences{DarkMode: true}, nil
}

func CombineUserInfo(ctx context.Context, p *pb.UserProfile, prefs *pb.UserPreferences) (*pb.EnrichedUser, error) {
	return &pb.EnrichedUser{Profile: p, Prefs: prefs}, nil
}

func main() {
	g := docket.NewGraph(docket.WithDefaultTimeout(30 * time.Second))

	// Register steps—dependencies inferred from function signatures
	g.Register(FetchUserProfile)    // *UserID → *UserProfile
	g.Register(FetchUserPreferences) // *UserID → *UserPreferences
	g.Register(CombineUserInfo)      // *UserProfile, *UserPreferences → *EnrichedUser

	if err := g.Validate(); err != nil {
		panic(err)
	}

	// Request the output type you want, provide the leaf inputs
	result, err := docket.Execute[*pb.EnrichedUser](
		context.Background(),
		g,
		"request-123",
		&pb.UserID{Id: "user-123"},
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Result: %+v\n", result)
}
```

**What happens:**
1. You ask for `*EnrichedUser`
2. Docket sees it needs `*UserProfile` and `*UserPreferences`
3. Both depend only on `*UserID` (which you provided)
4. Both fetches run **in parallel**
5. When both complete, `CombineUserInfo` runs
6. You get your result

## How It Works

Docket treats function signatures as dependency declarations. A function that takes `*UserProfile` depends on whatever produces `*UserProfile`. The graph emerges from the types.

This is similar to how `make` works—you declare targets and dependencies, and the build system figures out execution order. Docket does the same for data transformations.

| Concept | make | Docket |
|---------|------|--------|
| Target | Output file | Output proto type |
| Dependencies | Input files | Input proto types |
| Rule | Shell command | Go function |
| Cache key | File timestamp | SHA-256 of inputs |
| Incremental | Rebuild stale files | Recompute or use cache |

## Features

### Automatic Parallelism

Independent steps run concurrently without explicit coordination:

```go
// These three all depend only on *UserID
g.Register(FetchProfile)   // *UserID → *UserProfile
g.Register(FetchActivity)  // *UserID → *UserActivity
g.Register(FetchFriends)   // *UserID → *FriendsList

// This depends on all three
g.Register(BuildDashboard) // *UserProfile, *UserActivity, *FriendsList → *Dashboard
```

The three fetches run in parallel. `BuildDashboard` waits for all three automatically.

### Selective Caching

Not all steps are equal. Some are expensive; some are trivial:

```go
// Expensive API call—cache globally for 1 hour
g.Register(
	CallExpensiveAPI,
	docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
)

// Cheap validation—just recompute
g.Register(ValidateInput)
```

Cache keys are SHA-256 hashes of inputs. If inputs change, cache misses automatically.

### Retries and Timeouts

Configure resilience per-step:

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

### Batch Processing

Process collections with fan-out/fan-in:

```go
// Per-item: runs once per item, in parallel
g.Register(func(ctx context.Context, movie *pb.Movie) (*pb.EnrichedMovie, error) {
	return enrichMovie(ctx, movie)
})

// Aggregate: runs once for entire batch
g.RegisterAggregate(func(ctx context.Context, movies []*pb.Movie) (*pb.BatchStats, error) {
	return computeStats(movies), nil
})

// Execute batch with concurrency limit
results, err := docket.ExecuteBatch[*pb.EnrichedMovie](ctx, g, "batch-1", movies,
	docket.WithMaxConcurrency(50),
)
```

### Error Handling

**Don't retry specific errors:**
```go
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

**Stop the entire graph:**
```go
func CriticalStep(ctx context.Context, input *pb.Input) (*pb.Output, error) {
	if isCriticalFailure() {
		return nil, docket.NewAbortError(errors.New("critical failure"))
	}
	return &pb.Output{}, nil
}
```

## Execution Guarantees

| Configuration | Guarantee | Use Case |
|--------------|-----------|----------|
| Default | At most once | Simple stateless workflows |
| `WithRetry()` | At least once | Network calls, transient failures |
| `ScopeGlobal` persistence | Exactly once per input | Expensive computations, billing |
| `ScopeWorkflow` persistence | Exactly once per execution | Checkpointing, crash recovery |

### Exactly Once

```go
store := docket.NewMemoryStore()

g.Register(
	ExpensiveComputation,
	docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
)

// First call: executes
result1, _ := docket.Execute[*pb.Output](ctx, g, "exec-1", input)

// Second call, same input: cache hit (doesn't execute)
result2, _ := docket.Execute[*pb.Output](ctx, g, "exec-2", input)
```

### Crash Recovery

```go
store := docket.NewSQLStore(db, "checkpoints", docket.DialectPostgres)

g.Register(ExpensiveStep1, docket.WithPersistence(store, docket.ScopeWorkflow, 10*time.Minute))
g.Register(FlakyStep2, docket.WithPersistence(store, docket.ScopeWorkflow, 10*time.Minute))
g.Register(FinalStep3, docket.WithPersistence(store, docket.ScopeWorkflow, 10*time.Minute))

// First attempt: Step1 succeeds, Step2 fails
_, err := docket.Execute[*pb.Output](ctx, g, "exec-1", input)

// Retry same execution ID: Step1 restored from checkpoint, Step2 retries
result, _ := docket.Execute[*pb.Output](ctx, g, "exec-1", input)
```

## Persistence Backends

Store cached results in your existing infrastructure:

```go
// PostgreSQL
store := docket.NewSQLStore(db, "step_cache", docket.DialectPostgres)

// MySQL
store := docket.NewSQLStore(db, "step_cache", docket.DialectMySQL)

// SQLite
store := docket.NewSQLStore(db, "step_cache", docket.DialectSQLite)

// Redis
store := docket.NewRedisStore(redisClient)

// In-Memory (testing/development)
store := docket.NewMemoryStore()
```

Your cache is queryable:

```sql
-- Find stuck executions
SELECT * FROM step_cache
WHERE step_name = 'ProcessPayment'
AND created_at < NOW() - INTERVAL '1 hour';
```

## Observability

Hook into your existing monitoring:

```go
g := docket.NewGraph(
	docket.WithObserver(myObserver),
)

// Observer receives events for:
// - Graph start/end
// - Step start/end (with duration, errors)
// - Cache hits/misses
// - Retries
```

## Examples

Working examples in the `examples/` directory:

| Example | Description |
|---------|-------------|
| [hello_world](examples/hello_world) | Minimal getting started |
| [parallel](examples/parallel) | Automatic parallel execution |
| [batch](examples/batch) | Fan-out/fan-in collection processing |
| [struct_step](examples/struct_step) | Steps as struct methods |
| [error_handling](examples/error_handling) | Error classification and abort |
| [retry](examples/retry) | Retry policies and backoff |
| [exactly_once](examples/exactly_once) | Global persistence for idempotency |
| [crash_recovery](examples/crash_recovery) | Resume from checkpoints |
| [scope_comparison](examples/scope_comparison) | Workflow vs. global scope |
| [observability](examples/observability) | Prometheus metrics, structured logging |
| [river](examples/river) | Integration with River job queue |
| [persistence_postgres](examples/persistence_postgres) | PostgreSQL backend |
| [persistence_mysql](examples/persistence_mysql) | MySQL backend |
| [persistence_redis](examples/persistence_redis) | Redis backend |

## When to Use Docket

**Good fits:**
- Microservice aggregation (fan-out to multiple services, combine results)
- Feature computation (ML feature graphs at request latency)
- Multi-stage business logic (orders, applications, workflows)
- Any request-response flow with data dependencies

**Less good fits:**
- Long-running human-in-the-loop workflows (use Temporal)
- Batch ETL scheduled externally (use Dagster/Airflow)
- Stream processing (use Flink/Kafka Streams)
- Simple linear pipelines (just write sequential code)

## Contributing

Contributions welcome. Please include tests.

```bash
# Setup
make proto

# Test
make test
```

## License

MIT
