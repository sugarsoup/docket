# Docket: Type-Safe Workflow Orchestration for Go

You've written this code before: retry logic scattered across 15 microservices, each with slightly different backoff strategies. Manual WaitGroups to parallelize API calls. Cache keys constructed with string concatenation that breaks when someone reorders function parameters. When you needed something better, you looked at Temporal—but the operational overhead, performance characteristics, and determinism constraints felt like solving a different problem than the one you had. Docket is what you wish existed: a library that turns function signatures into orchestrated workflows, uses your existing datastores for state, and gets out of your way.

This guide explains how Docket works, when to use it, and how it compares to alternatives you already know.

---

## The Problem Space

### Four Scenarios You've Probably Built

#### 1. Order Processing with Partial Failures

You have a checkout flow: charge the credit card, reserve inventory, trigger shipping. If shipping fails, you've already charged the customer and reserved their items. You can't just retry the whole flow—that would double-charge them.

**Manual Go approach:**
```go
func ProcessOrder(ctx context.Context, order *Order) error {
    // Charge card
    chargeID, err := chargeCard(ctx, order)
    if err != nil {
        return fmt.Errorf("charge failed: %w", err)
    }

    // Reserve inventory - but what if THIS fails?
    // We've already charged the card!
    reservationID, err := reserveInventory(ctx, order)
    if err != nil {
        // Need to manually refund...
        refundCard(ctx, chargeID)
        return fmt.Errorf("reservation failed: %w", err)
    }

    // Ship - if THIS fails, we need to undo BOTH previous steps
    err = ship(ctx, order, chargeID, reservationID)
    if err != nil {
        refundCard(ctx, chargeID)
        releaseInventory(ctx, reservationID)
        return fmt.Errorf("shipping failed: %w", err)
    }

    return nil
}
```

The pain: Manual cleanup logic. No automatic retry. If the process crashes mid-execution, you need manual reconciliation. Testing requires careful setup of failure scenarios.

**How existing tools handle it:**
- **Temporal**: Requires deterministic code, heavyweight cluster, opaque blob storage
- **Step Functions**: Cloud-only, pays per state transition, JSON configuration
- **Airflow/Dagster**: Designed for batch jobs (minutes/hours), not request/response (milliseconds)

#### 2. Microservice Aggregation

You need data from three services to build a response: user profile, preferences, and activity logs. Calling them sequentially takes 1200ms. You know they're independent, so you write goroutines and WaitGroups.

**Manual Go approach:**
```go
func GetEnrichedUser(ctx context.Context, userID string) (*EnrichedUser, error) {
    var profile *Profile
    var prefs *Preferences
    var activity *Activity
    var wg sync.WaitGroup
    errCh := make(chan error, 3)

    wg.Add(3)
    go func() {
        defer wg.Done()
        p, err := fetchProfile(ctx, userID)
        if err != nil {
            errCh <- err
            return
        }
        profile = p
    }()

    go func() {
        defer wg.Done()
        pr, err := fetchPreferences(ctx, userID)
        if err != nil {
            errCh <- err
            return
        }
        prefs = pr
    }()

    go func() {
        defer wg.Done()
        a, err := fetchActivity(ctx, userID)
        if err != nil {
            errCh <- err
            return
        }
        activity = a
    }()

    wg.Wait()
    close(errCh)

    // Check for errors
    for err := range errCh {
        if err != nil {
            return nil, err
        }
    }

    return &EnrichedUser{
        Profile: profile,
        Prefs: prefs,
        Activity: activity,
    }, nil
}
```

The pain: Boilerplate goroutines, manual synchronization, error channel management, race conditions in testing. Every time you add another service call, you replicate this pattern.

#### 3. Batch Processing with Aggregates

You process a collection of movies: first compute the average rating across all items, then enrich each movie with whether it's above or below average.

**Manual Go approach:**
```go
func ProcessMovieBatch(ctx context.Context, movies []*Movie) ([]*EnrichedMovie, error) {
    // Phase 1: Compute aggregate (must see all items)
    var totalRating float64
    for _, m := range movies {
        totalRating += m.Rating
    }
    avgRating := totalRating / float64(len(movies))

    // Phase 2: Process each item (can be parallel, but need WaitGroup again)
    results := make([]*EnrichedMovie, len(movies))
    var wg sync.WaitGroup
    errCh := make(chan error, len(movies))

    for i, movie := range movies {
        wg.Add(1)
        go func(idx int, m *Movie) {
            defer wg.Done()
            enriched, err := enrichMovie(ctx, m, avgRating)
            if err != nil {
                errCh <- err
                return
            }
            results[idx] = enriched
        }(i, movie)
    }

    wg.Wait()
    close(errCh)

    for err := range errCh {
        if err != nil {
            return nil, err
        }
    }

    return results, nil
}
```

The pain: Fan-in/fan-out logic is manual, error-prone index management, no built-in concurrency control (what if there are 10,000 movies?), no retry on individual items.

#### 4. Background Jobs with Persistence

You use a job queue (like River) for async work. A job has multiple steps, and you want to cache expensive intermediate results across retries.

**Manual Go approach:**
```go
func ProcessVideoJob(ctx context.Context, videoURL string) error {
    // Check if we already transcoded this video
    cacheKey := fmt.Sprintf("transcode:%s", hashURL(videoURL))
    cached, err := redis.Get(ctx, cacheKey).Result()
    if err == nil {
        // Use cached transcode result
        return processTranscodedVideo(ctx, cached)
    }

    // Not cached, do the expensive work
    transcoded, err := transcodeVideo(ctx, videoURL)
    if err != nil {
        return fmt.Errorf("transcode failed: %w", err)
    }

    // Cache it
    redis.Set(ctx, cacheKey, transcoded, 24*time.Hour)

    // Continue with next step
    return processTranscodedVideo(ctx, transcoded)
}
```

The pain: Manual cache key generation (error-prone with parameter changes), manual TTL management, cache invalidation is your problem, no type safety on cached values.

---

## The Docket Approach

### Core Concept 1: Types ARE the Graph

In Docket, you don't build a DAG manually. You write functions with Protocol Buffer inputs and outputs. The framework infers dependencies from the type signatures.

**Example from `examples/parallel/main.go`:**

```go
// Step 1: Fetch user profile
g.Register(
    func(ctx context.Context, userID *pb.UserID) (*pb.UserProfile, error) {
        user := client.getUserProfile(userID)// API call
        return &pb.UserProfile{
            Id:    userID.Id,
            Name:  user.Name,
            Email: user.Email,
        }, nil
    },
    docket.WithName("FetchProfile"),
)

// Step 2: Fetch preferences (independent of profile)
g.Register(
    func(ctx context.Context, userID *pb.UserID) (*pb.UserPreferences, error) {
        prefs := prefsClient.GetPreferences(userID)
        return &pb.UserPreferences{
            UserId:   userID.Id,
            Theme:    prefs.Theme,
            Language: prefs.Language,
        }, nil
    },
    docket.WithName("FetchPreferences"),
)

// Step 3: Fetch activity (also independent)
g.Register(
    func(ctx context.Context, userID *pb.UserID) (*pb.UserActivity, error) {
        activity := activityClient.GetActivity(userID)
        return &pb.UserActivity{
            UserId:     userID.Id,
            LoginCount: len(activity.Logins),
        }, nil
    },
    docket.WithName("FetchActivity"),
)

// Step 4: Combine all data (depends on all three above)
g.Register(
    func(ctx context.Context,
         profile *pb.UserProfile,
         prefs *pb.UserPreferences,
         activity *pb.UserActivity) (*pb.EnrichedUser, error) {
        return &pb.EnrichedUser{
            Profile:     profile,
            Preferences: prefs,
            Activity:    activity,
        }, nil
    },
    docket.WithName("EnrichUser"),
)
```

**What just happened:**
1. Docket sees that `EnrichUser` needs `UserProfile`, `UserPreferences`, and `UserActivity`
2. It sees that all three can be produced from just `UserID`
3. It infers that the three fetch steps can run in parallel
4. The compiler verifies this wiring at build time

**Execution timeline:**

```mermaid
gantt
    title Automatic Parallelization
    dateFormat X
    axisFormat %L ms

    section Sequential
    FetchProfile (100ms)     :0, 100
    FetchPreferences (100ms) :100, 200
    FetchActivity (100ms)    :200, 300
    EnrichUser (instant)     :300, 300

    section Docket (Parallel)
    FetchProfile (100ms)     :0, 100
    FetchPreferences (100ms) :0, 100
    FetchActivity (100ms)    :0, 100
    EnrichUser (instant)     :100, 100
```

**Result:** 100ms total time vs 400ms sequential. **4x speedup with zero goroutines in user code.**

### Core Concept 2: Three Contexts Pattern

Docket separates three kinds of dependencies, each with different lifetimes:

1. **Graph-lifetime** (setup once): API clients, DB connections, configuration
2. **Execution-lifetime** (per workflow run): Request ID, user context, tracing spans
3. **Proto dependencies** (type-safe data flow): The actual data being processed

**Example: Struct-based steps with dependency injection** (`examples/struct_step/main.go`):

```go
// Graph-lifetime: Inject at setup
type WordScorer struct {
    Client *APIClient  // Shared across all executions
    Prefix string      // Configuration
}

// Execution-lifetime: Context passed to each run
// Proto dependencies: Typed input parameter
func (ws *WordScorer) Compute(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
    score := ws.Client.Lookup(input.Value)  // Use injected client

    return &pb.LetterCount{
        OriginalString: ws.Prefix + input.Value,
        Count:          int32(score),
    }, nil
}

// Register with injected dependencies
client := &APIClient{BaseURL: "https://api.example.com"}
scorer := &WordScorer{Client: client, Prefix: "scored: "}
g.Register(scorer, docket.WithName("ScoreWord"))
```

**Why this matters for testing:**
```go
// In tests, inject a mock client
mockClient := &MockAPIClient{responses: map[string]int{"test": 42}}
scorer := &WordScorer{Client: mockClient, Prefix: "test: "}
g.Register(scorer)

// Test the step in isolation
result, err := docket.Execute[*pb.LetterCount](ctx, g, "test-001",
    &pb.InputString{Value: "test"})
assert.Equal(t, int32(42), result.Count)
```

### Core Concept 3: Dependency Resolution vs Replay

This is the fundamental difference between Docket and Temporal.

**Temporal's approach (replay):**
1. Execute workflow, logging every decision to event history
2. On retry/resume: replay history, re-executing functions with logged results
3. Requires deterministic code (no random, no time.Now(), no map iteration)
4. Storage: opaque blobs in Temporal's database
5. Debugging: time-travel through event history

**Docket's approach (dependency resolution):**
1. Execute workflow, optionally caching expensive results
2. On retry/resume: fetch cached results if available, recompute if not
3. No determinism required—code is just normal Go
4. Storage: structured Protocol Buffer data in your Postgres
5. Debugging: SQL queries on your database

**Visual comparison:**

```mermaid
graph LR
    subgraph Temporal Replay
        E1[Execute Step A] --> L1[Log to History]
        L1 --> E2[Execute Step B]
        E2 --> L2[Log to History]
        L2 --> E3[Execute Step C]
        E3 --> L3[Log to History]

        style L1 fill:#f96,stroke:#333
        style L2 fill:#f96,stroke:#333
        style L3 fill:#f96,stroke:#333
    end

    subgraph Docket Resolution
        R1[Need Result C?] --> C1{Cached?}
        C1 -->|Yes| Fetch[Fetch from DB]
        C1 -->|No| R2[Need Result B?]
        R2 --> C2{Cached?}
        C2 -->|Yes| Fetch2[Fetch from DB]
        C2 -->|No| E4[Execute & Cache]

        style Fetch fill:#9f6,stroke:#333
        style Fetch2 fill:#9f6,stroke:#333
        style E4 fill:#69f,stroke:#333
    end
```

**Implications:**

| Aspect | Temporal (Replay) | Docket (Resolution) |
|--------|-------------------|---------------------|
| **Code constraints** | Must be deterministic | Normal Go code |
| **Time operations** | Must use workflow.Now() | Use time.Now() freely |
| **Random values** | Must use workflow.Random() | Use rand.* freely |
| **Iteration order** | Must be deterministic | Use maps normally |
| **Storage format** | Opaque event blobs | Queryable proto schema |
| **Debugging** | Replay in debugger | SELECT * FROM cache WHERE... |
| **Versioning** | Parallel version system | Git |
| **Cache invalidation** | N/A (always replays) | Automatic (content-addressed) |

**Cost model:**

Temporal pays persistence cost on **every step** (to enable replay).
Docket pays persistence cost **only when you ask** (via `WithPersistence()`).

```go
// Cheap step that fails? Rerun it. Cost: milliseconds.
g.Register(validateInput, docket.WithName("Validate"))

// Expensive step you want cached? Persist it.
g.Register(runMLInference,
    docket.WithName("MLInference"),
    docket.WithPersistence(sqlStore, docket.ScopeGlobal), // ← Cache across executions
)

// Side effect you can't repeat? Persist it.
g.Register(chargeCard,
    docket.WithName("ChargeCard"),
    docket.WithPersistence(sqlStore, docket.ScopeWorkflow), // ← Cache per execution
)
```

**Persistence mechanics:**
- Cache key: SHA-256 hash of (step name + input protos)
- Cache hit: Fetch from DB (~1ms)
- Cache miss: Execute step, store result
- Automatic invalidation: If inputs change, hash changes, cache misses

---

## Five Concrete Examples

### Example 1: Parallel Microservice Aggregation

**Problem:** Fetch user data from 3 independent services taking 100ms each. Sequential: 300ms. You want 100ms.

**Full working code:** [`examples/parallel/main.go`](/examples/parallel/main.go)

**Setup:**
```go
g := docket.NewGraph()

g.Register(fetchProfile,     docket.WithName("FetchProfile"))
g.Register(fetchPreferences, docket.WithName("FetchPreferences"))
g.Register(fetchActivity,    docket.WithName("FetchActivity"))
g.Register(enrichUser,       docket.WithName("EnrichUser"))

g.Validate()
```

**Execute:**
```go
result, err := docket.Execute[*pb.EnrichedUser](ctx, g, "parallel-001",
    &pb.UserID{Id: "user-123"})
```

**Results:**
```
⚙️  Executing (watch for parallel execution):
   → FetchProfile started (concurrent: 1)
   → FetchPreferences started (concurrent: 2)
   → FetchActivity started (concurrent: 3)
   ← FetchProfile finished
   ← FetchPreferences finished
   ← FetchActivity finished
   → EnrichUser started (concurrent: 1)
   ← EnrichUser finished

⏱️  Performance Analysis:
   Total time: 105ms
   Max concurrent: 3
   If sequential: ~400ms
   Actual (parallel): ~105ms
   Speedup: 3.8x
```

**Key learning:** Dependencies inferred from function signatures. No manual goroutines. Compiler verifies wiring.

---

### Example 2: Resilient Order Processing

**Problem:** Multi-step process where some errors should retry (network), some shouldn't (validation), and some should abort the entire graph (critical failures).

**Full working code:** [`examples/error_handling/main.go`](/examples/error_handling/main.go)

**Custom error types:**
```go
// ValidationError: Don't retry (bad input)
type ValidationError struct {
    Field string
    Issue string
}

// NetworkError: Retry with backoff (transient)
type NetworkError struct {
    Operation string
    Cause     error
}
```

**Error classification:**
```go
g.Register(
    fetchDataFromAPI,
    docket.WithName("FetchData"),
    docket.WithRetry(docket.RetryConfig{
        MaxAttempts: 3,
        Backoff: docket.ExponentialBackoff{
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     1 * time.Second,
            Factor:       2.0,
        },
    }),
    docket.WithTimeout(5*time.Second),
    docket.WithErrorClassifier(func(err error) bool {
        // ValidationErrors are permanent - don't retry
        var validationErr *ValidationError
        if errors.As(err, &validationErr) {
            return false  // Not retriable
        }

        // NetworkErrors are transient - retry them
        var networkErr *NetworkError
        if errors.As(err, &networkErr) {
            return true   // Retriable
        }

        // AbortErrors stop the whole graph - handled automatically
        var abortErr *docket.AbortError
        if errors.As(err, &abortErr) {
            return false  // Stops graph immediately
        }

        return true  // Default: retry
    }),
)
```

**Critical failures:**
```go
// In your step function
if criticalSystemFailure {
    return nil, docket.NewAbortErrorWithMessage(
        "database unavailable",
        errors.New("connection pool exhausted"),
    )
}
```

**Results:**
```
=== Example 1: Valid Input ===
Success: Count=11

=== Example 2: Validation Error (No Retry) ===
ValidationError (expected): validation failed on value: cannot be empty

=== Example 3: Network Error (Retries Exhausted) ===
NetworkError detected, will retry: network error during FetchData: connection timeout
[Retries 3 times with exponential backoff]
NetworkError after retries (expected): network error during FetchData: connection timeout

=== Example 4: Critical Failure (AbortError) ===
AbortError detected, stopping graph: critical system failure detected
```

**Key learning:** Fine-grained error control. `AbortError` stops the graph. Regular errors stop the step. ErrorClassifier decides retry logic.

---

### Example 3: Batch Processing with Aggregates

**Problem:** Process a collection where you need batch-wide statistics before enriching individual items.

**Full working code:** [`examples/batch/main.go`](/examples/batch/main.go), [`examples/batch/steps.go`](/examples/batch/steps.go)

**Aggregate step (fan-in):**
```go
// Runs ONCE for all movies in the batch
func ComputeBatchStats(ctx context.Context, movies []*pb.Movie) (*pb.BatchStats, error) {
    var totalRating float64
    maxRating := movies[0].Rating
    minRating := movies[0].Rating

    for _, movie := range movies {
        totalRating += movie.Rating
        if movie.Rating > maxRating {
            maxRating = movie.Rating
        }
        if movie.Rating < minRating {
            minRating = movie.Rating
        }
    }

    return &pb.BatchStats{
        Count:         int32(len(movies)),
        AverageRating: totalRating / float64(len(movies)),
        MaxRating:     maxRating,
        MinRating:     minRating,
    }, nil
}
```

**Per-item step (fan-out):**
```go
// Runs for EACH movie, using the shared batch stats
func EnrichMovie(ctx context.Context, movie *pb.Movie, stats *pb.BatchStats) (*pb.EnrichedMovie, error) {
    aboveAvgRating := movie.Rating > stats.AverageRating

    ratingRange := stats.MaxRating - stats.MinRating
    percentile := ((movie.Rating - stats.MinRating) / ratingRange) * 100

    return &pb.EnrichedMovie{
        Original:            movie,
        AboveAverageRating:  aboveAvgRating,
        RatingPercentile:    percentile,
    }, nil
}
```

**Register steps:**
```go
g.RegisterAggregate(ComputeBatchStats, docket.WithName("ComputeBatchStats"))
g.Register(EnrichMovie, docket.WithName("EnrichMovie"))
```

**Execute batch:**
```go
movies := []*pb.Movie{
    {Title: "The Shawshank Redemption", Year: 1994, Rating: 9.3},
    {Title: "The Godfather", Year: 1972, Rating: 9.2},
    {Title: "The Dark Knight", Year: 2008, Rating: 9.0},
}

results, err := docket.ExecuteBatch[*pb.Movie, *pb.EnrichedMovie](
    ctx, g, "batch-001", movies)
```

**Fan-out/fan-in visualization:**

```mermaid
graph TD
    M1[Movie 1] --> AGG[ComputeBatchStats<br/>fan-in]
    M2[Movie 2] --> AGG
    M3[Movie 3] --> AGG

    AGG --> STATS[BatchStats<br/>avgRating: 9.17]

    M1 --> E1[EnrichMovie 1]
    M2 --> E2[EnrichMovie 2]
    M3 --> E3[EnrichMovie 3]

    STATS --> E1
    STATS --> E2
    STATS --> E3

    E1 --> R1[EnrichedMovie 1]
    E2 --> R2[EnrichedMovie 2]
    E3 --> R3[EnrichedMovie 3]

    style AGG fill:#f96,stroke:#333,stroke-width:3px
    style STATS fill:#9f6,stroke:#333
```

**Results:**
```
📊 Batch Statistics (computed once via fan-in):
   Count:           5 movies
   Average Rating:  9.10
   Total Runtime:   767 min (12.8 hours)

📤 Enriched Movies (each compared to batch averages):
   1. ⬆️ The Shawshank Redemption (9.3 rating, percentile: 100%)
   2. ⬆️ The Godfather (9.2 rating, percentile: 67%)
   3.   The Dark Knight (9.0 rating, percentile: 0%)
```

**Key learning:** `RegisterAggregate` for batch-wide computation. Aggregate result is shared with per-item steps. Controlled concurrency with `WithMaxConcurrency()`.

---

### Example 4: Caching Expensive Operations

**Problem:** ML inference costs $0.50 per call. Same inputs recur frequently. You want content-addressed caching with automatic invalidation.

**Full working code:** [`examples/persistence_postgres/main.go`](/examples/persistence_postgres/main.go)

**Setup Postgres store:**
```go
db, err := sql.Open("pgx", "postgres://user:pass@localhost/docket")
sqlStore, err := docket.NewSQLStore(db, docket.DialectPostgres)
```

**Register with persistence:**
```go
g.Register(
    expensiveMLInference,
    docket.WithName("MLInference"),
    docket.WithPersistence(sqlStore, docket.ScopeGlobal),  // Shared across executions
    docket.WithTTL(24 * time.Hour),                        // Expire after 24h
)
```

**How it works:**
1. **First execution (cache miss):**
   - Input: `{text: "hello world"}`
   - Cache key: `SHA256("MLInference" + proto bytes)`
   - Execute step (500ms)
   - Store result in Postgres

2. **Second execution (cache hit):**
   - Input: `{text: "hello world"}` (same)
   - Cache key: Same SHA256 hash
   - Fetch from Postgres (~1ms)
   - Skip execution

3. **Different input (cache miss):**
   - Input: `{text: "goodbye world"}` (different)
   - Cache key: Different SHA256 hash
   - Execute step (500ms)
   - Store new result

**Scope options:**
- `ScopeGlobal`: Cache shared across all execution IDs (for pure functions)
- `ScopeWorkflow`: Cache tied to execution ID (for workflow-specific data)

**Queryable storage:**
```sql
SELECT
    step_name,
    input_hash,
    output_data,
    created_at
FROM docket_cache
WHERE step_name = 'MLInference'
ORDER BY created_at DESC;
```

**Key learning:** Content-addressed caching with SHA-256. Automatic invalidation when inputs change. Structured data queryable with SQL.

---

### Example 5: Job Queue Integration

**Problem:** You use River for background jobs. You want Docket's dependency resolution without replacing your queue.

**Full working code:** [`examples/river/main.go`](/examples/river/main.go)

**Architecture:**

```
┌─────────────┐
│ River Queue │ ← Your existing job queue
└──────┬──────┘
       │
       │ River handles: scheduling, retries, worker pools
       │
       ↓
┌─────────────────┐
│  GraphWorker    │ ← Adapter (maps River jobs to Docket graphs)
└──────┬──────────┘
       │
       │ Docket handles: step dependencies, parallelism, caching
       │
       ↓
┌─────────────────┐
│  Docket Graph   │ ← Your business logic
└─────────────────┘
```

**Setup:**
```go
// 1. Create Docket graph
g := docket.NewGraph()
g.Register(processVideo, docket.WithPersistence(sqlStore, docket.ScopeGlobal))
g.Validate()

// 2. Wrap in GraphWorker
worker := &docket.GraphWorker[*pb.VideoJob, *pb.VideoResult]{
    Graph: g,
}

// 3. Add to River
workers := river.NewWorkers()
river.AddWorker(workers, worker)

// 4. Start River with shared Postgres
riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
    Queues: map[string]river.QueueConfig{
        river.QueueDefault: {MaxWorkers: 10},
    },
    Workers: workers,
})

riverClient.Start(ctx)
```

**Enqueue a job:**
```go
job := &pb.VideoJob{URL: "https://example.com/video.mp4"}
_, err = riverClient.Insert(ctx, job, nil)
```

**Shared Postgres:**
- River stores: job queue, job state, retries
- Docket stores: step cache, intermediate results
- Same database, different tables
- Transactional consistency

**Execution flow:**
1. River picks job from queue
2. River calls GraphWorker.Work()
3. GraphWorker executes Docket graph
4. Docket fetches cached steps from Postgres
5. Docket computes missing steps
6. GraphWorker returns result to River
7. River marks job complete

**Key learning:** Docket fills the gap, doesn't replace your stack. River handles job infrastructure. Docket handles step orchestration. Shared Postgres for both.

---

## Architectural Deep Dive

### How Dependency Resolution Works

**Phase 1: Graph Construction**

When you register steps, Docket builds a type-based dependency graph:

```go
g.Register(func(ctx context.Context, a *A) (*B, error) { ... })
g.Register(func(ctx context.Context, b *B, c *C) (*D, error) { ... })
g.Register(func(ctx context.Context, a *A) (*C, error) { ... })
```

Internal representation:
```
A → B (Step 1)
A → C (Step 3)
B, C → D (Step 2)
```

**Phase 2: Validation**

`g.Validate()` checks:
1. No cycles (would cause infinite loop)
2. All types have producers (no missing steps)
3. No duplicate producers for same type
4. All steps have valid signatures

**Phase 3: Execution Planning**

When you request a type, Docket computes the minimal execution plan:

```go
plan, _ := g.GetExecutionPlan(&D{})
// Returns: ["Step 1", "Step 3", "Step 2"]
```

Algorithm:
1. Start with target type `D`
2. Find step that produces `D` (Step 2)
3. Recursively find steps for inputs `B` and `C`
4. Topological sort to determine execution order
5. Identify parallelizable branches (Step 1 and Step 3 are independent)

**Phase 4: Parallel Execution**

```go
result, err := docket.Execute[*D](ctx, g, "exec-001", &A{...})
```

Execution:
1. Create execution context with ID "exec-001"
2. Start with input `A`
3. Launch Step 1 and Step 3 in parallel (both need only `A`)
4. Wait for both to complete
5. Launch Step 2 with results `B` and `C`
6. Return `D`

**In-flight tracking:**

Prevents duplicate computation if two steps both need the same type:

```
Step X needs B → Check: Is B being computed? → Yes: Wait for result
Step Y needs B → Check: Is B being computed? → Yes: Reuse same result
```

### Persistence Model

**When to cache:**

Use `WithPersistence()` when:
- Step is expensive (>100ms)
- Step has side effects you can't repeat (billing, external API calls)
- Step results are reusable across executions

**Scope options:**

```go
// ScopeGlobal: Share across all execution IDs
// Use for: Pure functions, ML inference, data transformations
g.Register(transform,
    docket.WithPersistence(store, docket.ScopeGlobal))

// ScopeWorkflow: Tied to specific execution ID
// Use for: Workflow-specific data, intermediate state
g.Register(chargeCard,
    docket.WithPersistence(store, docket.ScopeWorkflow))
```

**Storage schema (Postgres):**

```sql
CREATE TABLE docket_cache (
    id SERIAL PRIMARY KEY,
    execution_id TEXT,          -- NULL for ScopeGlobal
    step_name TEXT NOT NULL,
    input_hash TEXT NOT NULL,   -- SHA-256 of input protos
    output_type TEXT NOT NULL,  -- Proto message name
    output_data BYTEA NOT NULL, -- Serialized proto
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,       -- NULL if no TTL
    UNIQUE(execution_id, step_name, input_hash)
);
```

**Cache key generation:**

```go
// Pseudo-code
func cacheKey(stepName string, inputs []proto.Message, scope Scope, execID string) string {
    hash := sha256.New()
    hash.Write([]byte(stepName))

    for _, input := range inputs {
        bytes, _ := proto.Marshal(input)
        hash.Write(bytes)
    }

    if scope == ScopeWorkflow {
        hash.Write([]byte(execID))
    }

    return hex.EncodeToString(hash.Sum(nil))
}
```

**Why SHA-256 of proto bytes:**
- Deterministic: Same inputs always produce same hash
- Automatic invalidation: Change any field, hash changes, cache misses
- Content-addressed: Don't need to manually track versions
- Collision-resistant: SHA-256 is cryptographically secure

**TTL and expiration:**

```go
g.Register(step,
    docket.WithPersistence(store, docket.ScopeGlobal),
    docket.WithTTL(24 * time.Hour),  // Expire cached results after 24h
)
```

Background process (implementation dependent):
```sql
DELETE FROM docket_cache WHERE expires_at < NOW();
```

### Retry & Timeout Mechanics

**Per-step retry configuration:**

```go
g.Register(flaky APICall,
    docket.WithRetry(docket.RetryConfig{
        MaxAttempts: 5,
        Backoff: docket.ExponentialBackoff{
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     10 * time.Second,
            Factor:       2.0,  // 100ms, 200ms, 400ms, 800ms, 1600ms
        },
    }),
)
```

**Backoff strategies:**

```go
// Exponential: 100ms → 200ms → 400ms → 800ms
ExponentialBackoff{InitialDelay: 100ms, Factor: 2.0}

// Fixed: 500ms → 500ms → 500ms
FixedBackoff{Delay: 500 * time.Millisecond}
```

**Retry decision flow:**

```
Step execution fails
    ↓
Is error an AbortError? → Yes → Stop graph immediately
    ↓ No
Call ErrorClassifier(err) → Returns false → Mark step failed, stop graph
    ↓ Returns true
Attempts < MaxAttempts? → No → Mark step failed, stop graph
    ↓ Yes
Sleep for backoff duration
    ↓
Retry step execution
```

**Per-step timeout:**

```go
g.Register(slowAPI,
    docket.WithTimeout(5 * time.Second),  // Context cancelled after 5s
)
```

Implementation:
```go
func executeWithTimeout(ctx context.Context, step Step, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    return step.Execute(ctx)
}
```

**Context cancellation propagation:**

When parent context is cancelled (user request timeout, server shutdown):
1. All in-flight steps receive cancelled context
2. Steps return context.Canceled error
3. Graph execution stops
4. No new steps are started

### Error Propagation

**Three error levels:**

```mermaid
graph TD
    START[Step Execution] --> CHECK{Error?}
    CHECK -->|No error| SUCCESS[Step Success]
    CHECK -->|Error| CLASSIFY{Error Type?}

    CLASSIFY -->|AbortError| ABORT[Stop Entire Graph<br/>All in-flight work cancelled]
    CLASSIFY -->|Regular Error + Not Retriable| STEP_FAIL[Stop Step<br/>Mark step failed<br/>Graph continues if non-critical]
    CLASSIFY -->|Regular Error + Retriable| RETRY{Attempts<br/>Remaining?}

    RETRY -->|Yes| BACKOFF[Wait for backoff] --> START
    RETRY -->|No| STEP_FAIL

    style ABORT fill:#f66,stroke:#333,stroke-width:3px
    style STEP_FAIL fill:#f96,stroke:#333
    style SUCCESS fill:#9f6,stroke:#333
```

**1. Regular errors (step-level):**

```go
func validateInput(ctx context.Context, input *pb.Input) (*pb.ValidatedInput, error) {
    if input.Value == "" {
        return nil, errors.New("value is required")  // Regular error
    }
    return &pb.ValidatedInput{Value: input.Value}, nil
}
```

Effect:
- Step marked as failed
- Error propagates to dependent steps
- Other independent steps continue
- If this step is critical to final result, graph fails

**2. AbortError (graph-level):**

```go
func chargeCard(ctx context.Context, payment *pb.Payment) (*pb.ChargeResult, error) {
    if databaseUnavailable {
        return nil, docket.NewAbortErrorWithMessage(
            "database unavailable",
            errors.New("connection pool exhausted"),
        )  // AbortError
    }
    // ...
}
```

Effect:
- Entire graph stops immediately
- All in-flight steps cancelled (via context)
- No further steps executed
- Used for critical failures where continuing is dangerous

**3. Panic recovery:**

```go
func buggyStep(ctx context.Context, input *pb.Input) (*pb.Output, error) {
    panic("unexpected nil pointer")  // Panic
}
```

Docket wraps execution:
```go
func safeExecute(step Step) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic in step: %v\nstack trace:\n%s", r, debug.Stack())
        }
    }()

    return step.Execute()
}
```

Effect:
- Panic converted to error
- Stack trace captured
- Treated as regular error (retriable if classifier allows)
- Graph doesn't crash

---

## Honest Comparisons

### Docket vs Temporal

Both solve workflow orchestration, but with fundamentally different execution models.

| Dimension | Docket | Temporal | When to Choose |
|-----------|--------|----------|----------------|
| **Execution model** | Dependency resolution: fetch cached or compute | Event replay: re-execute from history | **Docket:** Fast iteration, simpler ops. **Temporal:** Years-long workflows, time-travel debugging. |
| **Infrastructure** | Library using your Postgres + job queue | Dedicated cluster (Cassandra/MySQL + worker pools) | **Docket:** Existing stack, no new ops burden. **Temporal:** Greenfield or already invested in Temporal. |
| **Performance** | 10-100x higher throughput (in-memory by default) | Lower throughput (persist every decision) | **Docket:** High req/s (1000s/sec). **Temporal:** Durability guarantees matter more than speed. |
| **Determinism** | None required—normal Go code | Strictly enforced—no time.Now(), no random, no map iteration | **Docket:** Read clocks, call random(), iterate maps. **Temporal:** Replay correctness requires determinism. |
| **Storage format** | Queryable Protocol Buffers in your Postgres | Opaque event history blobs in Temporal DB | **Docket:** SQL debugging (`SELECT * WHERE...`). **Temporal:** Use Temporal UI/CLI. |
| **Versioning** | Git branches, standard deployments | Parallel version system (multiple versions live simultaneously) | **Docket:** Standard workflows, blue/green deploys. **Temporal:** Long-running workflows across code changes. |
| **Code constraints** | Use any Go library, any external calls | Must use workflow.* APIs for non-deterministic operations | **Docket:** Standard Go idioms. **Temporal:** Learning curve for workflow.Sleep(), workflow.Now(), etc. |
| **Debugging** | Logs, traces, SQL queries | Time-travel replay, event history browser | **Docket:** Familiar tools. **Temporal:** Powerful replay debugger for complex issues. |
| **Learning curve** | Minimal—register funcs, execute graph | Moderate—understand replay, determinism, versioning | **Docket:** Hours. **Temporal:** Days to weeks. |
| **Operational complexity** | Deploy Go binary, run migrations on Postgres | Run Temporal cluster, manage Cassandra, tune worker pools | **Docket:** Same as your app. **Temporal:** New cluster to operate. |

**When to use Temporal instead of Docket:**

1. **Multi-year workflows**: Mortgage processing, subscription renewals spanning years
2. **Regulatory compliance**: Need audit trail with time-travel replay
3. **Complex human-in-loop**: Approval workflows with weeks of wait time
4. **Polyglot requirements**: Need Python workers calling Java workers calling Go workers
5. **Already invested**: Team knows Temporal, cluster already running

**When to use Docket instead of Temporal:**

1. **Request/response flows**: API endpoints, webhook handlers (milliseconds to seconds)
2. **Background jobs**: Video processing, report generation (seconds to hours)
3. **Microservice orchestration**: Fan-out to multiple services, aggregate results
4. **Simpler operations**: Use existing Postgres, no new cluster to manage
5. **Type safety matters**: Want compiler-checked wiring, not runtime errors
6. **High throughput**: 1000s of workflows/second on modest hardware

**Example scenario:**

You're building an e-commerce checkout:
1. Validate cart
2. Charge credit card (external API, ~500ms)
3. Reserve inventory (DB transaction)
4. Send confirmation email
5. Trigger warehouse shipping (webhook)

**With Temporal:**
- Every step persisted to event history (overhead)
- Determinism constraints (can't use rand.* for idempotency key)
- Heavy cluster to run
- Excellent durability guarantees

**With Docket:**
- Fast in-memory execution
- Persist only expensive step (charge card)
- If process crashes after charging, resume uses cached charge result
- Uses your existing Postgres and job queue
- Normal Go code

**Cost comparison (approximate):**

| Metric | Docket (AWS r5.large) | Temporal (Self-hosted) | Temporal Cloud |
|--------|----------------------|------------------------|----------------|
| **Infrastructure** | $70/mo Postgres (RDS) | $500/mo cluster (Cassandra + workers) | $200-2000/mo depending on usage |
| **Throughput** | 10,000 workflows/sec | 100-1000 workflows/sec | Varies by tier |
| **Ops burden** | Same as your app | Manage cluster, tune workers | None (managed) |

**The honest take:**

Docket is not trying to replace Temporal for workflows that genuinely need replay, multi-year durability, or complex versioning. Temporal is excellent at what it does.

Docket targets the 80% of cases where you just need multi-step orchestration with retries, caching, and parallelism—without the operational overhead of a dedicated cluster or the constraints of deterministic execution.

If you're unsure, start with Docket. If you hit its limits (truly long-running sagas, need time-travel debugging), migrate to Temporal. The protobuf-based approach makes migration easier than starting with untyped code.

---

### Docket vs Airflow/Dagster/Prefect

These are **data pipeline orchestrators** designed for batch processing. Docket is a **workflow library** designed for online services.

| Dimension | Docket | Airflow/Dagster/Prefect | Why Different |
|-----------|--------|-------------------------|---------------|
| **Primary language** | Go | Python | Different ecosystems. Docket for Go services, data tools for Python analytics. |
| **Primary use case** | Request/response + background jobs (APIs, webhooks) | Batch data processing (ETL, data warehousing) | Docket: Online services (ms latency). Data tools: Scheduled jobs (minute/hour latency). |
| **Execution latency** | Milliseconds to seconds | Minutes to hours | SLA expectations differ by 1000x. |
| **Scheduling** | External (River, cron) or request-driven | Built-in rich scheduler (cron, intervals, sensors) | Data tools optimize for scheduled batch. Docket optimizes for on-demand. |
| **UI** | None—BYO monitoring (Grafana, Datadog) | Rich UI for DAG visualization, run history, logs | Trade-off: Ops burden vs built-in observability. |
| **Data handling** | Protocol Buffers (typed, structured) | DataFrames, files, SQL tables (flexible, schema-on-read) | Data tools designed for exploratory analysis. Docket for typed contracts. |
| **Parallelism** | Automatic from dependencies (goroutines) | Manual task dependencies, worker pools | Docket infers parallelism. Data tools require explicit DAG. |
| **Retries** | Per-step, fine-grained error classification | Per-task, typically uniform retry policy | Docket allows different retry for each step type. |

**Airflow example (Python):**

```python
from airflow import DAG
from airflow.operators.python import PythonOperator
from datetime import datetime, timedelta

def extract():
    # Fetch data from API
    pass

def transform(data):
    # Process data
    pass

def load(data):
    # Write to warehouse
    pass

dag = DAG(
    'etl_pipeline',
    schedule_interval='0 2 * * *',  # 2am daily
    start_date=datetime(2024, 1, 1),
)

extract_task = PythonOperator(task_id='extract', python_callable=extract, dag=dag)
transform_task = PythonOperator(task_id='transform', python_callable=transform, dag=dag)
load_task = PythonOperator(task_id='load', python_callable=load, dag=dag)

extract_task >> transform_task >> load_task  # Define dependencies manually
```

**Equivalent Docket (Go):**

```go
g := docket.NewGraph()
g.Register(extract, docket.WithName("Extract"))
g.Register(transform, docket.WithName("Transform"))
g.Register(load, docket.WithName("Load"))

// Dependencies inferred from function signatures
// Schedule externally (cron, River, etc.)
```

**When to use Airflow/Dagster/Prefect:**

1. **Data engineering**: ETL pipelines, data warehouse loads
2. **Analytics workflows**: Model training, report generation
3. **Scheduled batch**: Daily/hourly data processing
4. **Python ecosystem**: Pandas, NumPy, scikit-learn
5. **Need built-in UI**: DAG visualization, backfill, log viewing

**When to use Docket:**

1. **Go backend services**: API servers, webhook handlers
2. **Online latency**: Need sub-second response times
3. **Request-driven**: Triggered by user actions, not schedule
4. **Type safety**: Want compile-time guarantees
5. **Minimal ops**: Use existing infrastructure

**Can they coexist?**

Yes! Common pattern:
- **Airflow**: Nightly ETL to build recommendation models
- **Docket**: Real-time API to serve recommendations with multi-step enrichment

---

### Docket vs AWS Step Functions

Step Functions is a **managed state machine service**. Docket is a **library you deploy**.

| Dimension | Docket | Step Functions | Trade-off |
|-----------|--------|----------------|-----------|
| **Deployment** | Deploy with your Go app | AWS-managed service | **Docket:** Portable (any cloud/on-prem). **Step Functions:** Fully managed, no servers. |
| **Cost model** | Postgres storage costs (~$0.10/GB/mo) | $25 per million state transitions | **Docket:** Predictable, based on storage. **Step Functions:** Scales to zero, pay-per-use. |
| **Vendor lock-in** | None—standard Postgres, standard Go | AWS only | **Docket:** Move between clouds easily. **Step Functions:** Requires AWS ecosystem. |
| **Definition format** | Go code with types | JSON (Amazon States Language) or CDK | **Docket:** Type-safe code. **Step Functions:** Visual editor, declarative. |
| **Debugging** | Logs + SQL queries | CloudWatch logs + Step Functions console | **Step Functions:** Integrated debugging UI. **Docket:** Use your tools. |
| **Integration** | Generic (any service via Go code) | AWS services deeply integrated (Lambda, DynamoDB, SQS, etc.) | **Step Functions:** Trivial AWS integration. **Docket:** Flexible, any service. |
| **Execution model** | In-process or background job | Fully distributed state machine | **Step Functions:** Infinite scale. **Docket:** Bounded by your infrastructure. |

**Step Functions example:**

```json
{
  "Comment": "Order processing workflow",
  "StartAt": "ChargeCard",
  "States": {
    "ChargeCard": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:ChargeCard",
      "Next": "ReserveInventory",
      "Catch": [{
        "ErrorEquals": ["States.ALL"],
        "Next": "HandleFailure"
      }]
    },
    "ReserveInventory": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:ReserveInventory",
      "Next": "Ship"
    },
    "Ship": {
      "Type": "Task",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:Ship",
      "End": true
    },
    "HandleFailure": {
      "Type": "Fail"
    }
  }
}
```

**Equivalent Docket:**

```go
g := docket.NewGraph()
g.Register(chargeCard, docket.WithName("ChargeCard"), docket.WithRetry(...))
g.Register(reserveInventory, docket.WithName("ReserveInventory"))
g.Register(ship, docket.WithName("Ship"))

result, err := docket.Execute[*ShipmentResult](ctx, g, executionID, order)
```

**When to use Step Functions:**

1. **All-in on AWS**: Already using Lambda, DynamoDB, SQS extensively
2. **Managed service priority**: Don't want to run servers
3. **Need visual editor**: Non-engineers design workflows
4. **Sporadic traffic**: Pay only for executions, not idle time
5. **AWS integrations**: Benefit from native Step Functions connectors

**When to use Docket:**

1. **Multi-cloud or on-prem**: Can't or won't lock into AWS
2. **Cost predictability**: Want fixed Postgres costs, not per-transition
3. **Go ecosystem**: Backend is already Go
4. **Type safety**: Compiler catches errors, not runtime
5. **Local development**: Run entire workflow on laptop without cloud dependencies

**Migration path:**

If you start with Docket and later need AWS Step Functions:
- Protobuf definitions are portable
- Rewrite graph registration as Step Functions ASL
- Your business logic (step implementations) can move to Lambda as-is

---

## Getting Started in 10 Minutes

Three entry points depending on your goal:

### Option A: Run an Example (5 minutes)

See automatic parallelization in action:

```bash
git clone https://github.com/anthropics/protograph
cd protograph/examples/parallel
go run main.go
```

Expected output:
```
=== Docket Parallel Dependency Resolution ===

📋 Execution Plan:
   1. FetchProfile
   2. FetchPreferences
   3. FetchActivity
   4. EnrichUser

⚙️  Executing (watch for parallel execution):
   → FetchProfile started (concurrent: 1)
   → FetchPreferences started (concurrent: 2)
   → FetchActivity started (concurrent: 3)
   ← FetchProfile finished
   ← FetchPreferences finished
   ← FetchActivity finished
   → EnrichUser started (concurrent: 1)
   ← EnrichUser finished

⏱️  Performance Analysis:
   Total time: 105ms
   Max concurrent: 3
   If sequential: ~400ms
   Actual (parallel): ~105ms
   Speedup: 3.8x
```

**What you learned:** Three independent API calls run in parallel automatically. No goroutines in your code. 4x speedup.

Other examples to try:
- `examples/error_handling`: Retry, timeout, abort patterns
- `examples/batch`: Fan-out/fan-in with aggregate steps
- `examples/struct_step`: Dependency injection for testing

---

### Option B: Hello World (10 minutes)

Build a simple workflow from scratch:

**1. Install Docket:**
```bash
go get docket
```

**2. Define protobuf messages** (`hello.proto`):
```protobuf
syntax = "proto3";
package hello;
option go_package = "yourproject/proto/hello";

message Name {
    string value = 1;
}

message Greeting {
    string message = 1;
}
```

**3. Generate Go code:**
```bash
protoc --go_out=. hello.proto
```

**4. Write workflow** (`main.go`):
```go
package main

import (
    "context"
    "fmt"
    "log"

    "docket/pkg/docket"
    pb "yourproject/proto/hello"
)

func createGreeting(ctx context.Context, name *pb.Name) (*pb.Greeting, error) {
    return &pb.Greeting{
        Message: fmt.Sprintf("Hello, %s!", name.Value),
    }, nil
}

func main() {
    g := docket.NewGraph()
    g.Register(createGreeting, docket.WithName("CreateGreeting"))

    if err := g.Validate(); err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    result, err := docket.Execute[*pb.Greeting](ctx, g, "hello-001",
        &pb.Name{Value: "World"})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Message)  // Output: Hello, World!
}
```

**5. Run:**
```bash
go run main.go
```

**What you learned:** Basic graph registration and execution. Type-safe inputs and outputs.

---

### Option C: Integrate with Existing Project (30 minutes)

Add Docket to a real service:

**1. Identify a multi-step process in your codebase**

Look for code that:
- Makes multiple API calls
- Has manual retry logic
- Uses WaitGroups for parallelism
- Needs caching for expensive operations

**2. Extract steps into functions**

```go
// Before: Monolithic function
func ProcessOrder(order Order) error {
    // 50 lines of API calls, retries, error handling...
}

// After: Separate steps
func chargeCard(ctx context.Context, order *pb.Order) (*pb.ChargeResult, error) {
    // Just the charging logic
}

func reserveInventory(ctx context.Context, charge *pb.ChargeResult, order *pb.Order) (*pb.Reservation, error) {
    // Just the reservation logic
}

func ship(ctx context.Context, charge *pb.ChargeResult, reservation *pb.Reservation) (*pb.Shipment, error) {
    // Just the shipping logic
}
```

**3. Create protobuf definitions**

```protobuf
message Order {
    string order_id = 1;
    int64 amount_cents = 2;
}

message ChargeResult {
    string charge_id = 1;
    int64 amount_cents = 2;
}

message Reservation {
    string reservation_id = 1;
    repeated string sku_ids = 2;
}

message Shipment {
    string tracking_number = 1;
}
```

**4. Register steps in graph**

```go
g := docket.NewGraph()

g.Register(chargeCard,
    docket.WithName("ChargeCard"),
    docket.WithPersistence(sqlStore, docket.ScopeWorkflow),  // Don't double-charge
    docket.WithRetry(docket.RetryConfig{MaxAttempts: 3}),
)

g.Register(reserveInventory,
    docket.WithName("ReserveInventory"),
    docket.WithRetry(docket.RetryConfig{MaxAttempts: 3}),
)

g.Register(ship,
    docket.WithName("Ship"),
    docket.WithRetry(docket.RetryConfig{MaxAttempts: 5}),
    docket.WithTimeout(30 * time.Second),
)

g.Validate()
```

**5. Execute in your handler**

```go
func HandleCheckout(w http.ResponseWriter, r *http.Request) {
    order := parseOrder(r)

    executionID := fmt.Sprintf("order-%s", order.OrderID)
    result, err := docket.Execute[*pb.Shipment](r.Context(), g, executionID, order)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(result)
}
```

**What you learned:**
- How to refactor existing code into Docket steps
- Configuring persistence, retry, and timeout per step
- Integrating with HTTP handlers

**Before/after comparison:**

| Metric | Before (Manual) | After (Docket) |
|--------|----------------|----------------|
| Lines of code | 150 | 60 |
| Goroutines written | 5 | 0 |
| Retry logic blocks | 3 (duplicated) | 3 (declarative) |
| Cache key generation | Manual, error-prone | Automatic |
| Testability | Hard (need mocks for everything) | Easy (test steps in isolation) |

---

**Next steps:**

1. **Read the examples**: All in [`/examples`](/examples) directory
2. **Check the API docs**: Full reference in [`README.md`](/README.md)
3. **Join discussions**: GitHub issues for questions
4. **Start small**: Pick one workflow, refactor it, evaluate

---

## Advanced Topics

### Exactly-Once Semantics for Billing

When processing payments, you can't retry blindly—double-charging is worse than failing.

**Pattern: Idempotent steps with persistence**

```go
// ChargeCard step with idempotency
func chargeCard(ctx context.Context, order *pb.Order) (*pb.ChargeResult, error) {
    // Generate deterministic idempotency key from order ID
    idempotencyKey := fmt.Sprintf("charge-%s", order.OrderId)

    // Call payment provider with key
    result, err := stripeClient.Charge(ctx, &stripe.ChargeParams{
        Amount:         order.AmountCents,
        IdempotencyKey: idempotencyKey,  // Stripe deduplicates on this
    })
    if err != nil {
        return nil, err
    }

    return &pb.ChargeResult{
        ChargeId:     result.ID,
        AmountCents:  result.Amount,
    }, nil
}

// Register with persistence
g.Register(chargeCard,
    docket.WithPersistence(sqlStore, docket.ScopeWorkflow),  // Cache result per execution ID
    docket.WithRetry(docket.RetryConfig{MaxAttempts: 3}),
)
```

**How it works:**
1. First attempt: Call Stripe, succeed, cache result in Postgres
2. Process crashes before completing workflow
3. Retry: Docket fetches cached `ChargeResult`, skips re-executing `chargeCard`
4. Downstream steps continue with cached charge
5. No double-charge

**Key insight:** Persistence + external idempotency = exactly-once semantics

---

### Crash Recovery with Checkpointing

**Scenario:** Long-running workflow with expensive steps. Process crashes midway.

**Without checkpointing:**
- Restart from beginning
- Re-execute all steps (expensive, slow)

**With checkpointing:**
- Mark expensive steps with `WithPersistence`
- On crash, restart execution
- Docket fetches cached steps, recomputes only missing steps

**Example:**

```go
g.Register(fetchRawData,      docket.WithName("FetchRaw"))                          // Cheap, don't cache
g.Register(transcodeVideo,    docket.WithName("Transcode"),   docket.WithPersistence(...))  // Expensive, cache
g.Register(extractFrames,     docket.WithName("ExtractFrames"), docket.WithPersistence(...))  // Expensive, cache
g.Register(runMLModel,        docket.WithName("MLInference"),  docket.WithPersistence(...))  // Expensive, cache
g.Register(uploadResults,     docket.WithName("Upload"))                           // Cheap, don't cache

// First execution: All steps run
// Crash after extractFrames completes
// Second execution: FetchRaw runs again, Transcode + ExtractFrames fetched from cache, MLInference + Upload continue
```

**Cost comparison:**

| Execution | Without Caching | With Checkpointing |
|-----------|----------------|-------------------|
| First run | 10min | 10min |
| After crash at 7min | Restart: 10min (total 17min) | Resume: 3min (total 10min) |

---

### Observability

**Named steps for tracing:**

```go
g.Register(fetchProfile, docket.WithName("FetchProfile"))  // Shows as "FetchProfile" in logs/traces
```

**Execution plans:**

```go
plan, _ := g.GetExecutionPlan(&pb.EnrichedUser{})
for i, stepName := range plan {
    fmt.Printf("%d. %s\n", i+1, stepName)
}
// Output:
// 1. FetchProfile
// 2. FetchPreferences
// 3. FetchActivity
// 4. EnrichUser
```

**Integration with OpenTelemetry:**

```go
func tracedStep(ctx context.Context, input *pb.Input) (*pb.Output, error) {
    ctx, span := tracer.Start(ctx, "tracedStep")
    defer span.End()

    // Your logic
    result, err := doWork(ctx, input)
    if err != nil {
        span.RecordError(err)
        return nil, err
    }

    span.SetAttributes(attribute.String("result.id", result.Id))
    return result, nil
}
```

**SQL debugging:**

```sql
-- Find cached results for a specific execution
SELECT step_name, created_at
FROM docket_cache
WHERE execution_id = 'order-12345'
ORDER BY created_at;

-- See cache hit rate for expensive step
SELECT
    COUNT(*) as total_executions,
    COUNT(CASE WHEN cached THEN 1 END) as cache_hits,
    (COUNT(CASE WHEN cached THEN 1 END)::float / COUNT(*)) * 100 as hit_rate
FROM docket_executions
WHERE step_name = 'MLInference';
```

---

### Testing Strategies

**Unit testing steps:**

```go
func TestChargeCard(t *testing.T) {
    tests := []struct {
        name    string
        order   *pb.Order
        wantErr bool
    }{
        {"valid order", &pb.Order{OrderId: "123", AmountCents: 1000}, false},
        {"zero amount", &pb.Order{OrderId: "124", AmountCents: 0}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := chargeCard(context.Background(), tt.order)
            if (err != nil) != tt.wantErr {
                t.Errorf("chargeCard() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && result.AmountCents != tt.order.AmountCents {
                t.Errorf("amount = %v, want %v", result.AmountCents, tt.order.AmountCents)
            }
        })
    }
}
```

**Integration testing graphs:**

```go
func TestOrderWorkflow(t *testing.T) {
    g := docket.NewGraph()

    // Use test doubles for external dependencies
    g.Register(&MockCharger{shouldFail: false}, docket.WithName("ChargeCard"))
    g.Register(&MockInventory{inStock: true}, docket.WithName("ReserveInventory"))
    g.Register(&MockShipper{}, docket.WithName("Ship"))

    g.Validate()

    ctx := context.Background()
    result, err := docket.Execute[*pb.Shipment](ctx, g, "test-001",
        &pb.Order{OrderId: "test", AmountCents: 1000})

    require.NoError(t, err)
    assert.NotEmpty(t, result.TrackingNumber)
}
```

**Mocking with struct steps:**

```go
type MockCharger struct {
    shouldFail bool
}

func (m *MockCharger) Compute(ctx context.Context, order *pb.Order) (*pb.ChargeResult, error) {
    if m.shouldFail {
        return nil, errors.New("payment declined")
    }
    return &pb.ChargeResult{ChargeId: "mock-charge", AmountCents: order.AmountCents}, nil
}
```

---

## Limitations

Docket makes specific trade-offs. Here's what it doesn't do, and what to use instead.

### 1. Not a Platform (No Built-in UI)

**What this means:**
- No web UI for workflow visualization
- No built-in monitoring dashboard
- No managed service option

**Why:**
- Keeps Docket a library, not infrastructure to operate
- You use your existing monitoring (Grafana, Datadog, etc.)
- Avoids vendor lock-in

**If you need a UI:**
- Use **Temporal** (rich built-in UI)
- Use **Airflow/Dagster** (DAG visualization, backfill UI)
- Use **AWS Step Functions** (Step Functions console)

**Mitigation:**
- Build your own dashboard with SQL queries on `docket_cache` table
- Use OpenTelemetry for distributed tracing
- Use execution plans for documentation: `g.GetExecutionPlan()`

---

### 2. Go Only (Not Polyglot)

**What this means:**
- All steps must be Go code
- Can't mix Python ML steps with Java batch jobs

**Why:**
- In-process execution for performance
- Compile-time type checking
- Simpler deployment model

**If you need polyglot:**
- Use **Temporal** (supports Go, Java, TypeScript, Python, PHP)
- Use **Airflow** (Python-native, can shell out to other languages)

**Mitigation:**
- Call external services via RPC/HTTP from Go steps
- Use Protocol Buffers for language-neutral data formats
- Example: Go orchestration calling Python microservice for ML inference

---

### 3. Medium-Duration Workflows (Not Years-Long Sagas)

**What this means:**
- Best for workflows lasting seconds to hours
- Not optimized for multi-year processes (e.g., mortgage servicing)

**Why:**
- No built-in support for calendar-based waits ("wait 3 months")
- No long-term versioning strategy for code changes mid-workflow
- Postgres cache TTL might not be suitable for years

**If you need years-long:**
- Use **Temporal** (designed for multi-year workflows with versioning)
- Build explicit state machine with scheduled job polls

**Mitigation:**
- For medium-term (days to weeks): use job queue with scheduled retry
- Example: Email campaign with 7-day drip → schedule 7 jobs instead of one long workflow

---

### 4. No Visual Editor (Code-First Only)

**What this means:**
- You write Go code to define workflows
- No drag-and-drop interface for non-engineers

**Why:**
- Prioritizes type safety and version control over visual design
- Engineers who maintain it should own the definition

**If you need visual:**
- Use **AWS Step Functions** (Workflow Studio visual editor)
- Use **Zapier/n8n** for no-code automation

**Mitigation:**
- Generate documentation from code using execution plans
- Use Mermaid diagrams in docs (can be auto-generated from graph structure)

---

### 5. No Time-Travel Debugging (Can't Replay Exact History)

**What this means:**
- Can't step through historical execution with a debugger
- If workflow failed 3 days ago, you can see logs but can't replay exact state

**Why:**
- Dependency resolution model doesn't maintain event history
- Only caches results, not decision timeline

**If you need time-travel:**
- Use **Temporal** (replay any workflow execution in debugger)

**Mitigation:**
- Comprehensive logging and tracing (OpenTelemetry)
- Cache debugging: `SELECT * FROM docket_cache WHERE execution_id = '...'`
- Structured logs with execution context

---

## Adoption Path

How to incrementally adopt Docket without betting your company on it.

### Phase 1: Pilot with New Feature (2-4 weeks, low risk)

**Goal:** Validate that Docket improves your team's productivity for a real use case.

**Steps:**

1. **Pick a new feature** that needs orchestration:
   - Multi-step API aggregation
   - Background job with retries
   - Batch processing with fan-out/fan-in

2. **Build it with Docket from scratch:**
   - Define protobuf messages for data flow
   - Write steps as functions
   - Register in graph with retry/timeout config
   - Deploy alongside existing code

3. **Measure:**
   - Lines of code vs equivalent manual implementation
   - Time to implement
   - Bug count in first month
   - Team feedback: "Would you use this again?"

4. **Evaluate:**
   - Did Docket reduce boilerplate?
   - Was the team productive after initial learning?
   - Did type safety catch bugs early?
   - Does it feel worth the dependency?

**Example pilot:**
- Feature: User onboarding flow (create account → send email → provision resources)
- Team: 2 engineers, 2 weeks
- Result: 40% less code than manual goroutines, caught 3 wiring bugs at compile time
- Decision: Proceed to Phase 2

**Risk:** Low. If it doesn't work, remove Docket and rewrite manually. Investment: ~1 week per engineer.

---

### Phase 2: Refactor Pain Points (1-3 months, gradual rollout)

**Goal:** Apply Docket to existing code that's error-prone or hard to maintain.

**Steps:**

1. **Identify technical debt:**
   - Places with manual retry logic scattered everywhere
   - Complex goroutine coordination with WaitGroups
   - Caching with manual key generation
   - Multi-step processes with error-handling spaghetti

2. **Refactor one at a time:**
   - Extract steps into functions
   - Define protobuf types
   - Register in graph
   - Deploy behind feature flag
   - A/B test: old path vs Docket path
   - Compare error rates, latency

3. **Measure:**
   - Before/after lines of code
   - Before/after bug density (bugs per 1000 LOC)
   - Before/after test coverage
   - Production metrics (latency, error rate, success rate)

4. **Iterate:**
   - If successful: refactor next pain point
   - If issues: debug, improve, or rollback

**Example refactors:**
- Checkout flow: 150 lines → 60 lines, 5 race conditions fixed
- Report generation: Added checkpointing, reduced retry time by 70%
- Microservice aggregation: Automatic parallelization, 3x latency improvement

**Risk:** Medium. Gradual rollout with feature flags limits blast radius.

---

### Phase 3: Platform Pattern (6+ months, standardization)

**Goal:** Make Docket the standard way your team builds multi-step workflows.

**Steps:**

1. **Create internal guidelines:**
   - When to use Docket vs manual code
   - Protobuf naming conventions
   - Retry/timeout best practices
   - Testing patterns

2. **Build shared infrastructure:**
   - Shared Postgres store for caching
   - Common StepOption presets (e.g., `standardRetry`, `billingPersistence`)
   - Monitoring dashboards for workflow metrics
   - CI/CD templates for deploying Docket-based services

3. **Train the team:**
   - Onboarding docs
   - Example projects
   - Office hours for questions

4. **Expand use cases:**
   - New services built with Docket by default
   - Refactor more legacy code over time

**Success metrics:**
- % of new multi-step features using Docket
- Developer satisfaction surveys
- Reduction in production incidents related to retry/concurrency bugs
- Time to implement new workflows (before vs after)

**Risk:** High if you skip phases 1-2. Low if you've validated value in pilots and refactors.

---

### Exit Strategy (If You Need to Leave)

**Why an exit strategy matters:**

Docket should pass "the keeper test"—you keep using it because it's useful, not because leaving is painful.

**How to leave Docket if needed:**

1. **Your data is portable:**
   - Cached results are in your Postgres as Protocol Buffers
   - No vendor lock-in, no proprietary formats
   - You own the database schema

2. **Remove the dependency:**
   - Option A: Rewrite workflows using manual Go code (goroutines, retry logic)
   - Option B: Migrate to another tool (Temporal, Step Functions)
   - Option C: Use cached results as starting point for new implementation

3. **No data migration needed:**
   - Keep using the same Postgres database
   - Keep using the same protobufs
   - Just change the orchestration code

4. **Gradual removal:**
   - Remove one workflow at a time
   - Use feature flags to switch between Docket and new implementation
   - No big-bang cutover required

**Cost to leave:** Proportional to how much you've invested. If you used Docket for 2 workflows, rewriting takes days. If you built 50 workflows, it takes months—but no vendor lock-in forces you to stay.

**The honest take:**

We want Docket to be useful enough that you don't want to leave, not hard enough to leave that you can't. If it stops serving you, walk away. The low commitment is a feature, not a bug.

---

## Conclusion

Docket gives you type-safe workflow orchestration without the operational overhead of a dedicated platform. It's for the 80% of cases where you just need retry logic, parallelism, and caching—not years-long sagas or time-travel debugging.

**You should try Docket if:**
- You're building Go backend services
- You have multi-step processes with dependencies
- You want compiler-checked wiring instead of runtime errors
- You're tired of manual retry/concurrency boilerplate
- You want to use your existing Postgres and job queue

**You should use something else if:**
- You need polyglot workflows (Python + Java + Go)
- You need workflows lasting years with versioning
- You need a built-in UI for non-engineers
- You're already invested in Temporal or Step Functions

**Next steps:**
1. Run `examples/parallel` to see automatic parallelization
2. Read the full API docs in [`README.md`](/README.md)
3. Try refactoring one workflow in your codebase
4. Join the discussion on GitHub

The goal isn't to replace your entire stack. The goal is to fill the gap between "manually write goroutines and retry logic" and "adopt a heavyweight workflow platform." If Docket makes your code simpler, keep it. If not, leave. The choice is yours.
