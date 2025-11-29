package protograph

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	pb "protograph/proto/examples/lettercount"
)

// ============ Execute Before Validation ============

func TestExecute_BeforeValidation(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	// Don't call Validate()

	_, err := Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{})
	if err == nil {
		t.Error("expected error for Execute before validation")
	}
	if !strings.Contains(err.Error(), "validated") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

// ============ Missing Leaf Input ============

func TestExecute_MissingLeafInput(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{}, nil
	})

	g.Validate()

	// Execute without providing the required InputString
	_, err := Execute[*pb.LetterCount](context.Background(), g, "test")
	if err == nil {
		t.Error("expected error for missing leaf input")
	}
	if !strings.Contains(err.Error(), "no step registered") {
		t.Errorf("expected 'no step registered' error, got: %v", err)
	}
}

// ============ Unreachable Output ============

func TestExecute_UnreachableOutput(t *testing.T) {
	g := NewGraph()

	// Register a step that produces LetterCount
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{}, nil
	})

	g.Validate()

	// Ask for InputString as output, but don't provide it as input.
	// Since no step produces InputString, this is unreachable/missing.
	_, err := Execute[*pb.InputString](context.Background(), g, "test")
	if err == nil {
		t.Fatal("expected error for unreachable output")
	}
	if !strings.Contains(err.Error(), "no step registered") {
		t.Errorf("expected 'no step registered' error, got: %v", err)
	}
}

func TestExecute_ProvidedOutputUsedDirectly(t *testing.T) {
	g := NewGraph()

	var stepCalled bool
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		stepCalled = true
		return &pb.LetterCount{Count: 999}, nil
	})

	g.Validate()

	// When you provide the output type as input, it uses it directly from cache
	// (this is correct behavior - step is skipped)
	result, err := Execute[*pb.LetterCount](context.Background(), g, "test", &pb.LetterCount{Count: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step should NOT be called - we provided the output directly
	if stepCalled {
		t.Error("step should not be called when output is provided as input")
	}

	// Should use the provided value
	if result.Count != 42 {
		t.Errorf("expected Count 42 (provided), got %d", result.Count)
	}
}

// ============ Step Returns Nil Output ============

func TestExecute_StepReturnsNilOutput(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil // Nil output with nil error
	})

	g.Validate()

	_, err := Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{})
	if err == nil {
		t.Error("expected error for nil output")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected nil output error, got: %v", err)
	}
}

// ============ Step Returns Error ============

func TestExecute_StepReturnsError(t *testing.T) {
	g := NewGraph()
	expectedErr := errors.New("step failed")

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, expectedErr
	})

	g.Validate()

	_, err := Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{})
	if err == nil {
		t.Error("expected error from step")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

// ============ Context Cancellation ============

func TestExecute_ContextCancelled(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		select {
		case <-time.After(1 * time.Second):
			return &pb.LetterCount{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	g.Validate()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := Execute[*pb.LetterCount](ctx, g, "test", &pb.InputString{})
	if err == nil {
		t.Error("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestExecute_ContextDeadline(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		select {
		case <-time.After(1 * time.Second):
			return &pb.LetterCount{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	g.Validate()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := Execute[*pb.LetterCount](ctx, g, "test", &pb.InputString{})
	if err == nil {
		t.Error("expected context deadline error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
}

// ============ Unresponsive Step on Cancellation ============

func TestExecute_UnresponsiveStepOnCancel(t *testing.T) {
	g := NewGraph()

	// Step that ignores context and sleeps
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		// Simulate blocked work that doesn't check context
		time.Sleep(200 * time.Millisecond)
		return &pb.LetterCount{}, nil
	})

	g.Validate()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := Execute[*pb.LetterCount](ctx, g, "test-unresponsive", &pb.InputString{})
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error from context cancellation")
	}

	// Should return roughly when context is cancelled (50ms), not when step finishes (200ms)
	// We allow some buffer, but it should be significantly less than 200ms
	if elapsed > 150*time.Millisecond {
		t.Errorf("Execute blocked waiting for unresponsive step: took %v", elapsed)
	}
}

// ============ StepName in Context ============

func TestExecute_StepNameInContext(t *testing.T) {
	g := NewGraph()
	var capturedName string

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			capturedName = StepName(ctx)
			return &pb.LetterCount{}, nil
		},
		WithName("MyStep"),
	)

	g.Validate()
	Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{})

	if capturedName != "MyStep" {
		t.Errorf("expected step name 'MyStep', got %q", capturedName)
	}
}

// ============ ExecutionID in Context ============

func TestExecute_ExecutionIDInContext(t *testing.T) {
	g := NewGraph()
	var capturedID string

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		capturedID = ExecutionID(ctx)
		return &pb.LetterCount{}, nil
	})

	g.Validate()
	Execute[*pb.LetterCount](context.Background(), g, "exec-123", &pb.InputString{})

	if capturedID != "exec-123" {
		t.Errorf("expected execution ID 'exec-123', got %q", capturedID)
	}
}

// ============ Context Helpers Outside Execution ============

func TestExecutionID_OutsideExecution(t *testing.T) {
	id := ExecutionID(context.Background())
	if id != "" {
		t.Errorf("expected empty string outside execution, got %q", id)
	}
}

func TestStepName_OutsideExecution(t *testing.T) {
	name := StepName(context.Background())
	if name != "" {
		t.Errorf("expected empty string outside execution, got %q", name)
	}
}

// ============ Multiple Inputs ============

func TestExecute_MultipleInputs(t *testing.T) {
	g := NewGraph()

	// Step that uses InputString to produce LetterCount
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{OriginalString: input.Value, Count: int32(len(input.Value))}, nil
	})

	g.Validate()

	// Provide multiple inputs (only the matching type should be used)
	result, err := Execute[*pb.LetterCount](
		context.Background(), g, "test",
		&pb.InputString{Value: "hello"},
		&pb.InputString{Value: "world"}, // This will overwrite in cache
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Last input of same type wins (cached)
	if result.OriginalString != "world" {
		t.Errorf("expected 'world' (last input), got %q", result.OriginalString)
	}
}

// ============ Caching Within Single Execution ============

func TestExecute_CachingWithinExecution(t *testing.T) {
	g := NewGraph()
	callCount := 0

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount++
		return &pb.LetterCount{Count: int32(callCount)}, nil
	})

	g.Validate()

	// First execution
	result1, _ := Execute[*pb.LetterCount](context.Background(), g, "test1", &pb.InputString{})
	// Second execution (different execution ID = fresh cache)
	result2, _ := Execute[*pb.LetterCount](context.Background(), g, "test2", &pb.InputString{})

	// Each execution should call the step once
	if callCount != 2 {
		t.Errorf("expected 2 calls (one per execution), got %d", callCount)
	}
	if result1.Count != 1 {
		t.Errorf("first result should be 1, got %d", result1.Count)
	}
	if result2.Count != 2 {
		t.Errorf("second result should be 2, got %d", result2.Count)
	}
}

// ============ No-Dependency Step ============

func TestExecute_NoDependencyStep(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: 42}, nil
	})

	g.Validate()

	result, err := Execute[*pb.LetterCount](context.Background(), g, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 42 {
		t.Errorf("expected 42, got %d", result.Count)
	}
}

// ============ Batch Execution Tests ============

func TestExecuteBatch_BeforeValidation(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	// Don't call Validate()

	inputs := []*pb.InputString{{Value: "test"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test", inputs)
	if err == nil {
		t.Error("expected error for ExecuteBatch before validation")
	}
}

func TestExecuteBatch_EmptyBatch(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	g.Validate()

	inputs := []*pb.InputString{}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test", inputs)
	if err == nil {
		t.Error("expected error for empty batch")
	}
}

func TestExecuteBatch_SingleItem(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})

	g.Validate()

	inputs := []*pb.InputString{{Value: "hello"}}
	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test", inputs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Count != 5 {
		t.Errorf("expected 5, got %d", results[0].Count)
	}
}

func TestExecuteBatch_PerItemError(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		if input.Value == "error" {
			return nil, errors.New("intentional error")
		}
		return &pb.LetterCount{Count: 1}, nil
	})

	g.Validate()

	inputs := []*pb.InputString{{Value: "ok"}, {Value: "error"}, {Value: "ok"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test", inputs)

	if err == nil {
		t.Error("expected error from batch")
	}
}

func TestExecuteBatch_AggregateError(t *testing.T) {
	g := NewGraph()

	g.RegisterAggregate(func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
		return nil, errors.New("aggregate error")
	})

	g.Validate()

	inputs := []*pb.InputString{{Value: "a"}, {Value: "b"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test", inputs)

	if err == nil {
		t.Error("expected error from aggregate step")
	}
}

func TestExecuteBatch_ContextCancellation(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		select {
		case <-time.After(1 * time.Second):
			return &pb.LetterCount{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	g.Validate()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	inputs := []*pb.InputString{{Value: "a"}, {Value: "b"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](ctx, g, "test", inputs)

	if err == nil {
		t.Error("expected context deadline error")
	}
}

func TestExecuteBatch_UnresponsiveStepOnCancel(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		time.Sleep(200 * time.Millisecond)
		return &pb.LetterCount{}, nil
	})

	g.Validate()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	inputs := []*pb.InputString{{Value: "test"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](ctx, g, "test", inputs)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error from context cancellation")
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("ExecuteBatch blocked waiting for unresponsive step: took %v", elapsed)
	}
}

// ============ Panic Recovery ============

func TestExecute_StepPanic(t *testing.T) {
	g := NewGraph()
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		panic("boom")
	})
	g.Validate()

	_, err := Execute[*pb.LetterCount](context.Background(), g, "panic-test", &pb.InputString{})
	if err == nil {
		t.Fatal("expected panic error")
	}

	var panicErr *StepPanicError
	if !errors.As(err, &panicErr) {
		t.Errorf("expected StepPanicError, got %T: %v", err, err)
	}
}

// ============ Concurrency Limit ============

func TestExecuteBatch_ConcurrencyLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow concurrency test")
	}

	g := NewGraph()
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			time.Sleep(50 * time.Millisecond)
			return &pb.LetterCount{Count: 1}, nil
		},
		WithMaxConcurrency(1),
	)
	g.Validate()

	// 5 items * 50ms = 250ms if sequential (limit 1)
	// 50ms if parallel (unlimited)
	inputs := make([]*pb.InputString, 5)
	for i := range inputs {
		inputs[i] = &pb.InputString{}
	}

	start := time.Now()
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "limit-test", inputs)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}

	if elapsed < 200*time.Millisecond {
		t.Errorf("expected sequential execution (>200ms), took %v", elapsed)
	}
}

// ============ Global Timeout ============

func TestGlobalTimeout(t *testing.T) {
	g := NewGraph(WithDefaultTimeout(10 * time.Millisecond))
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return &pb.LetterCount{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	g.Validate()

	// Call without context timeout, should use default
	_, err := Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{})
	if err == nil {
		t.Error("expected global timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}
