# Exactly-Once Semantics Example

This example demonstrates **exactly-once execution semantics** using Docket's `ScopeGlobal` persistence.

## What This Demonstrates

When a graph is configured with `ScopeGlobal` persistence, Docket guarantees that a step will execute **exactly once** for any given set of inputs, even across multiple independent graph executions.

## How It Works

1. **First Execution**: The step runs and its result is cached with a key based on `(step_name, hash(inputs))`
2. **Second Execution**: Same inputs → cache hit → step does NOT execute again
3. **Third Execution**: Different inputs → cache miss → step executes with new inputs

## Running the Example

```bash
go run examples/exactly_once/main.go
```

## Expected Output

```
=== Exactly-Once Semantics Demo ===

--- First Execution ---
Expected: Step will execute (cache miss)

  [EXECUTING] ExpensiveComputation for user Alice (execution #1)

Result: Processed: Alice
Execution count: 1

--- Second Execution (Same Input) ---
Expected: Step will NOT execute (cache hit - exactly once guarantee)

Result: Processed: Alice
Execution count: 1 (should still be 1!)

✓ SUCCESS: Exactly-once semantics verified!

--- Third Execution (Different Input) ---
Expected: Step WILL execute (different inputs = different cache key)

  [EXECUTING] ExpensiveComputation for user Bob (execution #2)

Result: Processed: Bob
Execution count: 2 (should be 2 now)
```

## Key Concepts

### Exactly-Once Guarantee

With `ScopeGlobal`:
- Results are cached based on `(step_name, input_hash)`
- Same inputs always return cached results
- Different inputs trigger new executions
- Cache persists across graph instances

### Use Cases

Perfect for:
- **Expensive computations**: Avoid recomputing costly operations
- **External API calls**: Prevent duplicate requests
- **Database writes**: Ensure idempotent operations
- **Billing operations**: Guarantee charges happen exactly once

### Comparison with ScopeWorkflow

| Scope | Cache Lifetime | Use Case |
|-------|---------------|----------|
| `ScopeGlobal` | Shared across all executions | Exactly-once guarantee globally |
| `ScopeWorkflow` | Per-execution only | Deduplication within single execution |

## Related Examples

- `examples/persistence` - Basic persistence setup
- `examples/persistence_sqlite` - SQLite-backed persistence
- `examples/scope_comparison` - Side-by-side scope behavior comparison
