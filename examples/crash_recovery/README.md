# Crash Recovery Example

This example demonstrates how Docket's persistence enables **crash recovery** by checkpointing completed steps and restoring them on retry.

## What This Demonstrates

When a step fails in the middle of a pipeline:
1. **Previously completed steps** are checkpointed and their results are persisted
2. **On retry**, completed steps are restored from checkpoints (not re-executed)
3. **Only failed steps** and downstream dependencies need to execute

## Pipeline

```
IngestData (Step1) → ProcessData (Step2) → EnrichData (Step3)
```

## Execution Flow

### First Attempt (Crash)
1. **Step1 (IngestData)**: ✓ Executes successfully → Result checkpointed
2. **Step2 (ProcessData)**: ✗ Fails (simulated crash) → No checkpoint created
3. **Step3 (EnrichData)**: Never attempted (dependency failed)

### Second Attempt (Recovery)
1. **Step1 (IngestData)**: Restored from checkpoint (NOT re-executed)
2. **Step2 (ProcessData)**: ✓ Retries and succeeds → Result checkpointed
3. **Step3 (EnrichData)**: ✓ Executes for first time → Result checkpointed

## Running the Example

```bash
go run examples/crash_recovery/main.go
```

## Expected Output

```
=== Crash Recovery Example ===

--- First Attempt: Simulating Crash ---

=== Attempt #1 ===
Configuration: Step2 will FAIL (simulating crash)

  [STEP 1] IngestData executing (run #1)
  [STEP 2] ProcessData executing (run #1)
    ⚠️  CRASH: Simulated failure in ProcessData

❌ Execution failed: simulated crash during processing

--- Second Attempt: Recovery ---

=== Attempt #2 ===
Configuration: All steps should succeed (recovery mode)

  [STEP 2] ProcessData executing (run #2)
  [STEP 3] EnrichData executing (run #1)

✓ Execution succeeded!

=== Execution Summary ===
Step1 (IngestData) executions:  1 (expected: 1)
Step2 (ProcessData) executions: 2 (expected: 2)
Step3 (EnrichData) executions:  1 (expected: 1)

✓ Step1: Executed once (checkpoint restored on retry)
✓ Step2: Executed twice (failed first, succeeded on retry)
✓ Step3: Executed once (only after Step2 succeeded)

🎉 SUCCESS: Crash recovery working correctly!
```

## Key Concepts

### Checkpointing

Docket automatically checkpoints step results when persistence is enabled:
- Results saved with key: `(execution_context, step_name, input_hash)`
- Checkpoints survive process crashes/restarts (with durable stores)
- Only successful step executions create checkpoints

### Recovery Behavior

On retry after failure:
1. Graph attempts to resolve each dependency
2. For each step, checks if checkpoint exists
3. If checkpoint found → restore result (skip execution)
4. If no checkpoint → execute step normally

### Efficiency Gains

Crash recovery is especially valuable when:
- **Early steps are expensive** (API calls, database queries, computations)
- **Failures are transient** (network issues, rate limits)
- **Partial progress is valuable** (multi-stage ETL pipelines)

## Use Cases

### Data Pipelines
```
Extract → Transform → Load
   ✓         ✗        -
```
On retry: Skip expensive extraction, retry transform

### ML Training
```
LoadData → Preprocess → Train → Evaluate
    ✓          ✓          ✗       -
```
On retry: Restore preprocessed data, resume training

### External API Workflows
```
FetchUserData → CallPaymentAPI → SendNotification
      ✓               ✗                -
```
On retry: Avoid duplicate user data fetch, retry payment

## Configuration Options

### Persistence Scope

```go
// ScopeGlobal: Checkpoints shared across all executions
docket.WithPersistence(store, docket.ScopeGlobal, ttl)

// ScopeWorkflow: Checkpoints only within single execution
docket.WithPersistence(store, docket.ScopeWorkflow, ttl)
```

For crash recovery, use **ScopeGlobal** to preserve checkpoints across process restarts.

### TTL (Time To Live)

```go
// Short-lived recovery window
docket.WithPersistence(store, scope, 5*time.Minute)

// Long-lived recovery window
docket.WithPersistence(store, scope, 24*time.Hour)
```

Choose TTL based on how long partial results remain valid.

## Related Examples

- `examples/retry` - Retry configuration and error handling
- `examples/exactly_once` - Exactly-once execution semantics
- `examples/persistence_sqlite` - Durable checkpoint storage
