CORE_INTERFACES.md 
# Protograph: Technical Architecture

## Core Interfaces

**Step Interface**
```go
// Step represents a computation that transforms proto inputs into a proto output
type Step[Out proto.Message, Deps any] interface {
    // Compute executes the step's logic
    // ctx: execution context with cancellation, timeouts, execution ID
    // deps: struct containing all proto dependencies, resolved by the graph
    Compute(ctx context.Context, deps Deps) (Out, error)
}
```

The Step interface is intentionally minimal. Steps are stateless functions that transform data. All graph-lifetime dependencies (API clients, databases) are stored as fields on the concrete step struct, injected via constructor. All execution-lifetime context flows through the `ctx` parameter. All proto dependencies are in the `deps` struct, with fields matching the proto types the step needs.

**Graph Interface**
```go
// Graph manages step registration and dependency resolution
type Graph interface {
    // Register adds a step to the graph with optional configuration
    Register(step interface{}, opts ...StepOption) error
    
    // Validate checks that all dependencies can be resolved, no cycles exist,
    // and all runtime dependencies are provided
    Validate() error
    
    // GetExecutionPlan returns the ordered list of steps needed to compute outputType
    GetExecutionPlan(outputType reflect.Type, leafInputs map[reflect.Type]proto.Message) ([]Step, error)
}
```

The graph is built at application startup and remains immutable afterward. Registration uses reflection to inspect step constructors and `Compute` method signatures, extracting output types, dependency types, and runtime requirements. Validation runs after all steps are registered, building the complete dependency graph and checking for errors. The execution plan shows which steps will run for a given request without actually executing them—useful for debugging and visualization.

**Executor Interface**
```go
// Executor orchestrates step execution with dependency resolution
type Executor interface {
    // Compute synchronously executes the pipeline and returns the result
    // executionID: unique identifier for tracking and checkpointing
    // output: the desired output type (empty proto instance)
    // inputs: leaf inputs (proto instances with data)
    Compute(ctx context.Context, executionID string, output proto.Message, inputs ...proto.Message) (proto.Message, error)
}
```

The executor is the runtime component. Given a desired output type and leaf inputs, it walks the dependency graph recursively: for each step's dependencies, check if already computed (in-memory cache), recurse if needed, execute the step with resolved dependencies, cache the result in memory. If any step fails, the entire execution fails—checkpoint logic (added in Milestone 1) changes this to enable resume on retry.

**StepOption Interface**
```go
// StepOption configures a step's behavior at registration time
type StepOption interface {
    apply(*stepConfig)
}

// Common options
func WithRetry(maxAttempts int, backoff BackoffStrategy) StepOption
func WithTimeout(duration time.Duration) StepOption
func WithRetryableErrors(classifier func(error) bool) StepOption
```

Options are applied at registration, stored in the graph's metadata for each step, and consulted during execution. The options pattern allows extending configuration without breaking existing code. Retry configuration is essential for MVP (handle transient failures), timeout prevents steps from running forever, and retryable error classification lets steps distinguish "network timeout, retry" from "invalid input, don't retry."

## Data Flow

**Registration Phase (Application Startup):**

1. Developer creates step structs with constructors that declare runtime dependencies
2. Developer calls `graph.Register()` for each step, graph uses reflection to inspect constructor signature and extract required runtime dependencies (API clients, DB connections)
3. Graph uses reflection on step's `Compute` method to extract output type and proto dependencies
4. Graph stores: `outputType -> (step instance, dependency types, config options)`
5. After all registrations, developer calls `graph.Validate()`
6. Validation builds complete dependency graph, checks for cycles, verifies one step per output type, confirms all dependencies can be resolved

**Execution Phase (Per Request):**

1. Caller invokes `executor.Compute(ctx, executionID, outputType, leafInputs...)`
2. Executor creates execution context: wraps ctx with execution ID, initializes in-memory result cache
3. Executor calls internal `resolve(outputType)` function
4. Resolve checks in-memory cache: if output already computed this execution, return it
5. If not cached, find step registered for outputType
6. For each dependency of that step, recursively call `resolve(dependencyType)`
7. Once all dependencies resolved, construct deps struct with resolved protos
8. Call `step.Compute(ctx, deps)`, handle retries if configured
9. Store result in in-memory cache, return to caller
10. When top-level resolve completes, return final output to original caller

**Error Flow:**

1. If any step's `Compute` returns error, check if error is retriable (using classifier if configured)
2. If retriable and attempts remaining, wait according to backoff strategy, retry
3. If not retriable or max attempts exceeded, propagate error up the call stack
4. Entire execution fails, return error to caller with context about which step failed

## Testing Strategy

**Unit Testing Steps:** Steps are pure functions with injected dependencies, making them trivial to test. Create step instance with mock clients, call `Compute` directly with test proto instances, assert output matches expected. No framework, no test harness, just standard Go table-driven tests.

**Testing Graph Validation:** Create graph, register steps with various dependency patterns (valid, missing dependencies, cycles), call `Validate()`, assert expected errors. Tests ensure bad configurations are caught at startup, not discovered at runtime.

**Integration Testing Execution:** Create real graph with real steps (but mock external services), execute full pipeline, verify correct output produced. Assert execution order (using spy pattern or examining logs), verify each step received correct inputs, confirm error handling (inject failures, verify retries).

**Testing Retry Logic:** Create step that fails N times then succeeds, configure retry with specific backoff, execute, verify step was called N+1 times with correct delays between attempts. Test retriable vs non-retriable errors, test max attempts exceeded.

## Implementation Guidance

**Start with the Step interface and proto definitions.** Define your movie tagging protos first (`MovieID`, `Movie`, `DirectorProfile`, `ContentAnalysis`, `Classification`, `TagSet`). Then implement each step as a standalone struct with a `Compute` method. Get these working in isolation with unit tests before touching the graph or executor.

**Build the Graph incrementally.** Start with registration that just stores steps in a map: `outputType -> step`. Add validation one check at a time: verify output types are unique, verify dependencies exist, detect cycles with topological sort. Test each validation rule independently.

**Implement naive Executor first.** Don't optimize. Recursively resolve dependencies with simple in-memory map cache. Get the basic flow working: resolve dependencies, call Compute, cache result, return. Then add retry logic. Then add timeout handling. Build features incrementally with tests confirming each works.

**Use reflection carefully.** Reflection is needed for registration (extracting types from constructors and method signatures) but should be minimized at execution time. Cache reflection results during registration—store reflect.Type for outputs and dependencies, not function signatures. Execution should mostly be direct function calls on cached step instances.

**Proto marshaling for cache keys.** When computing cache keys (in-memory cache for MVP, checkpoint keys in M1), marshal proto inputs to bytes, hash them. Use deterministic marshaling (protocol buffers are deterministic by default). Cache key format: `outputType + hash(marshaled inputs)`.

**Error handling conventions.** Steps return `(proto.Message, error)`. Nil error means success. Non-nil error fails the step. Distinguish error types with custom error types that implement `Retriable() bool` method, or use error wrapping with sentinel errors. Executor checks error type to decide whether to retry.

**Context usage.** Every `Compute` method receives `context.Context` as first parameter. Use it for cancellation (respect ctx.Done()), timeouts (context.WithTimeout at executor level), and execution metadata (store execution ID in context with type-safe key). Follow Go context conventions—don't store context in struct fields, pass it explicitly.

**Constructor-based DI mechanics.** When registering a step, the developer passes a constructor function, not a step instance. The graph inspects the constructor's parameters (all must be runtime dependencies like `*sql.DB`, `APIClient`), verifies those types are available in the RuntimeContext provided at graph creation, calls the constructor with those dependencies, and stores the resulting step instance. This happens once at startup, creating properly initialized steps ready for execution.

