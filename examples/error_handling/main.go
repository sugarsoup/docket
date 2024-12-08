// Package main demonstrates error handling patterns in Docket.
//
// This example shows:
// 1. Custom error types with ErrorClassifier (ValidationError)
// 2. Transient errors that should be retried (NetworkError)
// 3. AbortError to stop the entire graph
// 4. Context cancellation propagation
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

// ValidationError represents invalid input that should not be retried.
type ValidationError struct {
	Field string
	Issue string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Issue)
}

// NetworkError represents a transient failure that should be retried.
type NetworkError struct {
	Operation string
	Cause     error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Cause)
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// FetchDataFromAPI simulates an external API call that might fail transiently.
// It validates input first, then fetches data.
// Returns ValidationError for invalid inputs (non-retriable).
// Returns NetworkError for transient failures (retriable).
func FetchDataFromAPI(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	// Validation: These errors should not be retried
	if input.Value == "" {
		return nil, &ValidationError{Field: "value", Issue: "cannot be empty"}
	}
	if len(input.Value) > 100 {
		return nil, &ValidationError{Field: "value", Issue: "exceeds maximum length"}
	}

	// Simulate transient network failures for specific inputs
	if strings.Contains(input.Value, "flaky") {
		return nil, &NetworkError{
			Operation: "FetchData",
			Cause:     errors.New("connection timeout"),
		}
	}

	// Simulate critical system failure
	if strings.Contains(input.Value, "critical") {
		return nil, docket.NewAbortErrorWithMessage(
			"critical system failure detected",
			errors.New("database unavailable"),
		)
	}

	return &pb.LetterCount{
		Count:          int32(len(input.Value)),
		OriginalString: input.Value,
	}, nil
}

// EnrichData adds metadata to the result.
// This step won't be reached if upstream steps fail.
func EnrichData(ctx context.Context, lc *pb.LetterCount) (*pb.LetterCount, error) {
	log.Printf("Enriching data for: %s", lc.OriginalString)
	return lc, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	// Build the graph with error handling
	g := docket.NewGraph()

	// API fetch step: Custom error classification
	// - ValidationError: Don't retry (invalid input)
	// - NetworkError: Retry with backoff (transient failure)
	// - AbortError: Never retry (critical failure)
	g.Register(
		FetchDataFromAPI,
		docket.WithName("FetchData"),
		docket.WithRetry(docket.RetryConfig{
			MaxAttempts: 3,
			Backoff: docket.ExponentialBackoff{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Factor:       2.0,
			},
		}),
		docket.WithTimeout(5*time.Second),
		docket.WithErrorClassifier(func(err error) bool {
			// ValidationErrors are permanent - don't retry
			var validationErr *ValidationError
			if errors.As(err, &validationErr) {
				log.Printf("ValidationError detected, will not retry: %v", err)
				return false
			}

			// NetworkErrors are transient - retry them
			var networkErr *NetworkError
			if errors.As(err, &networkErr) {
				log.Printf("NetworkError detected, will retry: %v", err)
				return true
			}

			// AbortErrors stop the whole graph - handled automatically
			var abortErr *docket.AbortError
			if errors.As(err, &abortErr) {
				log.Printf("AbortError detected, stopping graph: %v", err)
				return false
			}

			return true
		}),
	)

	// Enrichment step
	g.Register(
		EnrichData,
		docket.WithName("EnrichData"),
	)

	if err := g.Validate(); err != nil {
		return fmt.Errorf("graph validation failed: %w", err)
	}

	// Example 1: Valid input - should succeed
	fmt.Println("\n=== Example 1: Valid Input ===")
	result, err := docket.Execute[*pb.LetterCount](
		ctx, g, "exec-1",
		&pb.InputString{Value: "hello world"},
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success: Count=%d\n", result.Count)
	}

	// Example 2: Validation failure - should fail immediately without retry
	fmt.Println("\n=== Example 2: Validation Error (No Retry) ===")
	result, err = docket.Execute[*pb.LetterCount](
		ctx, g, "exec-2",
		&pb.InputString{Value: ""},
	)
	if err != nil {
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			fmt.Printf("ValidationError (expected): %v\n", validationErr)
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}

	// Example 3: Transient network error - would retry but still fail
	// (This will attempt 3 times and then fail)
	fmt.Println("\n=== Example 3: Network Error (Retries Exhausted) ===")
	result, err = docket.Execute[*pb.LetterCount](
		ctx, g, "exec-3",
		&pb.InputString{Value: "flaky"},
	)
	if err != nil {
		var networkErr *NetworkError
		if errors.As(err, &networkErr) {
			fmt.Printf("NetworkError after retries (expected): %v\n", networkErr)
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}

	// Example 4: Critical failure - AbortError stops graph immediately
	fmt.Println("\n=== Example 4: Critical Failure (AbortError) ===")
	result, err = docket.Execute[*pb.LetterCount](
		ctx, g, "exec-4",
		&pb.InputString{Value: "critical"},
	)
	if err != nil {
		var abortErr *docket.AbortError
		if errors.As(err, &abortErr) {
			fmt.Printf("AbortError (expected): %v\n", abortErr)
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}

	// Example 5: Context timeout - cancels in-flight work
	fmt.Println("\n=== Example 5: Context Timeout ===")
	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	result, err = docket.Execute[*pb.LetterCount](
		timeoutCtx, g, "exec-5",
		&pb.InputString{Value: "test"},
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Timeout (expected): %v\n", err)
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}
