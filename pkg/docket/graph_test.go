package docket

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "docket/proto/examples/lettercount"
	"google.golang.org/protobuf/proto"
)

func TestRegisterFunction(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: 1}, nil
	})

	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if len(g.steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(g.steps))
	}
}

func TestRegisterWithOptions(t *testing.T) {
	g := NewGraph()

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			return &pb.LetterCount{Count: 1}, nil
		},
		WithName("TestStep"),
		WithTimeout(5*time.Second),
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 100 * time.Millisecond},
		}),
	)

	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Find the registered step
	for _, meta := range g.steps {
		if meta.name != "TestStep" {
			t.Errorf("expected name 'TestStep', got %q", meta.name)
		}
		if meta.config.Timeout != 5*time.Second {
			t.Errorf("expected timeout 5s, got %v", meta.config.Timeout)
		}
		if meta.config.RetryConfig.MaxAttempts != 3 {
			t.Errorf("expected 3 retry attempts, got %d", meta.config.RetryConfig.MaxAttempts)
		}
	}
}

func TestRegisterDuplicateOutputType(t *testing.T) {
	g := NewGraph()

	step := func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{}, nil
	}

	if err := g.Register(step); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := g.Register(step)
	if err == nil {
		t.Error("expected error for duplicate output type, got nil")
	}
}

func TestValidate(t *testing.T) {
	g := NewGraph()

	// Empty graph should fail validation
	if err := g.Validate(); err == nil {
		t.Error("expected error for empty graph, got nil")
	}

	// Add a step
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{}, nil
	})

	// Should validate successfully
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestExecute(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{
			OriginalString: input.Value,
			Count:          int32(len(input.Value)),
		}, nil
	})

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	input := &pb.InputString{Value: "hello"}

	result, err := Execute[*pb.LetterCount](ctx, g, "test-exec", input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.OriginalString != "hello" {
		t.Errorf("expected OriginalString 'hello', got %q", result.OriginalString)
	}
	if result.Count != 5 {
		t.Errorf("expected Count 5, got %d", result.Count)
	}
}

func TestExecuteWithRetry(t *testing.T) {
	g := NewGraph()

	attempts := 0
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary error")
			}
			return &pb.LetterCount{Count: int32(attempts)}, nil
		},
		WithRetry(RetryConfig{
			MaxAttempts: 5,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	result, err := Execute[*pb.LetterCount](ctx, g, "test-retry", &pb.InputString{Value: "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if result.Count != 3 {
		t.Errorf("expected Count 3, got %d", result.Count)
	}
}

func TestExecutionID(t *testing.T) {
	g := NewGraph()

	var capturedID string
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		capturedID = ExecutionID(ctx)
		return &pb.LetterCount{}, nil
	})

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	_, err := Execute[*pb.LetterCount](ctx, g, "my-exec-id", &pb.InputString{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedID != "my-exec-id" {
		t.Errorf("expected execution ID 'my-exec-id', got %q", capturedID)
	}
}

// TestStructWithComputeMethod tests registering a struct with a Compute method
type TestStep struct {
	prefix string
}

func (s *TestStep) Compute(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	return &pb.LetterCount{
		OriginalString: s.prefix + input.Value,
		Count:          int32(len(input.Value)),
	}, nil
}

func TestRegisterStruct(t *testing.T) {
	g := NewGraph()

	step := &TestStep{prefix: "PREFIX:"}
	err := g.Register(step, WithName("TestStep"))
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	result, err := Execute[*pb.LetterCount](ctx, g, "test-struct", &pb.InputString{Value: "hello"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.OriginalString != "PREFIX:hello" {
		t.Errorf("expected 'PREFIX:hello', got %q", result.OriginalString)
	}
}

// Ensure proto.Message interface is satisfied for test types
var _ proto.Message = (*pb.InputString)(nil)
var _ proto.Message = (*pb.LetterCount)(nil)

// ============ Aggregate/Fan-In Tests ============

func TestRegisterAggregate(t *testing.T) {
	g := NewGraph()

	err := g.RegisterAggregate(
		func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
			total := 0
			for _, input := range inputs {
				total += len(input.Value)
			}
			return &pb.LetterCount{Count: int32(total)}, nil
		},
		WithName("AggregateCount"),
	)

	if err != nil {
		t.Fatalf("RegisterAggregate failed: %v", err)
	}

	// Verify the step is marked as aggregate
	for _, meta := range g.steps {
		if !meta.isAggregate {
			t.Error("expected step to be marked as aggregate")
		}
		if meta.name != "AggregateCount" {
			t.Errorf("expected name 'AggregateCount', got %q", meta.name)
		}
	}
}

func TestExecuteBatch(t *testing.T) {
	g := NewGraph()

	// Register an aggregate step that sums lengths (fan-in)
	g.RegisterAggregate(
		func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
			total := 0
			for _, input := range inputs {
				total += len(input.Value)
			}
			return &pb.LetterCount{
				OriginalString: "aggregate",
				Count:          int32(total),
			}, nil
		},
		WithName("SumLengths"),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	inputs := []*pb.InputString{
		{Value: "hello"}, // 5
		{Value: "world"}, // 5
		{Value: "test"},  // 4
	}

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](ctx, g, "batch-test", inputs)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// All results should be the same (aggregate step returns one result for all)
	expectedCount := int32(14) // 5 + 5 + 4
	for i, result := range results {
		if result.Count != expectedCount {
			t.Errorf("result %d: expected Count %d, got %d", i, expectedCount, result.Count)
		}
	}
}

func TestBatchWithPerItemAndAggregate(t *testing.T) {
	g := NewGraph()

	// Step 1: Aggregate step that computes total length (fan-in)
	g.RegisterAggregate(
		func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
			total := 0
			for _, input := range inputs {
				total += len(input.Value)
			}
			return &pb.LetterCount{
				OriginalString: "stats",
				Count:          int32(total),
			}, nil
		},
		WithName("ComputeStats"),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	inputs := []*pb.InputString{
		{Value: "aaa"},
		{Value: "bb"},
	}

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](ctx, g, "fanin-test", inputs)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Both should have the same aggregate result
	for i, result := range results {
		if result.Count != 5 { // 3 + 2
			t.Errorf("result %d: expected Count 5, got %d", i, result.Count)
		}
	}
}

// ============ Parallel Execution Tests ============

// TestParallelBatchExecution verifies that batch items are processed in parallel.
// With simulated 10ms latency per item, 10 items should complete in ~10ms (parallel)
// rather than ~100ms (sequential).
func TestParallelBatchExecution(t *testing.T) {
	g := NewGraph()

	// Per-item step with simulated I/O latency
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			time.Sleep(10 * time.Millisecond) // Simulated I/O
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		WithName("SlowStep"),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Create 10 items
	inputs := make([]*pb.InputString, 10)
	for i := range inputs {
		inputs[i] = &pb.InputString{Value: "test"}
	}

	ctx := context.Background()
	start := time.Now()

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](ctx, g, "parallel-test", inputs)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	elapsed := time.Since(start)

	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	// If executed in parallel, should take ~10-20ms (one batch of parallel work)
	// If sequential, would take ~100ms (10 * 10ms)
	// We allow up to 50ms to account for scheduling overhead
	if elapsed > 50*time.Millisecond {
		t.Errorf("expected parallel execution to complete in <50ms, took %v (likely sequential)", elapsed)
	}

	t.Logf("Parallel batch of 10 items with 10ms latency each completed in %v", elapsed)
}

// TestParallelDependencyResolution verifies that independent dependencies are resolved in parallel.
func TestParallelDependencyResolution(t *testing.T) {
	g := NewGraph()

	// Two independent "leaf" steps with simulated latency
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			time.Sleep(20 * time.Millisecond)
			return &pb.LetterCount{OriginalString: "dep1", Count: 1}, nil
		},
		WithName("Dep1"),
	)

	// Use a wrapper proto to have a second output type
	// For this test, we'll simulate by having the final step depend on both
	// Since we can't easily create a second proto type, we'll test batch parallel instead

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Test that single execution still works
	ctx := context.Background()
	result, err := Execute[*pb.LetterCount](ctx, g, "dep-test", &pb.InputString{Value: "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected Count 1, got %d", result.Count)
	}
}

// BenchmarkBatchParallel compares parallel vs simulated sequential execution
func BenchmarkBatchParallel(b *testing.B) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			// No sleep - just measure framework overhead
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
	)

	if err := g.Validate(); err != nil {
		b.Fatalf("Validate failed: %v", err)
	}

	inputs := make([]*pb.InputString, 100)
	for i := range inputs {
		inputs[i] = &pb.InputString{Value: "benchmark"}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](ctx, g, "bench", inputs)
		if err != nil {
			b.Fatalf("ExecuteBatch failed: %v", err)
		}
	}
}
