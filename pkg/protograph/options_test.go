package protograph

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "protograph/proto/examples/lettercount"
)

func TestWithRetry_FixedBackoff(t *testing.T) {
	g := NewGraph()

	var attempts int
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temp error")
			}
			return &pb.LetterCount{Count: 1}, nil
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	result, err := Execute[*pb.LetterCount](context.Background(), g, "test-retry", &pb.InputString{Value: "a"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected count 1, got %d", result.Count)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_MaxAttemptsExceeded(t *testing.T) {
	g := NewGraph()

	var attempts int
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			return nil, errors.New("persistent error")
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	_, err := Execute[*pb.LetterCount](context.Background(), g, "test-fail", &pb.InputString{Value: "a"})
	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithTimeout(t *testing.T) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return &pb.LetterCount{Count: 1}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		WithTimeout(10*time.Millisecond),
		WithRetry(RetryConfig{
			MaxAttempts: 2,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	_, err := Execute[*pb.LetterCount](context.Background(), g, "test-timeout", &pb.InputString{Value: "a"})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestRetryWithCustomClassifier(t *testing.T) {
	g := NewGraph()
	retryErr := errors.New("retry me")
	fatalErr := errors.New("fatal")

	var attempts int
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			if input.Value == "retry" {
				if attempts < 2 {
					return nil, retryErr
				}
				return &pb.LetterCount{Count: 1}, nil
			}
			return nil, fatalErr
		},
		WithRetry(RetryConfig{
			MaxAttempts: 3,
			Backoff:     FixedBackoff{Delay: 1 * time.Millisecond},
		}),
		WithErrorClassifier(func(err error) bool {
			return errors.Is(err, retryErr)
		}),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Retryable error
	attempts = 0
	_, err := Execute[*pb.LetterCount](context.Background(), g, "test-retry", &pb.InputString{Value: "retry"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}

	// Fatal error
	attempts = 0
	_, err = Execute[*pb.LetterCount](context.Background(), g, "test-fatal", &pb.InputString{Value: "fatal"})
	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestExponentialBackoff(t *testing.T) {
	backoff := ExponentialBackoff{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Factor:       2.0,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 10 * time.Millisecond},
		{2, 20 * time.Millisecond},
		{3, 40 * time.Millisecond},
		{4, 80 * time.Millisecond},
		{5, 100 * time.Millisecond}, // Capped at MaxDelay
	}

	for _, tt := range tests {
		got := backoff.NextDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.want)
		}
	}
}
