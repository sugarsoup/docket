# Docket: The Data Types Are The Graph

## The Insight

Here's a function:

```go
func EnrichUser(ctx context.Context, profile *UserProfile, prefs *UserPreferences) (*EnrichedUser, error)
```

What does this tell you?

1. It needs a `UserProfile` and `UserPreferences`
2. It produces an `EnrichedUser`
3. It can't run until both inputs exist

That's a dependency graph. The type signature *is* the edge declaration. You didn't draw a DAG—you wrote a function, and the graph emerged from the types.

Now here's the question: what if your runtime understood this?

What if, when you ask for an `EnrichedUser`, the system automatically:
- Finds the step that produces `UserProfile`
- Finds the step that produces `UserPreferences`
- Notices they're independent and runs them in parallel
- Waits for both, then runs your enrichment step
- Caches the expensive ones, recomputes the cheap ones

No manual DAG construction. No explicit parallelism. No goroutines or WaitGroups. The types already encode the structure—the runtime just follows them.

That's Docket.

---

## Make for Data

Build systems solved this problem decades ago.

`make` has targets, dependencies, and rules. When you request a target, it walks the dependency graph backward, figures out what's stale, and rebuilds only what's necessary. The graph is implicit in the Makefile declarations.

Docket applies the same insight to data transformations:

| make | Docket |
|------|------------|
| Targets (output files) | Output types (protos) |
| Dependencies (input files) | Input types (function params) |
| Rules (shell commands) | Steps (typed functions) |
| File timestamps | Content hashes |
| Incremental rebuild | Selective caching |

The difference: `make` operates on files with explicit dependency declarations. Docket operates on typed data with dependencies inferred from function signatures.

```go
// This IS the dependency declaration
func FetchProfile(ctx context.Context, id *UserID) (*UserProfile, error)
func FetchPrefs(ctx context.Context, id *UserID) (*UserPreferences, error)
func Enrich(ctx context.Context, p *UserProfile, prefs *UserPreferences) (*EnrichedUser, error)
```

Register these three functions. Ask for `*EnrichedUser`. The graph resolves itself.

---

## Three Problems, One Pattern

### 1. Microservice Aggregation

Your API needs to combine data from five services. The naive approach:

```go
profile, err := profileService.Get(userID)      // 50ms
prefs, err := prefsService.Get(userID)          // 50ms
activity, err := activityService.Get(userID)    // 50ms
friends, err := friendsService.Get(userID)      // 50ms
recommendations, err := recsService.Get(userID) // 50ms
// Total: 250ms sequential
```

The manual parallel approach: goroutines, WaitGroups, error channels, coordination logic. It works, but it's boilerplate that obscures intent.

With Docket:

```go
g.Register(FetchProfile)      // *UserID → *UserProfile
g.Register(FetchPrefs)        // *UserID → *UserPreferences
g.Register(FetchActivity)     // *UserID → *UserActivity
g.Register(FetchFriends)      // *UserID → *FriendsList
g.Register(FetchRecs)         // *UserID → *Recommendations
g.Register(BuildResponse)     // all five → *APIResponse

result, _ := docket.Execute[*APIResponse](ctx, g, requestID, userID)
// Total: ~50ms (parallel) + combination step
```

The five fetches run in parallel automatically—the graph sees they share a common dependency (`UserID`) and have no dependencies on each other. You didn't write parallelism. You wrote functions with types.

### 2. Feature Computation

ML feature serving has the same structure. You have raw entities, derived features, and composed features:

```go
// Raw data
g.Register(FetchUser)           // *UserID → *User
g.Register(FetchTransactions)   // *UserID → *TransactionHistory

// Derived features (can run in parallel)
g.Register(ComputeSpendingPattern)   // *TransactionHistory → *SpendingPattern
g.Register(ComputeRiskSignals)       // *TransactionHistory → *RiskSignals
g.Register(ComputeUserSegment)       // *User → *UserSegment

// Composed features
g.Register(BuildFeatureVector)  // all derived → *FeatureVector
```

This is what Chronon, Feast, and Tecton do for feature definitions—but they're Python frameworks designed for batch pipelines. Docket gives you the same dependency semantics at request-response latency, embedded in your Go service.

Cache the expensive features (`ScopeGlobal`). Recompute the cheap derivations. The graph handles it.

### 3. Multi-Stage Business Logic

Order processing, loan applications, onboarding flows—any process with stages that depend on previous stages:

```go
g.Register(ValidateOrder)        // *OrderRequest → *ValidatedOrder
g.Register(CheckInventory)       // *ValidatedOrder → *InventoryCheck
g.Register(CalculatePricing)     // *ValidatedOrder → *PricingResult
g.Register(ApplyDiscounts)       // *PricingResult, *Customer → *FinalPrice
g.Register(ReserveInventory)     // *InventoryCheck, *FinalPrice → *Reservation
g.Register(ProcessPayment)       // *FinalPrice, *PaymentMethod → *PaymentResult
g.Register(FinalizeOrder)        // *Reservation, *PaymentResult → *CompletedOrder
```

The validation and inventory check can run in parallel. Pricing and discounts sequence naturally. The payment step waits for everything it needs. You didn't orchestrate this—the types did.

---

## Selective Durability

Not all computations are equal:

- **Charging a credit card** has a side effect you cannot repeat
- **Calling an expensive API** costs money per call
- **Validating input** takes 2ms and is trivially repeatable
- **Computing a hash** is pure and deterministic

Docket lets you choose:

```go
// Precious: cache globally, never repeat for same inputs
g.Register(ChargeCard,
    docket.WithPersistence(store, docket.ScopeGlobal, 24*time.Hour))

// Expensive: cache within this execution
g.Register(CallExpensiveAPI,
    docket.WithPersistence(store, docket.ScopeWorkflow, 1*time.Hour))

// Cheap: just recompute
g.Register(ValidateInput)  // no persistence
```

The cache key is a SHA-256 hash of the step name and serialized inputs. If the inputs change, the cache misses automatically. If the inputs are identical, you get exactly-once execution.

This is the key difference from replay-based systems. Temporal records *every* step and replays the full history. Docket caches *selectively* and recomputes the rest. Eight milliseconds of validation is cheaper than a database round trip.

---

## Crystalline Growth

Here's a property that emerges from "one producer per type":

**New code can only attach to existing types.**

When you add a feature, you either:
1. Consume existing types (depend on what's there)
2. Produce a new type (extend the graph)

You cannot create a shadow `UserProfile` that conflicts with the existing one. You cannot introduce hidden dependencies. The graph structure enforces coherence.

This changes how teams scale:

**Without Docket**: "I need to understand how user data flows through the system before I can add this feature. Let me trace through seven files to find all the places that touch UserProfile."

**With Docket**: "I need a `UserProfile`. There's a step that produces it. I declare my dependency and write my logic. The framework handles acquisition."

Engineers don't need perfect knowledge of the codebase. They need to know what types exist and what type they're producing. Code review shifts from "what hidden state might this affect?" to "does this step's contract make sense?"

The codebase crystallizes along well-defined edges instead of spaghettifying through implicit dependencies.

---

## What This Replaces

### vs. Workflow Engines (Temporal, Cadence, Inngest)

Workflow engines use **replay**: they record every step's result and reconstruct state by re-running your code against the recorded history.

This requires:
- Deterministic code (no random, no time, no iteration order variance)
- Opaque blob storage (the engine can't understand your types)
- Version patching (old history, new code)
- Dedicated infrastructure (Cassandra clusters, worker pools)

Docket doesn't replay. It **resolves and caches**. Your code runs normally. Precious results come from the cache. Cheap results recompute. There's no determinism requirement because there's no history to replay against.

**Temporal printing "Hello World" at 1,000 workflows/sec**: $100k/month in Cassandra + ElasticSearch + Kubernetes + operations team.

**Docket doing equivalent coordination**: Your existing Postgres, your existing workers. Maybe $1k/month. The 100x cost reduction comes from letting your database understand what it's storing.

### vs. DAG Frameworks (Dagster, Airflow, Prefect)

These are batch orchestrators. They're designed for:
- Minute-to-hour execution times
- Scheduler-triggered runs
- External coordination infrastructure
- Python ecosystems

Docket is a library. It's designed for:
- Millisecond execution times
- Request-triggered runs
- Embedded in your service
- Go ecosystem (though the pattern ports)

Dagster's "software-defined assets" are conceptually similar—typed outputs with declared dependencies. Docket is that model as an embeddable library at request-response latency.

### vs. Manual Parallelism

```go
// Manual approach
var wg sync.WaitGroup
var profile *UserProfile
var prefs *UserPreferences
var profileErr, prefsErr error

wg.Add(2)
go func() {
    defer wg.Done()
    profile, profileErr = fetchProfile(ctx, userID)
}()
go func() {
    defer wg.Done()
    prefs, prefsErr = fetchPrefs(ctx, userID)
}()
wg.Wait()

if profileErr != nil { return nil, profileErr }
if prefsErr != nil { return nil, prefsErr }

return enrich(profile, prefs)
```

This is fine for two dependencies. Now do it for seven. Now add caching. Now add retries. Now add timeouts. Now add observability.

Docket absorbs this complexity. The parallelism is structural, not manual.

---

## The API

```go
// Create graph
g := docket.NewGraph(
    docket.WithDefaultTimeout(30*time.Second),
    docket.WithObserver(prometheusObserver),
)

// Register steps (dependencies inferred from signatures)
g.Register(FetchProfile)
g.Register(FetchPrefs)
g.Register(EnrichUser)

// Configure resilience per-step
g.Register(FlakyExternalCall,
    docket.WithTimeout(5*time.Second),
    docket.WithRetry(docket.RetryConfig{
        MaxAttempts: 3,
        Backoff: docket.ExponentialBackoff{
            InitialDelay: 100*time.Millisecond,
            Factor: 2,
        },
    }),
    docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
)

// Validate graph structure (catches cycles, missing deps)
if err := g.Validate(); err != nil {
    log.Fatal(err)
}

// Execute: request output type, provide leaf inputs
result, err := docket.Execute[*EnrichedUser](ctx, g, "request-123", userID)
```

That's it. Register functions. Validate. Execute. The graph handles parallelism, caching, retries, timeouts, and observability.

---

## Persistence Backends

Docket stores cached results in your infrastructure:

- **PostgreSQL**: Production-ready, works with your existing pgx pool
- **MySQL**: Same pattern, different dialect
- **SQLite**: Single-machine deployments, local development
- **Redis**: Fast caching layer with native TTL
- **In-Memory**: Testing and development

The cache schema is simple: key (step + input hash), value (serialized proto), metadata (timestamps, TTL). Your database understands it. You can query it:

```sql
-- Show all orders stuck at inventory check
SELECT * FROM step_cache
WHERE step_name = 'CheckInventory'
AND created_at < NOW() - INTERVAL '1 hour';

-- Retry failed payments from yesterday
DELETE FROM step_cache
WHERE step_name = 'ProcessPayment'
AND metadata->>'status' = 'failed'
AND created_at > NOW() - INTERVAL '1 day';
```

This is impossible with opaque blob storage. When your workflow state is queryable, debugging becomes SQL instead of spelunking through proprietary UIs.

---

## Batch Processing

Some workflows process collections. Docket handles fan-out/fan-in natively:

```go
// Per-item step: runs once per item, in parallel
g.Register(func(ctx context.Context, movie *Movie) (*EnrichedMovie, error) {
    return enrichMovie(ctx, movie)
})

// Aggregate step: runs once for entire batch, produces shared result
g.RegisterAggregate(func(ctx context.Context, movies []*Movie) (*BatchStats, error) {
    return computeStats(movies), nil
})

// Combine: per-item step that uses aggregate result
g.Register(func(ctx context.Context, movie *EnrichedMovie, stats *BatchStats) (*RankedMovie, error) {
    return rankAgainstBatch(movie, stats), nil
})

// Execute batch
results, err := docket.ExecuteBatch[*RankedMovie](ctx, g, "batch-1", movies,
    docket.WithMaxConcurrency(50),  // Limit parallel items
)
```

The framework:
1. Computes `BatchStats` once (aggregate)
2. Enriches each movie in parallel (fan-out)
3. Ranks each movie against batch stats (fan-in of aggregate)

Concurrency limiting prevents overwhelming downstream services. The parallelism is automatic but bounded.

---

## When To Use This

**Good fits:**
- Request-response services that aggregate multiple data sources
- ML feature serving with complex feature dependencies
- Multi-stage business logic (orders, applications, onboarding)
- Any workflow where you want type-safe step composition

**Less good fits:**
- Long-running human-in-the-loop workflows (use Temporal)
- Batch ETL scheduled by external triggers (use Dagster/Airflow)
- Stream processing (use Flink/Kafka Streams)
- Simple linear pipelines with no branching (just write sequential code)

Docket is for **typed DAGs at request-response latency**. If your execution spans days with human approvals, you want a different tool. If your execution spans milliseconds with data dependencies, this is the tool.

---

## The Economics

Why does this matter financially?

**Replay-based systems** pay storage costs proportional to execution depth. Every step is recorded. The storage doesn't understand the data, so it can't compress, index, or optimize. You need specialized infrastructure because general-purpose databases aren't designed for high-throughput opaque blob insertion.

**Docket** pays storage costs proportional to what you choose to cache. Cheap steps don't touch storage. Expensive steps cache with full schema support. Your existing database handles it because it's just rows with typed columns.

The 100x cost reduction isn't about Docket being clever. It's about not paying for replay semantics when you only need selective caching.

---

## Summary

Docket is **typed dependency resolution with lazy evaluation and selective memoization**.

The types are the graph. You write functions with typed inputs and outputs. The framework infers dependencies, parallelizes automatically, and caches selectively.

It's `make` for data transformations. It's Dagster's asset graph as an embeddable library. It's dependency injection for data, not just objects.

For microservices: parallel aggregation without manual goroutines.
For ML: feature dependency graphs at serving latency.
For business logic: structured workflows without replay complexity.
For teams: crystalline code growth instead of spaghetti dependencies.

You probably already have queues, retries, scheduling, and persistence. You're missing one thing: something that understands step dependencies and knows which results to cache.

Add that. Keep everything else.
