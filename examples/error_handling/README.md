# Error Handling Example

This example demonstrates Docket's error handling patterns and semantics.

## Running the Example

```bash
go run main.go
```

## Error Patterns Demonstrated

### 1. Custom Error Types

The example defines custom error types for different failure scenarios:

- **ValidationError**: Represents invalid input (non-retriable)
- **NetworkError**: Represents transient failures (retriable)
- **AbortError**: Signals critical failures that should stop the entire graph

### 2. ErrorClassifier

The `FetchDataFromAPI` step uses an `ErrorClassifier` to control retry behavior:

```go
docket.WithErrorClassifier(func(err error) bool {
    // ValidationErrors are permanent - don't retry
    var validationErr *ValidationError
    if errors.As(err, &validationErr) {
        return false // Don't retry
    }

    // NetworkErrors are transient - retry them
    var networkErr *NetworkError
    if errors.As(err, &networkErr) {
        return true // Retry
    }

    return true // Default: retry
})
```

### 3. Stop Just a Step vs. Stop the Whole Graph

**Stop a single step** (don't retry this specific step):
- Return an error and configure `ErrorClassifier` to return `false`
- Example: `ValidationError` in this demo

**Stop the whole graph** (cancel all parallel work):
- Return an `AbortError`
- Example: `critical` input triggers `docket.NewAbortError()`
- The executor automatically marks `AbortError` as non-retriable

### 4. Context Cancellation

Context cancellation propagates through the entire graph:
- Timeout contexts (`context.WithTimeout`)
- Manual cancellation (`context.WithCancel`)
- All in-flight goroutines respect `ctx.Done()`

## Test Coverage

Run the tests to verify all error scenarios:

```bash
go test -v
```

Tests verify:
- ✅ Valid input succeeds
- ✅ Empty input fails with ValidationError (no retry)
- ✅ Long input fails with ValidationError (no retry)
- ✅ "flaky" input retries 3 times, then fails
- ✅ "critical" input returns AbortError immediately
- ✅ Context timeout cancels execution
- ✅ Context cancellation stops execution

## Key Takeaways

1. **ErrorClassifier gives fine-grained control**: You can classify errors per-step
2. **AbortError stops everything**: Use it for critical failures
3. **Context propagation is automatic**: All steps respect context cancellation
4. **Custom error types are encouraged**: Make your error semantics explicit
