package protograph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "protograph/proto"
)

// Execute runs the graph to produce the requested output type.
//
// Dependencies are resolved in parallel when possible. If a step depends on
// multiple independent types, they are computed concurrently.
//
// Parameters:
//   - ctx: Context for cancellation, deadlines, and tracing
//   - g: The validated graph
//   - executionID: Unique identifier for this execution (for logging, checkpointing)
//   - inputs: Leaf proto inputs provided by caller
//
// Returns the output proto and any error encountered.
//
// Example:
//
//	tags, err := protograph.Execute[*pb.TagSet](ctx, g, "exec-123", &pb.MovieID{Id: "tt0111161"})
func Execute[Out proto.Message](
	ctx context.Context,
	g *Graph,
	executionID string,
	inputs ...proto.Message,
) (Out, error) {
	var zero Out

	// Create zero value to get output type
	outputType := reflect.TypeOf(zero)

	result, err := g.execute(ctx, executionID, outputType, inputs)
	if err != nil {
		return zero, err
	}

	return result.(Out), nil
}

// execute is the internal implementation of Execute.
func (g *Graph) execute(
	ctx context.Context,
	executionID string,
	outputType reflect.Type,
	inputs []proto.Message,
) (proto.Message, error) {
	if !g.validated {
		return nil, fmt.Errorf("graph must be validated before execution")
	}

	// Create execution context with ID
	ctx = withExecutionID(ctx, executionID)

	// Apply default timeout if configured
	if g.defaultTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.defaultTimeout)
		defer cancel()
	}

	// Create executor state with thread-safe cache
	exec := &executor{
		graph:       g,
		executionID: executionID,
		cache:       make(map[reflect.Type]proto.Message),
		inFlight:    make(map[reflect.Type]*inflightResult),
	}

	// Populate cache with leaf inputs
	for _, input := range inputs {
		inputType := reflect.TypeOf(input)
		exec.cache[inputType] = input
	}

	// Resolve the output type
	// Run in goroutine to allow context cancellation to interrupt waiting for unresponsive steps
	type result struct {
		val proto.Message
		err error
	}
	ch := make(chan result, 1)

	go func() {
		val, err := exec.resolve(ctx, outputType)
		ch <- result{val, err}
	}()

	select {
	case res := <-ch:
		return res.val, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// executor holds state for a single execution.
type executor struct {
	graph       *Graph
	executionID string

	mu       sync.Mutex
	cache    map[reflect.Type]proto.Message
	inFlight map[reflect.Type]*inflightResult
}

// inflightResult tracks an in-progress resolution to avoid duplicate work.
type inflightResult struct {
	done   chan struct{}
	result proto.Message
	err    error
}

// resolve recursively computes a proto type by resolving dependencies in parallel.
func (e *executor) resolve(ctx context.Context, targetType reflect.Type) (proto.Message, error) {
	// Fast path: check cache without lock for leaf inputs
	e.mu.Lock()

	// Check cache first (includes leaf inputs)
	if cached, ok := e.cache[targetType]; ok {
		e.mu.Unlock()
		return cached, nil
	}

	// Check if already in-flight (another goroutine is computing this)
	if inflight, ok := e.inFlight[targetType]; ok {
		e.mu.Unlock()
		// Wait for the other goroutine to finish
		select {
		case <-inflight.done:
			return inflight.result, inflight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Find step that produces this type
	meta := e.graph.steps[targetType]
	if meta == nil {
		e.mu.Unlock()
		return nil, &ExecutionError{
			ExecutionID: e.executionID,
			StepType:    targetType,
			Message:     "no step registered for type (missing leaf input?)",
		}
	}

	// Mark as in-flight
	inflight := &inflightResult{done: make(chan struct{})}
	e.inFlight[targetType] = inflight
	e.mu.Unlock()

	// Resolve all dependencies IN PARALLEL
	deps, err := e.resolveParallel(ctx, meta.dependencies)
	if err != nil {
		inflight.err = fmt.Errorf("resolving dependencies: %w", err)
		close(inflight.done)
		return nil, inflight.err
	}

	// Execute the step with retry logic
	result, err := e.executeWithRetry(ctx, meta, targetType, deps)
	if err != nil {
		inflight.err = err
		close(inflight.done)
		return nil, err
	}

	// Cache and signal completion
	e.mu.Lock()
	e.cache[targetType] = result
	e.mu.Unlock()

	inflight.result = result
	close(inflight.done)

	return result, nil
}

// resolveParallel resolves multiple dependencies concurrently.
func (e *executor) resolveParallel(ctx context.Context, depTypes []reflect.Type) ([]reflect.Value, error) {
	if len(depTypes) == 0 {
		return nil, nil
	}

	// For single dependency, no need for goroutines
	if len(depTypes) == 1 {
		dep, err := e.resolve(ctx, depTypes[0])
		if err != nil {
			return nil, err
		}
		return []reflect.Value{reflect.ValueOf(dep)}, nil
	}

	// Resolve multiple dependencies in parallel
	type result struct {
		index int
		value proto.Message
		err   error
	}

	results := make(chan result, len(depTypes))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, depType := range depTypes {
		go func(idx int, dt reflect.Type) {
			dep, err := e.resolve(ctx, dt)
			results <- result{index: idx, value: dep, err: err}
		}(i, depType)
	}

	// Collect results
	deps := make([]reflect.Value, len(depTypes))
	var firstErr error

	for range depTypes {
		select {
		case r := <-results:
			if r.err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("resolving %v: %w", depTypes[r.index], r.err)
					cancel() // Cancel other goroutines
				}
				continue
			}
			deps[r.index] = reflect.ValueOf(r.value)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return deps, nil
}

// executeWithRetry executes a step with configured retry logic and persistence.
func (e *executor) executeWithRetry(
	ctx context.Context,
	meta *stepMeta,
	outputType reflect.Type,
	deps []reflect.Value,
) (proto.Message, error) {
	// Check persistence first (read cache)
	if meta.config.Store != nil {
		cached, err := checkStore(ctx, meta.config, e.executionID, meta.name, deps)
		if err == nil && cached != nil {
			return cached, nil
		}
		// Ignore store read errors, proceed to execute
	}

	maxAttempts := 1
	if meta.config.RetryConfig != nil {
		maxAttempts = meta.config.RetryConfig.MaxAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Apply timeout if configured
		execCtx := ctx
		var cancel context.CancelFunc
		if meta.config.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, meta.config.Timeout)
		}

		// Add step name to context
		execCtx = withStepName(execCtx, meta.name)

		// Execute the step
		result, err := e.callStep(execCtx, meta, deps)

		if cancel != nil {
			cancel()
		}

		if err == nil {
			// Save successful result if persistence is enabled
			if meta.config.Store != nil {
				_ = saveToStore(ctx, meta.config, e.executionID, meta.name, deps, result, nil)
			}
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if attempt >= maxAttempts {
			break
		}

		if !e.isRetriable(err, meta.config.ErrorClassifier) {
			break
		}

		// Wait before retry
		if meta.config.RetryConfig != nil && meta.config.RetryConfig.Backoff != nil {
			delay := meta.config.RetryConfig.Backoff.NextDelay(attempt)
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, fmt.Errorf("canceled during retry backoff: %w", ctx.Err())
			}
		}
	}

	// Save failure if persistence is enabled
	if meta.config.Store != nil {
		_ = saveToStore(ctx, meta.config, e.executionID, meta.name, deps, nil, lastErr)
	}

	return nil, &ExecutionError{
		ExecutionID: e.executionID,
		StepName:    meta.name,
		StepType:    outputType,
		Cause:       lastErr,
		Message:     fmt.Sprintf("step failed after %d attempts", maxAttempts),
	}
}

// callStep invokes the step function/method with resolved dependencies.
func (e *executor) callStep(
	ctx context.Context,
	meta *stepMeta,
	deps []reflect.Value,
) (result proto.Message, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &StepPanicError{
				ExecutionError: &ExecutionError{
					ExecutionID: e.executionID,
					StepName:    meta.name,
					Message:     "step panicked",
				},
				PanicValue: r,
				Stack:      debug.Stack(),
			}
		}
	}()

	// Build arguments: context + dependencies
	args := make([]reflect.Value, 0, 1+len(deps))
	args = append(args, reflect.ValueOf(ctx))
	args = append(args, deps...)

	// Call the function (stepValue is either a function or a bound method)
	results := meta.stepValue.Call(args)

	// Extract results: (proto.Message, error)
	if len(results) != 2 {
		return nil, fmt.Errorf("step returned %d values, expected 2", len(results))
	}

	// Check error (second return value)
	if !results[1].IsNil() {
		err, ok := results[1].Interface().(error)
		if !ok {
			return nil, fmt.Errorf("step returned non-error as second value: %v", results[1].Interface())
		}
		return nil, err
	}

	// Extract output (first return value)
	if results[0].IsNil() {
		return nil, fmt.Errorf("step returned nil output")
	}

	output, ok := results[0].Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("step returned non-proto output: %T", results[0].Interface())
	}

	return output, nil
}

// isRetriable determines if an error should trigger a retry.
func (e *executor) isRetriable(err error, classifier func(error) bool) bool {
	if classifier != nil {
		return classifier(err)
	}
	// Default: all errors are retriable
	return true
}

// ============ Context Keys ============

type contextKey string

const (
	executionIDKey contextKey = "protograph_execution_id"
	stepNameKey    contextKey = "protograph_step_name"
)

func withExecutionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, executionIDKey, id)
}

func withStepName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, stepNameKey, name)
}

// ExecutionID retrieves the execution ID from context.
// Returns empty string if not in an execution context.
func ExecutionID(ctx context.Context) string {
	if id, ok := ctx.Value(executionIDKey).(string); ok {
		return id
	}
	return ""
}

// StepName retrieves the current step name from context.
// Returns empty string if not in a step context.
func StepName(ctx context.Context) string {
	if name, ok := ctx.Value(stepNameKey).(string); ok {
		return name
	}
	return ""
}

// ============ Batch Execution ============

// ExecuteBatch runs the graph for a batch of items, supporting aggregate steps.
//
// Per-item steps are executed in parallel across all items. Aggregate steps
// process all items at once. Dependencies are also resolved in parallel.
//
// Parameters:
//   - ctx: Context for cancellation, deadlines, and tracing
//   - g: The validated graph
//   - executionID: Unique identifier for this batch execution
//   - inputs: Slice of leaf proto inputs (one per item in the batch)
//
// Returns a slice of output protos (one per item) and any error encountered.
//
// Example:
//
//	// Process a batch of movies
//	inputs := []*pb.Movie{movie1, movie2, movie3}
//	results, err := protograph.ExecuteBatch[*pb.Movie, *pb.EnrichedMovie](
//	    ctx, g, "batch-001", inputs,
//	)
func ExecuteBatch[In, Out proto.Message](
	ctx context.Context,
	g *Graph,
	executionID string,
	inputs []In,
) ([]Out, error) {
	var zeroOut Out
	outputType := reflect.TypeOf(zeroOut)

	// Convert inputs to []proto.Message
	protoInputs := make([]proto.Message, len(inputs))
	for i, input := range inputs {
		protoInputs[i] = input
	}

	results, err := g.executeBatch(ctx, executionID, outputType, protoInputs)
	if err != nil {
		return nil, err
	}

	// Convert results back to typed slice
	typedResults := make([]Out, len(results))
	for i, result := range results {
		typedResults[i] = result.(Out)
	}

	return typedResults, nil
}

// executeBatch is the internal implementation of ExecuteBatch.
func (g *Graph) executeBatch(
	ctx context.Context,
	executionID string,
	outputType reflect.Type,
	inputs []proto.Message,
) ([]proto.Message, error) {
	if !g.validated {
		return nil, fmt.Errorf("graph must be validated before execution")
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("batch inputs cannot be empty")
	}

	// Create execution context with ID
	ctx = withExecutionID(ctx, executionID)

	// Apply default timeout if configured
	if g.defaultTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.defaultTimeout)
		defer cancel()
	}

	// Create batch executor state
	exec := &batchExecutor{
		graph:          g,
		executionID:    executionID,
		batchSize:      len(inputs),
		batchCache:     make(map[reflect.Type][]proto.Message),
		aggregateCache: make(map[reflect.Type]proto.Message),
		inFlight:       make(map[reflect.Type]*inflightBatchResult),
	}

	// Populate batch cache with leaf inputs
	inputType := reflect.TypeOf(inputs[0])
	exec.batchCache[inputType] = inputs

	// Resolve the output type for batch
	// Run in goroutine to allow context cancellation to interrupt waiting for unresponsive steps
	type result struct {
		val []proto.Message
		err error
	}
	ch := make(chan result, 1)

	go func() {
		val, err := exec.resolveBatch(ctx, outputType)
		ch <- result{val, err}
	}()

	select {
	case res := <-ch:
		return res.val, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// batchExecutor holds state for a batch execution.
type batchExecutor struct {
	graph          *Graph
	executionID    string
	batchSize      int

	mu             sync.Mutex
	batchCache     map[reflect.Type][]proto.Message // per-item results
	aggregateCache map[reflect.Type]proto.Message   // aggregate results (shared across all items)
	inFlight       map[reflect.Type]*inflightBatchResult
}

// inflightBatchResult tracks an in-progress batch resolution.
type inflightBatchResult struct {
	done    chan struct{}
	results []proto.Message
	err     error
}

// resolveBatch resolves an output type for all items in the batch.
func (e *batchExecutor) resolveBatch(ctx context.Context, targetType reflect.Type) ([]proto.Message, error) {
	e.mu.Lock()

	// Check batch cache first
	if cached, ok := e.batchCache[targetType]; ok {
		e.mu.Unlock()
		return cached, nil
	}

	// Check if already in-flight
	if inflight, ok := e.inFlight[targetType]; ok {
		e.mu.Unlock()
		select {
		case <-inflight.done:
			return inflight.results, inflight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Find step that produces this type
	meta := e.graph.steps[targetType]
	if meta == nil {
		e.mu.Unlock()
		return nil, &ExecutionError{
			ExecutionID: e.executionID,
			StepType:    targetType,
			Message:     "no step registered for type (missing leaf input?)",
		}
	}

	// Mark as in-flight
	inflight := &inflightBatchResult{done: make(chan struct{})}
	e.inFlight[targetType] = inflight
	e.mu.Unlock()

	var results []proto.Message
	var err error

	if meta.isAggregate {
		results, err = e.resolveAggregate(ctx, meta, targetType)
	} else {
		results, err = e.resolvePerItem(ctx, meta, targetType)
	}

	if err != nil {
		inflight.err = err
		close(inflight.done)
		return nil, err
	}

	// Cache results
	e.mu.Lock()
	e.batchCache[targetType] = results
	e.mu.Unlock()

	inflight.results = results
	close(inflight.done)

	return results, nil
}

// resolveAggregate executes an aggregate step (fan-in) that processes all items at once.
func (e *batchExecutor) resolveAggregate(ctx context.Context, meta *stepMeta, outputType reflect.Type) ([]proto.Message, error) {
	// Check aggregate cache
	e.mu.Lock()
	if cached, ok := e.aggregateCache[outputType]; ok {
		e.mu.Unlock()
		results := make([]proto.Message, e.batchSize)
		for i := range results {
			results[i] = cached
		}
		return results, nil
	}
	e.mu.Unlock()

	// Resolve all dependencies as batches IN PARALLEL
	depBatches, err := e.resolveBatchParallel(ctx, meta.dependencies)
	if err != nil {
		return nil, err
	}

	// Convert to typed slices for the aggregate step
	typedDepBatches := make([]reflect.Value, len(meta.dependencies))
	for i, depType := range meta.dependencies {
		sliceType := reflect.SliceOf(depType)
		sliceVal := reflect.MakeSlice(sliceType, len(depBatches[i]), len(depBatches[i]))
		for j, item := range depBatches[i] {
			sliceVal.Index(j).Set(reflect.ValueOf(item))
		}
		typedDepBatches[i] = sliceVal
	}

	// Execute the aggregate step WITH RETRY
	result, err := e.executeAggregateWithRetry(ctx, meta, outputType, typedDepBatches)
	if err != nil {
		return nil, err
	}

	// Cache the aggregate result
	e.mu.Lock()
	e.aggregateCache[outputType] = result
	e.mu.Unlock()

	// Return the same result for all items
	results := make([]proto.Message, e.batchSize)
	for i := range results {
		results[i] = result
	}

	return results, nil
}

// executeAggregateWithRetry executes an aggregate step with configured retry logic.
func (e *batchExecutor) executeAggregateWithRetry(
	ctx context.Context,
	meta *stepMeta,
	outputType reflect.Type,
	depBatches []reflect.Value,
) (proto.Message, error) {
	// Check persistence first (read cache)
	if meta.config.Store != nil {
		// Aggregate key is global or workflow. If global, depends on input hash.
		// Input is depBatches (slices of protos). Need to hash ALL items.
		cached, err := checkStore(ctx, meta.config, e.executionID, meta.name, depBatches)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	maxAttempts := 1
	if meta.config.RetryConfig != nil {
		maxAttempts = meta.config.RetryConfig.MaxAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Apply timeout if configured
		execCtx := ctx
		var cancel context.CancelFunc
		if meta.config.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, meta.config.Timeout)
		}

		// Add step name to context
		execCtx = withStepName(execCtx, meta.name)

		// Execute the aggregate step
		result, err := e.callAggregateStep(execCtx, meta, depBatches)

		if cancel != nil {
			cancel()
		}

		if err == nil {
			// Save successful result
			if meta.config.Store != nil {
				_ = saveToStore(ctx, meta.config, e.executionID, meta.name, depBatches, result, nil)
			}
			return result, nil
		}

		lastErr = err

		// Check if context was cancelled (don't retry)
		if ctx.Err() != nil {
			break
		}

		// Check if we should retry
		if attempt >= maxAttempts {
			break
		}

		if !e.isRetriable(err, meta.config.ErrorClassifier) {
			break
		}

		// Wait before retry
		if meta.config.RetryConfig != nil && meta.config.RetryConfig.Backoff != nil {
			delay := meta.config.RetryConfig.Backoff.NextDelay(attempt)
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, fmt.Errorf("aggregate: canceled during retry backoff: %w", ctx.Err())
			}
		}
	}

	// Save failure
	if meta.config.Store != nil {
		_ = saveToStore(ctx, meta.config, e.executionID, meta.name, depBatches, nil, lastErr)
	}

	return nil, &ExecutionError{
		ExecutionID: e.executionID,
		StepName:    meta.name,
		StepType:    outputType,
		Cause:       lastErr,
		Message:     fmt.Sprintf("aggregate step failed after %d attempts", maxAttempts),
	}
}

// resolvePerItem executes a regular step for each item in the batch IN PARALLEL.
func (e *batchExecutor) resolvePerItem(ctx context.Context, meta *stepMeta, outputType reflect.Type) ([]proto.Message, error) {
	// Resolve all dependencies as batches first (in parallel)
	depBatches, err := e.resolveBatchParallel(ctx, meta.dependencies)
	if err != nil {
		return nil, err
	}

	// Execute step for each item IN PARALLEL
	results := make([]proto.Message, e.batchSize)
	errChan := make(chan error, e.batchSize)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Concurrency control
	maxConcurrent := meta.config.MaxConcurrency
	if maxConcurrent == 0 {
		maxConcurrent = 100 // Safe default
	} else if maxConcurrent < 0 {
		maxConcurrent = e.batchSize // Unlimited
	}
	if maxConcurrent > e.batchSize {
		maxConcurrent = e.batchSize
	}
	sem := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	for itemIdx := 0; itemIdx < e.batchSize; itemIdx++ {
		wg.Add(1)

		// Acquire semaphore
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Done()
			return nil, ctx.Err()
		}

		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			// Gather dependencies for this item
			deps := make([]reflect.Value, len(meta.dependencies))
			for depIdx, depBatch := range depBatches {
				deps[depIdx] = reflect.ValueOf(depBatch[idx])
			}

			// Execute the step for this item WITH RETRY
			result, err := e.executeStepWithRetry(ctx, meta, outputType, deps, idx)
			if err != nil {
				select {
				case errChan <- err:
					cancel() // Cancel other goroutines
				default:
				}
				return
			}
			results[idx] = result
		}(itemIdx)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if err := <-errChan; err != nil {
		return nil, err
	}

	return results, nil
}

// executeStepWithRetry executes a per-item step with configured retry logic.
func (e *batchExecutor) executeStepWithRetry(
	ctx context.Context,
	meta *stepMeta,
	outputType reflect.Type,
	deps []reflect.Value,
	itemIndex int,
) (proto.Message, error) {
	// Check persistence first (read cache)
	if meta.config.Store != nil {
		cached, err := checkStore(ctx, meta.config, e.executionID, meta.name, deps)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	maxAttempts := 1
	if meta.config.RetryConfig != nil {
		maxAttempts = meta.config.RetryConfig.MaxAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Apply timeout if configured
		execCtx := ctx
		var cancel context.CancelFunc
		if meta.config.Timeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, meta.config.Timeout)
		}

		// Add step name to context
		execCtx = withStepName(execCtx, meta.name)

		// Execute the step
		result, err := e.callStep(execCtx, meta, deps)

		if cancel != nil {
			cancel()
		}

		if err == nil {
			// Save successful result
			if meta.config.Store != nil {
				_ = saveToStore(ctx, meta.config, e.executionID, meta.name, deps, result, nil)
			}
			return result, nil
		}

		lastErr = err

		// Check if context was cancelled (don't retry)
		if ctx.Err() != nil {
			break
		}

		// Check if we should retry
		if attempt >= maxAttempts {
			break
		}

		if !e.isRetriable(err, meta.config.ErrorClassifier) {
			break
		}

		// Wait before retry
		if meta.config.RetryConfig != nil && meta.config.RetryConfig.Backoff != nil {
			delay := meta.config.RetryConfig.Backoff.NextDelay(attempt)
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, fmt.Errorf("item %d: canceled during retry backoff: %w", itemIndex, ctx.Err())
			}
		}
	}

	// Save failure
	if meta.config.Store != nil {
		_ = saveToStore(ctx, meta.config, e.executionID, meta.name, deps, nil, lastErr)
	}

	return nil, &ExecutionError{
		ExecutionID: e.executionID,
		StepName:    meta.name,
		StepType:    outputType,
		Cause:       lastErr,
		Message:     fmt.Sprintf("item %d: step failed after %d attempts", itemIndex, maxAttempts),
	}
}

// isRetriable determines if an error should trigger a retry.
func (e *batchExecutor) isRetriable(err error, classifier func(error) bool) bool {
	if classifier != nil {
		return classifier(err)
	}
	// Default: all errors are retriable
	return true
}

// resolveBatchParallel resolves multiple dependency types concurrently.
func (e *batchExecutor) resolveBatchParallel(ctx context.Context, depTypes []reflect.Type) ([][]proto.Message, error) {
	if len(depTypes) == 0 {
		return nil, nil
	}

	// For single dependency, no need for goroutines
	if len(depTypes) == 1 {
		batch, err := e.resolveBatch(ctx, depTypes[0])
		if err != nil {
			return nil, err
		}
		return [][]proto.Message{batch}, nil
	}

	// Resolve multiple dependencies in parallel
	type result struct {
		index int
		batch []proto.Message
		err   error
	}

	results := make(chan result, len(depTypes))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, depType := range depTypes {
		go func(idx int, dt reflect.Type) {
			batch, err := e.resolveBatch(ctx, dt)
			results <- result{index: idx, batch: batch, err: err}
		}(i, depType)
	}

	// Collect results
	batches := make([][]proto.Message, len(depTypes))
	var firstErr error

	for range depTypes {
		select {
		case r := <-results:
			if r.err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("resolving %v: %w", depTypes[r.index], r.err)
					cancel()
				}
				continue
			}
			batches[r.index] = r.batch
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return batches, nil
}

// callAggregateStep invokes an aggregate step with batch dependencies.
func (e *batchExecutor) callAggregateStep(
	ctx context.Context,
	meta *stepMeta,
	depBatches []reflect.Value,
) (result proto.Message, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &StepPanicError{
				ExecutionError: &ExecutionError{
					ExecutionID: e.executionID,
					StepName:    meta.name,
					Message:     "step panicked",
				},
				PanicValue: r,
				Stack:      debug.Stack(),
			}
		}
	}()

	// Add step name to context
	ctx = withStepName(ctx, meta.name)

	// Build arguments: context + batch dependencies
	args := make([]reflect.Value, 0, 1+len(depBatches))
	args = append(args, reflect.ValueOf(ctx))
	args = append(args, depBatches...)

	// Call the function
	results := meta.stepValue.Call(args)

	// Extract results
	if len(results) != 2 {
		return nil, fmt.Errorf("aggregate step returned %d values, expected 2", len(results))
	}

	// Check error
	if !results[1].IsNil() {
		err, ok := results[1].Interface().(error)
		if !ok {
			return nil, fmt.Errorf("step returned non-error as second value: %v", results[1].Interface())
		}
		return nil, err
	}

	// Extract output
	if results[0].IsNil() {
		return nil, fmt.Errorf("aggregate step returned nil output")
	}

	output, ok := results[0].Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("step returned non-proto output: %T", results[0].Interface())
	}

	return output, nil
}

// callStep invokes a regular step for a single item.
func (e *batchExecutor) callStep(
	ctx context.Context,
	meta *stepMeta,
	deps []reflect.Value,
) (result proto.Message, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &StepPanicError{
				ExecutionError: &ExecutionError{
					ExecutionID: e.executionID,
					StepName:    meta.name,
					Message:     "step panicked",
				},
				PanicValue: r,
				Stack:      debug.Stack(),
			}
		}
	}()

	// Add step name to context
	ctx = withStepName(ctx, meta.name)

	// Build arguments: context + dependencies
	args := make([]reflect.Value, 0, 1+len(deps))
	args = append(args, reflect.ValueOf(ctx))
	args = append(args, deps...)

	// Call the function (stepValue is either a function or a bound method)
	results := meta.stepValue.Call(args)

	// Extract results
	if len(results) != 2 {
		return nil, fmt.Errorf("step returned %d values, expected 2", len(results))
	}

	// Check error
	if !results[1].IsNil() {
		err, ok := results[1].Interface().(error)
		if !ok {
			return nil, fmt.Errorf("step returned non-error as second value: %v", results[1].Interface())
		}
		return nil, err
	}

	// Extract output
	if results[0].IsNil() {
		return nil, fmt.Errorf("step returned nil output")
	}

	output, ok := results[0].Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("step returned non-proto output: %T", results[0].Interface())
	}

	return output, nil
}

// ============ Persistence Helpers ============

func checkStore(
	ctx context.Context,
	config *StepConfig,
	executionID, stepName string,
	deps []reflect.Value,
) (proto.Message, error) {
	key, err := calculateCacheKey(config.PersistenceScope, executionID, stepName, deps)
	if err != nil {
		return nil, err
	}

	entry, err := config.Store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	// If stored entry has an error, we assume it's a failed step.
	// For caching purposes, we typically want to retry failures, so we return nil (not found)
	// unless we specifically want to cache failures. For now, treating error as "no result".
	if entry.Error != "" {
		return nil, nil
	}

	if entry.Value == nil {
		return nil, nil
	}

	// Unmarshal Any to the expected type?
	// Actually the Any contains the type URL. We need to unmarshal it.
	// But we don't know the exact type here easily without reflecting on the return type of the step again.
	// However, proto.Message is an interface. anypb.UnmarshalTo can unmarshal to a provided message.
	// Or anypb.UnmarshalNew() creates a new message of the type.
	
	// NOTE: This is a bit tricky because we need to return a proto.Message that matches what the graph expects.
	// entry.Value.UnmarshalNew() returns a proto.Message. This should work if the type URL matches.
	
	msg, err := entry.Value.UnmarshalNew()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached value: %w", err)
	}
	
	return msg, nil
}

func saveToStore(
	ctx context.Context,
	config *StepConfig,
	executionID, stepName string,
	deps []reflect.Value,
	result proto.Message,
	stepErr error,
) error {
	key, err := calculateCacheKey(config.PersistenceScope, executionID, stepName, deps)
	if err != nil {
		return err
	}

	entry := &pb.PersistenceEntry{
		CreatedAt: timestamppb.Now(),
	}

	if config.TTL > 0 {
		entry.ExpiresAt = timestamppb.New(time.Now().Add(config.TTL))
	}

	if stepErr != nil {
		entry.Error = stepErr.Error()
	} else if result != nil {
		anyVal, err := anypb.New(result)
		if err != nil {
			return fmt.Errorf("failed to marshal result to Any: %w", err)
		}
		entry.Value = anyVal
	}

	return config.Store.Set(ctx, key, entry)
}

func calculateCacheKey(scope PersistenceScope, executionID, stepName string, deps []reflect.Value) (string, error) {
	// We calculate the hash of inputs for ALL scopes.
	// This ensures that even within a workflow, if the input changes (e.g. in a batch loop),
	// we generate a unique key.
	hash := sha256.New()

	for _, dep := range deps {
		msg, ok := dep.Interface().(proto.Message)
		if !ok {
			// Handle slice case for aggregate steps
			if dep.Kind() == reflect.Slice {
				// It's a slice of protos
				length := dep.Len()
				for i := 0; i < length; i++ {
					item := dep.Index(i).Interface().(proto.Message)

					// Write Type Name
					hash.Write([]byte(item.ProtoReflect().Descriptor().FullName()))
					hash.Write([]byte("|"))

					bytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(item)
					if err != nil {
						return "", fmt.Errorf("failed to marshal input for hashing: %w", err)
					}
					hash.Write(bytes)
				}
				continue
			}

			return "", fmt.Errorf("dependency is not a proto message or slice")
		}

		// Write Type Name to ensure unique keys even if wire format is identical
		hash.Write([]byte(msg.ProtoReflect().Descriptor().FullName()))
		hash.Write([]byte("|")) // Separator

		bytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
		if err != nil {
			return "", fmt.Errorf("failed to marshal input for hashing: %w", err)
		}
		hash.Write(bytes)
	}

	sum := hash.Sum(nil)
	hashStr := hex.EncodeToString(sum)

	if scope == ScopeWorkflow {
		return fmt.Sprintf("workflow:%s:%s:%s", executionID, stepName, hashStr), nil
	}

	// ScopeGlobal
	return fmt.Sprintf("global:%s:%s", stepName, hashStr), nil
}
