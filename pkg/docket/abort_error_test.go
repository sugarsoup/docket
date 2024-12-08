package docket

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "docket/proto/examples/lettercount"
)

func TestAbortError_IsNeverRetriable(t *testing.T) {
	g := NewGraph()

	var attempts int
	abortErr := errors.New("critical failure")

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			return nil, NewAbortError(abortErr)
		},
		WithRetry(RetryConfig{
			MaxAttempts: 5,
			Backoff:     FixedBackoff{Delay: 1},
		}),
	)

	g.Validate()

	_, err := Execute[*pb.LetterCount](
		context.Background(), g, "test-abort",
		&pb.InputString{Value: "test"},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt for AbortError, got %d", attempts)
	}

	var ae *AbortError
	if !errors.As(err, &ae) {
		t.Errorf("expected AbortError in chain, got: %v", err)
	}

	if !errors.Is(ae, abortErr) {
		t.Errorf("expected cause to be wrapped correctly")
	}
}

func TestAbortError_WithMessage(t *testing.T) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			return nil, NewAbortErrorWithMessage(
				"system failure",
				errors.New("db down"),
			)
		},
	)

	g.Validate()

	_, err := Execute[*pb.LetterCount](
		context.Background(), g, "test-abort-msg",
		&pb.InputString{Value: "test"},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	var ae *AbortError
	if !errors.As(err, &ae) {
		t.Fatalf("expected AbortError in chain, got: %v", err)
	}

	if ae.Message != "system failure" {
		t.Errorf("expected message='system failure', got %q", ae.Message)
	}

	if !strings.Contains(err.Error(), "system failure") {
		t.Errorf("error string should contain message: %v", err)
	}

	if !strings.Contains(err.Error(), "db down") {
		t.Errorf("error string should contain cause: %v", err)
	}
}

func TestAbortError_OverridesCustomClassifier(t *testing.T) {
	g := NewGraph()

	var attempts int

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			return nil, NewAbortError(errors.New("critical"))
		},
		WithRetry(RetryConfig{
			MaxAttempts: 5,
			Backoff:     FixedBackoff{Delay: 1},
		}),
		WithErrorClassifier(func(err error) bool {
			return true
		}),
	)

	g.Validate()

	_, err := Execute[*pb.LetterCount](
		context.Background(), g, "test-abort-override",
		&pb.InputString{Value: "test"},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if attempts != 1 {
		t.Errorf("AbortError should override classifier, expected 1 attempt, got %d", attempts)
	}
}

func TestSentinelError_ErrAbortGraph(t *testing.T) {
	if ErrAbortGraph == nil {
		t.Fatal("ErrAbortGraph should be defined")
	}

	err := NewAbortError(ErrAbortGraph)
	if !errors.Is(err, ErrAbortGraph) {
		t.Error("should be able to unwrap to sentinel")
	}
}
