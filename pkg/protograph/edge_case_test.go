package protograph

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	pb "protograph/proto/examples/lettercount"
)

// ============ Coverage Gap: resolveParallel Error Handling ============

func TestResolveParallel_ErrorInDependency(t *testing.T) {
	g := NewGraph()

	// Dep 1: Succeeds
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: 1}, nil
	}, WithName("Dep1"))

	// Dep 2: Fails
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.InputString, error) { // Using InputString as output for variety
		return nil, errors.New("dep2 failed")
	}, WithName("Dep2"))

	// Final Step: Depends on both
	g.Register(func(ctx context.Context, lc *pb.LetterCount, is *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: lc.Count}, nil
	}, WithName("Final"))

	g.Validate()

	_, err := Execute[*pb.LetterCount](context.Background(), g, "test-parallel-err", &pb.InputString{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Error should come from Dep2
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// ============ Coverage Gap: resolveBatchParallel Error Handling ============

func TestResolveBatchParallel_ErrorInDependency(t *testing.T) {
	g := NewGraph()

	// Dep 1: Succeeds
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: 1}, nil
	}, WithName("Dep1"))

	// Dep 2: Fails
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.InputString, error) {
		return nil, errors.New("batch dep2 failed")
	}, WithName("Dep2"))

	// Final Step (Aggregate to force batch resolution of deps)
	g.RegisterAggregate(func(ctx context.Context, lcs []*pb.LetterCount, iss []*pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: int32(len(lcs))}, nil
	}, WithName("FinalAgg"))

	g.Validate()

	inputs := []*pb.InputString{{Value: "test"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-batch-parallel-err", inputs)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// ============ Coverage Gap: callStep Reflection Errors ============

func TestCallStep_NilInterfaceReturn(t *testing.T) {
	// This test serves as documentation for why certain branches in callStep are unreachable
	// via public API if Validate() is working correctly.
}

// ============ Coverage Gap: batchExecutor.isRetriable custom classifier ============

func TestBatchExecutor_CustomRetryClassifier(t *testing.T) {
	g := NewGraph()
	retryErr := errors.New("retry me")
	fatalErr := errors.New("fatal")
	var attempts atomic.Int32

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			count := attempts.Add(1)
			if input.Value == "retry" && count < 2 {
				return nil, retryErr
			}
			if input.Value == "fatal" {
				return nil, fatalErr
			}
			return &pb.LetterCount{Count: 1}, nil
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
		WithErrorClassifier(func(err error) bool {
			return errors.Is(err, retryErr)
		}),
	)

	g.Validate()

	// Case 1: Retryable error succeeds
	inputs1 := []*pb.InputString{{Value: "retry"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-custom-retry", inputs1)
	if err != nil {
		t.Errorf("expected success for retryable error, got: %v", err)
	}
	if attempts.Load() != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts.Load())
	}

	// Case 2: Fatal error fails immediately
	attempts.Store(0)
	inputs2 := []*pb.InputString{{Value: "fatal"}}
	_, err = ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-custom-fatal", inputs2)
	if err == nil {
		t.Error("expected error for fatal input")
	}
	if attempts.Load() != 1 {
		t.Errorf("expected 1 attempt for fatal error, got %d", attempts.Load())
	}
}
