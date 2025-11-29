# Protograph

**Protograph** is a type-safe, proto-native workflow engine for Go. It turns your Protocol Buffers into a self-assembling execution graph, handling dependency resolution, concurrency, and persistence automatically.

> **Philosophy**: The data types *are* the graph. You define functions that transform Protos, and Protograph figures out how to wire them together.

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
go get github.com/sugarsoup/protograph
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
Create a graph and register functions. Protograph inspects the arguments to determine dependencies.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sugarsoup/protograph/pkg/protograph"
	pb "myapp/proto" // Your generated protos
)

func main() {
	// Create graph with optional default timeout
	g := protograph.NewGraph(protograph.WithDefaultTimeout(30 * time.Second))

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

	result, err := protograph.Execute[*pb.EnrichedUser](ctx, g, "request-id-1", input)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Result: %+v\n", result)
}
```

## 🧠 Core Concepts

### Dependency Injection
Dependencies are resolved by type. If a function requires `*pb.UserProfile`, Protograph finds the registered step that returns `*pb.UserProfile` and executes it first.

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
    protograph.WithMaxConcurrency(50), // Process at most 50 items at once
)
```

See `examples/batch` for a complete movie processing pipeline.

### Persistence & Caching
Cache expensive steps automatically using SQLite, Postgres, or Memory.

```go
// Initialize Store
store := protograph.NewSQLStore(db, "step_cache", protograph.DialectPostgres)
store.InitSchema(ctx)

// Register with Persistence
g.Register(
	HeavyComputationStep,
	protograph.WithPersistence(store, protograph.ScopeGlobal, 1*time.Hour),
)
```
Keys are generated using **SHA-256 hashes of the inputs**, ensuring that if your data changes, the cache invalidates automatically. See `examples/persistence`.

### Retries & Timeouts
Configure robustness policies per-step:

```go
g.Register(
	FlakyNetworkCall,
	protograph.WithTimeout(5*time.Second),
	protograph.WithRetry(protograph.RetryConfig{
		MaxAttempts: 3,
		Backoff:     protograph.ExponentialBackoff{InitialDelay: 100*time.Millisecond, Factor: 2},
	}),
)
```

## 🤝 Contributing

Contributions are welcome! Please ensure new features are covered by tests.

1. Install Protoc
2. `make proto`
3. `make test`

## License

MIT
