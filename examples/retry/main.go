// Example: Retry with Backoff
//
// This example demonstrates how protograph handles transient failures
// with configurable retry logic and backoff strategies.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

// Simulated transient errors
var (
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
	ErrRateLimit          = errors.New("rate limit exceeded")
)

func main() {
	fmt.Println("=== Protograph Retry Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates retry with backoff strategies.")
	fmt.Println()

	// ============ EXAMPLE 1: Fixed Backoff ============
	fmt.Println("📌 Example 1: Fixed Backoff (100ms between retries)")
	runWithFixedBackoff()

	// ============ EXAMPLE 2: Exponential Backoff ============
	fmt.Println("📌 Example 2: Exponential Backoff (100ms → 200ms → 400ms)")
	runWithExponentialBackoff()

	// ============ EXAMPLE 3: Selective Retry ============
	fmt.Println("📌 Example 3: Selective Retry (only retry specific errors)")
	runWithSelectiveRetry()
}

func runWithFixedBackoff() {
	g := protograph.NewGraph()
	attempts := 0

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			fmt.Printf("   Attempt %d...\n", attempts)

			// Fail first 2 attempts
			if attempts < 3 {
				return nil, ErrServiceUnavailable
			}

			return &pb.LetterCount{
				OriginalString: input.Value,
				Count:          int32(len(input.Value)),
			}, nil
		},
		protograph.WithName("UnreliableStep"),
		protograph.WithRetry(5, protograph.FixedBackoff{Delay: 100 * time.Millisecond}),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	start := time.Now()

	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "fixed-backoff-001", &pb.InputString{Value: "hello"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   ✓ Success after %d attempts in %v\n", attempts, time.Since(start))
	fmt.Printf("   Result: %q has %d characters\n\n", result.OriginalString, result.Count)

	// Reset for next example
	attempts = 0
}

func runWithExponentialBackoff() {
	g := protograph.NewGraph()
	attempts := 0
	lastAttemptTime := time.Now()

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			elapsed := time.Since(lastAttemptTime)
			lastAttemptTime = time.Now()

			if attempts == 1 {
				fmt.Printf("   Attempt %d\n", attempts)
			} else {
				fmt.Printf("   Attempt %d (after %v delay)\n", attempts, elapsed.Round(time.Millisecond))
			}

			// Fail first 3 attempts
			if attempts < 4 {
				return nil, ErrRateLimit
			}

			return &pb.LetterCount{
				OriginalString: input.Value,
				Count:          int32(len(input.Value)),
			}, nil
		},
		protograph.WithName("RateLimitedStep"),
		protograph.WithRetry(5, protograph.ExponentialBackoff{
			Initial: 100 * time.Millisecond,
			Factor:  2.0,
			Max:     2 * time.Second,
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	start := time.Now()

	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "exp-backoff-001", &pb.InputString{Value: "world"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   ✓ Success after %d attempts in %v\n", attempts, time.Since(start))
	fmt.Printf("   Result: %q has %d characters\n\n", result.OriginalString, result.Count)
}

func runWithSelectiveRetry() {
	g := protograph.NewGraph()
	attempts := 0

	// Only retry on specific error types
	isRetriable := func(err error) bool {
		// Only retry rate limit errors, not service unavailable
		return errors.Is(err, ErrRateLimit)
	}

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			fmt.Printf("   Attempt %d...\n", attempts)

			// Randomly fail with different errors
			if attempts < 3 {
				if rand.Float32() < 0.5 {
					fmt.Println("     → Rate limit error (will retry)")
					return nil, ErrRateLimit
				}
				fmt.Println("     → Rate limit error (will retry)")
				return nil, ErrRateLimit
			}

			return &pb.LetterCount{
				OriginalString: input.Value,
				Count:          int32(len(input.Value)),
			}, nil
		},
		protograph.WithName("SelectiveRetryStep"),
		protograph.WithRetry(5, protograph.FixedBackoff{Delay: 50 * time.Millisecond}),
		protograph.WithRetryOn(isRetriable),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "selective-retry-001", &pb.InputString{Value: "retry"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   ✓ Success after %d attempts\n", attempts)
	fmt.Printf("   Result: %q has %d characters\n\n", result.OriginalString, result.Count)

	fmt.Println("💡 Key Points:")
	fmt.Println("   • WithRetry(maxAttempts, backoffStrategy) configures retry behavior")
	fmt.Println("   • FixedBackoff: constant delay between retries")
	fmt.Println("   • ExponentialBackoff: increasing delays (100ms → 200ms → 400ms...)")
	fmt.Println("   • WithRetryOn(classifier) selectively retries based on error type")
}
