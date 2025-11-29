package protograph

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	pb "protograph/proto/examples/lettercount"
)

// ============ Batch Execution Retry Tests ============

func TestExecuteBatch_PerItemRetry(t *testing.T) {
	g := NewGraph()
	var attempts atomic.Int32

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			// Only fail for a specific input value
			if input.Value == "fail" {
				count := attempts.Add(1)
				if count < 3 {
					return nil, errors.New("temporary error")
				}
			}
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	g.Validate()

	inputs := []*pb.InputString{
		{Value: "ok1"},
		{Value: "fail"}, // This item will fail twice, then succeed
		{Value: "ok2"},
	}

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-retry", inputs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Verify the retry happened
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts for failing item, got %d", attempts.Load())
	}

	// Verify results are correct
	if results[1].Count != 4 { // len("fail")
		t.Errorf("expected count 4 for retried item, got %d", results[1].Count)
	}
}

func TestExecuteBatch_PerItemMaxAttemptsExceeded(t *testing.T) {
	g := NewGraph()
	var attempts atomic.Int32

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			if input.Value == "fail" {
				attempts.Add(1)
				return nil, errors.New("persistent error")
			}
			return &pb.LetterCount{Count: 1}, nil
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	g.Validate()

	inputs := []*pb.InputString{
		{Value: "ok"},
		{Value: "fail"},
	}

	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-fail", inputs)

	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts before failure, got %d", attempts.Load())
	}
}

func TestExecuteBatch_AggregateRetry(t *testing.T) {
	g := NewGraph()
	var attempts atomic.Int32

	g.RegisterAggregate(
		func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
			count := attempts.Add(1)
			if count < 3 {
				return nil, errors.New("temporary aggregate error")
			}
			return &pb.LetterCount{Count: int32(len(inputs))}, nil
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	g.Validate()

	inputs := []*pb.InputString{{Value: "a"}, {Value: "b"}}
	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-agg-retry", inputs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if attempts.Load() != 3 {
		t.Errorf("expected 3 aggregate attempts, got %d", attempts.Load())
	}

	if results[0].Count != 2 {
		t.Errorf("expected count 2, got %d", results[0].Count)
	}
}

func TestExecuteBatch_AggregateMaxAttemptsExceeded(t *testing.T) {
	g := NewGraph()
	var attempts atomic.Int32

	g.RegisterAggregate(
		func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
			attempts.Add(1)
			return nil, errors.New("persistent aggregate error")
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	g.Validate()

	inputs := []*pb.InputString{{Value: "a"}}
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-agg-fail", inputs)

	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts.Load() != 3 {
		t.Errorf("expected 3 aggregate attempts, got %d", attempts.Load())
	}
}

func TestExecuteBatch_TimeoutPerItem(t *testing.T) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			if input.Value == "slow" {
				select {
				case <-time.After(200 * time.Millisecond):
					return &pb.LetterCount{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return &pb.LetterCount{Count: 1}, nil
		},
		WithTimeout(50*time.Millisecond),
		WithRetry(RetryConfig{MaxAttempts: 1}), // No retry
	)

	g.Validate()

	inputs := []*pb.InputString{
		{Value: "fast"},
		{Value: "slow"},
	}

	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "test-timeout", inputs)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}
