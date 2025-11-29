package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func main() {
	fmt.Println("=== Protograph Letter Counting Example ===")
	fmt.Println()

	// ============ SETUP PHASE (not timed) ============

	g := protograph.NewGraph()

	// Register the step using a named function
	// The function signature declares: output type (*pb.LetterCount) and dependencies (*pb.InputString)
	err := g.Register(
		CountLetterR, // Named function from steps.go
		protograph.WithName("CountLetterR"),
	)
	if err != nil {
		log.Fatalf("Failed to register step: %v", err)
	}

	if err := g.Validate(); err != nil {
		log.Fatalf("Graph validation failed: %v", err)
	}

	fmt.Println("✓ Graph setup complete (not timed)")
	fmt.Println()

	// Prepare test input
	input := &pb.InputString{Value: "strawberry"}
	ctx := context.Background()

	// ============ FUNCTIONAL TEST ============

	fmt.Println("📋 Functional Test:")
	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "test-001", input)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}
	fmt.Printf("  Input: %q → Count: %d (letter 'r')\n", input.Value, result.Count)

	// Verify correctness against plain Go implementation
	plainResult := CountLetterRPlain(input)
	if result.Count != plainResult.Count {
		log.Fatalf("Results don't match! Graph: %d, Plain: %d", result.Count, plainResult.Count)
	}
	fmt.Println("  ✓ Graph and plain Go produce identical results")
	fmt.Println()

	// ============ PERFORMANCE TEST ============

	fmt.Println("⏱️  Performance Test:")
	fmt.Println()

	iterations := []int{1000, 10000, 100000}

	for _, n := range iterations {
		fmt.Printf("  %d iterations:\n", n)

		// Benchmark plain Go
		startPlain := time.Now()
		for i := 0; i < n; i++ {
			_ = CountLetterRPlain(input)
		}
		plainDuration := time.Since(startPlain)

		// Benchmark protograph
		startGraph := time.Now()
		for i := 0; i < n; i++ {
			_, _ = protograph.Execute[*pb.LetterCount](ctx, g, fmt.Sprintf("perf-%d", i), input)
		}
		graphDuration := time.Since(startGraph)

		// Calculate overhead
		overhead := float64(graphDuration) / float64(plainDuration)
		plainPerOp := plainDuration / time.Duration(n)
		graphPerOp := graphDuration / time.Duration(n)

		fmt.Printf("    Plain Go:    %v total, %v/op\n", plainDuration, plainPerOp)
		fmt.Printf("    Protograph:  %v total, %v/op\n", graphDuration, graphPerOp)
		fmt.Printf("    Overhead:    %.2fx\n", overhead)
		fmt.Println()
	}

	// ============ ANALYSIS ============

	fmt.Println("📊 Analysis:")
	fmt.Println("  The overhead comes from:")
	fmt.Println("    • Reflection to call the step function")
	fmt.Println("    • Dependency resolution (cache lookup)")
	fmt.Println("    • Context wrapping (execution ID, step name)")
	fmt.Println("    • Result caching")
	fmt.Println()
	fmt.Println("  This overhead is acceptable when:")
	fmt.Println("    • Steps do real work (API calls, DB queries, ML inference)")
	fmt.Println("    • You need retry logic, timeouts, observability")
	fmt.Println("    • You want automatic dependency resolution")
	fmt.Println("    • The happy path avoids persistence (unlike Temporal)")
}
