// Example: Timeout Handling
//
// This example demonstrates how docket handles step timeouts,
// preventing slow operations from blocking the entire execution.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

func main() {
	fmt.Println("=== Docket Timeout Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates timeout handling for steps.")
	fmt.Println()

	// ============ EXAMPLE 1: Step Completes Before Timeout ============
	fmt.Println("📌 Example 1: Step completes before timeout")
	runFastStep()

	// ============ EXAMPLE 2: Step Times Out ============
	fmt.Println("📌 Example 2: Step exceeds timeout")
	runSlowStep()

	// ============ EXAMPLE 3: Timeout with Retry ============
	fmt.Println("📌 Example 3: Timeout combined with retry")
	runTimeoutWithRetry()
}

func runFastStep() {
	g := docket.NewGraph()

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			fmt.Println("   Processing... (50ms)")

			// Simulate fast operation
			select {
			case <-time.After(50 * time.Millisecond):
				// Completed normally
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			return &pb.LetterCount{
				OriginalString: input.Value,
				Count:          int32(len(input.Value)),
			}, nil
		},
		docket.WithName("FastStep"),
		docket.WithTimeout(200*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	start := time.Now()

	result, err := docket.Execute[*pb.LetterCount](ctx, g, "timeout-001", &pb.InputString{Value: "quick"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   ✓ Completed in %v (timeout: 200ms)\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("   Result: %q has %d characters\n\n", result.OriginalString, result.Count)
}

func runSlowStep() {
	g := docket.NewGraph()

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			fmt.Println("   Processing... (500ms - will timeout)")

			// Simulate slow operation that exceeds timeout
			select {
			case <-time.After(500 * time.Millisecond):
				// This won't happen - context will cancel first
				return &pb.LetterCount{Count: 1}, nil
			case <-ctx.Done():
				fmt.Println("   ⚠️  Context cancelled:", ctx.Err())
				return nil, ctx.Err()
			}
		},
		docket.WithName("SlowStep"),
		docket.WithTimeout(100*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	start := time.Now()

	_, err = docket.Execute[*pb.LetterCount](ctx, g, "timeout-002", &pb.InputString{Value: "slow"})
	if err != nil {
		fmt.Printf("   ✗ Failed in %v: %v\n\n", time.Since(start).Round(time.Millisecond), err)
	}
}

func runTimeoutWithRetry() {
	g := docket.NewGraph()
	attempts := 0

	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++

			// First attempt: slow (times out)
			// Second attempt: fast (succeeds)
			delay := 200 * time.Millisecond
			if attempts > 1 {
				delay = 20 * time.Millisecond
			}

			fmt.Printf("   Attempt %d: processing for %v...\n", attempts, delay)

			select {
			case <-time.After(delay):
				fmt.Printf("   Attempt %d: completed!\n", attempts)
				return &pb.LetterCount{
					OriginalString: input.Value,
					Count:          int32(len(input.Value)),
				}, nil
			case <-ctx.Done():
				fmt.Printf("   Attempt %d: timed out\n", attempts)
				return nil, ctx.Err()
			}
		},
		docket.WithName("FlakeyStep"),
		docket.WithTimeout(50*time.Millisecond),
		docket.WithRetry(docket.RetryConfig{
			MaxAttempts: 3,
			Backoff:     docket.FixedBackoff{Delay: 10 * time.Millisecond},
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

	result, err := docket.Execute[*pb.LetterCount](ctx, g, "timeout-003", &pb.InputString{Value: "retry-timeout"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   ✓ Success after %d attempts in %v\n", attempts, time.Since(start).Round(time.Millisecond))
	fmt.Printf("   Result: %q has %d characters\n\n", result.OriginalString, result.Count)

	fmt.Println("💡 Key Points:")
	fmt.Println("   • WithTimeout(duration) sets a per-step timeout")
	fmt.Println("   • Steps should check ctx.Done() to respond to cancellation")
	fmt.Println("   • Timeout errors are retriable by default")
	fmt.Println("   • Combine WithTimeout + WithRetry for resilient steps")
}
