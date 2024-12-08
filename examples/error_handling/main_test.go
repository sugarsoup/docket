package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

func buildTestGraph(t *testing.T) *docket.Graph {
	g := docket.NewGraph()

	g.Register(
		FetchDataFromAPI,
		docket.WithName("FetchData"),
		docket.WithRetry(docket.RetryConfig{
			MaxAttempts: 3,
			Backoff: docket.ExponentialBackoff{
				InitialDelay: 1 * time.Millisecond,
				MaxDelay:     10 * time.Millisecond,
				Factor:       2.0,
			},
		}),
		docket.WithTimeout(5*time.Second),
		docket.WithErrorClassifier(func(err error) bool {
			var validationErr *ValidationError
			if errors.As(err, &validationErr) {
				return false
			}
			var networkErr *NetworkError
			if errors.As(err, &networkErr) {
				return true
			}
			var abortErr *docket.AbortError
			if errors.As(err, &abortErr) {
				return false
			}
			return true
		}),
	)

	g.Register(
		EnrichData,
		docket.WithName("EnrichData"),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("graph validation failed: %v", err)
	}
	return g
}

func TestValidInput_Succeeds(t *testing.T) {
	g := buildTestGraph(t)
	ctx := context.Background()

	result, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-valid",
		&pb.InputString{Value: "hello"},
	)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if result.Count != 5 {
		t.Errorf("expected count=5, got %d", result.Count)
	}
}

func TestEmptyInput_ValidationError_NoRetry(t *testing.T) {
	g := buildTestGraph(t)
	ctx := context.Background()

	_, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-empty",
		&pb.InputString{Value: ""},
	)

	if err == nil {
		t.Fatal("expected error for empty input")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("expected ValidationError, got: %v", err)
	}

	if validationErr.Field != "value" {
		t.Errorf("expected field=value, got %s", validationErr.Field)
	}
}

func TestLongInput_ValidationError_NoRetry(t *testing.T) {
	g := buildTestGraph(t)
	ctx := context.Background()

	longString := string(make([]byte, 101))
	_, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-long",
		&pb.InputString{Value: longString},
	)

	if err == nil {
		t.Fatal("expected error for long input")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

func TestFlakyInput_NetworkError_RetriesExhausted(t *testing.T) {
	g := buildTestGraph(t)
	ctx := context.Background()

	_, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-flaky",
		&pb.InputString{Value: "flaky"},
	)

	if err == nil {
		t.Fatal("expected error for flaky input")
	}

	var networkErr *NetworkError
	if !errors.As(err, &networkErr) {
		t.Errorf("expected NetworkError in chain, got: %v", err)
	}
}

func TestCriticalFailure_AbortError_StopsGraph(t *testing.T) {
	g := buildTestGraph(t)
	ctx := context.Background()

	_, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-critical",
		&pb.InputString{Value: "critical"},
	)

	if err == nil {
		t.Fatal("expected error for critical input")
	}

	var abortErr *docket.AbortError
	if !errors.As(err, &abortErr) {
		t.Errorf("expected AbortError in chain, got: %v", err)
	}

	if abortErr.Message != "critical system failure detected" {
		t.Errorf("expected specific abort message, got: %s", abortErr.Message)
	}
}

func TestContextTimeout_CancelsExecution(t *testing.T) {
	g := buildTestGraph(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	_, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-timeout",
		&pb.InputString{Value: "test"},
	)

	if err == nil {
		t.Fatal("expected error for timeout")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected DeadlineExceeded or Canceled, got: %v", err)
	}
}

func TestContextCancellation_StopsExecution(t *testing.T) {
	g := buildTestGraph(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := docket.Execute[*pb.LetterCount](
		ctx, g, "test-cancel",
		&pb.InputString{Value: "test"},
	)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got: %v", err)
	}
}
