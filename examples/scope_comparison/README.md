# Persistence Scope Comparison

This example provides a **side-by-side comparison** of `ScopeWorkflow` vs `ScopeGlobal` persistence scopes in Docket.

## What This Demonstrates

Both persistence scopes provide deduplication, but with different cache lifetimes:

- **ScopeWorkflow**: Cache is **execution-local** (cleared after each graph execution)
- **ScopeGlobal**: Cache is **persistent across executions** (shared globally)

## Running the Example

```bash
go run examples/scope_comparison/main.go
```

## Expected Output

### Part 1: Multiple Executions

```
--- ScopeWorkflow: Cache per execution ---

  Execution #1 (Workflow Scope)
    [WORKFLOW SCOPE] Executing for user Alice (execution #1)

  Execution #2 (Workflow Scope)
    [WORKFLOW SCOPE] Executing for user Alice (execution #2)

  Execution #3 (Workflow Scope)
    [WORKFLOW SCOPE] Executing for user Alice (execution #3)

Workflow Scope Executions: 3 (expected: 3)

--- ScopeGlobal: Cache across executions ---

  Execution #1 (Global Scope)
    [GLOBAL SCOPE] Executing for user Alice (execution #1)

  Execution #2 (Global Scope)
    [result from cache - no execution]

  Execution #3 (Global Scope)
    [result from cache - no execution]

Global Scope Executions: 1 (expected: 1)
```

### Part 2: Within-Execution Deduplication

Both scopes deduplicate within a single execution:

```
--- ScopeWorkflow: Within-execution deduplication ---
  Resolving ProcessedData 3 times in same execution:
    Resolve #1: Done
    Resolve #2: Done (cached)
    Resolve #3: Done (cached)

  Workflow executions: 1 (deduplicated within execution)

--- ScopeGlobal: Within-execution deduplication ---
  Resolving ProcessedData 3 times in same execution:
    Resolve #1: Done
    Resolve #2: Done (cached)
    Resolve #3: Done (cached)

  Global executions: 1 (deduplicated within execution)
```

## Comparison Table

```
┌─────────────────────┬──────────────────┬──────────────────┐
│ Scenario            │ ScopeWorkflow    │ ScopeGlobal      │
├─────────────────────┼──────────────────┼──────────────────┤
│ Within execution    │ ✓ Deduplicated   │ ✓ Deduplicated   │
│ Across executions   │ ✗ Re-executes    │ ✓ Cached         │
│ Cache key includes  │ execution_id     │ step_name only   │
│ Use case            │ Temp results     │ Reusable results │
└─────────────────────┴──────────────────┴──────────────────┘
```

## Key Differences

### ScopeWorkflow

**Cache Key**: `(execution_id, step_name, input_hash)`

**Lifetime**: Single execution only

**Behavior**:
- Results cached within single graph execution
- Cache cleared when execution completes
- New graph execution = fresh cache
- Same inputs in different executions = re-execution

**When to Use**:
- Results are execution-specific
- Temporary/transient data processing
- Each execution should be independent
- Don't want results persisting across runs

**Example Use Cases**:
```go
// User session processing (shouldn't carry over to next session)
WithPersistence(store, ScopeWorkflow, 1*time.Hour)

// One-off batch job (fresh start each run)
WithPersistence(store, ScopeWorkflow, 30*time.Minute)

// Development/testing (avoid stale cached data)
WithPersistence(store, ScopeWorkflow, 5*time.Minute)
```

### ScopeGlobal

**Cache Key**: `(step_name, input_hash)`

**Lifetime**: Until TTL expires or store cleared

**Behavior**:
- Results cached globally across all executions
- Same inputs = cache hit (no re-execution)
- Persists across graph instances
- Provides exactly-once guarantee per unique input

**When to Use**:
- Results are reusable across executions
- Expensive computations worth caching
- Idempotent operations
- Want to avoid duplicate work

**Example Use Cases**:
```go
// External API calls (cache responses)
WithPersistence(store, ScopeGlobal, 1*time.Hour)

// Expensive ML inference (reuse predictions)
WithPersistence(store, ScopeGlobal, 24*time.Hour)

// Database lookups (cache query results)
WithPersistence(store, ScopeGlobal, 15*time.Minute)

// Billing operations (ensure exactly-once)
WithPersistence(store, ScopeGlobal, 7*24*time.Hour)
```

## Decision Guide

Choose **ScopeWorkflow** when:
- ✓ Each execution should be independent
- ✓ Results shouldn't persist across runs
- ✓ You want fresh computation each time
- ✓ Data is execution-specific or time-sensitive

Choose **ScopeGlobal** when:
- ✓ Results are expensive to compute
- ✓ Same inputs should return cached results
- ✓ You need exactly-once semantics
- ✓ Operations are idempotent and reusable

## Implementation Details

### ScopeWorkflow Cache Key
```
execution_id + step_name + sha256(marshal(inputs))
```
Example: `"exec_12345_fetch_user_a1b2c3d4"`

### ScopeGlobal Cache Key
```
step_name + sha256(marshal(inputs))
```
Example: `"fetch_user_a1b2c3d4"`

The execution_id makes ScopeWorkflow cache entries unique per execution, while ScopeGlobal shares entries across all executions.

## Related Examples

- `examples/exactly_once` - Exactly-once semantics with ScopeGlobal
- `examples/crash_recovery` - Recovery using persistent checkpoints
- `examples/persistence` - Basic persistence setup
- `examples/persistence_sqlite` - Durable storage backend
