package main

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
	"google.golang.org/protobuf/proto"
)

// executionCounter tracks how many times the expensive computation actually runs
var executionCounter atomic.Int32

// ExpensiveComputation simulates a costly operation that should only run once
// for the same inputs, even across multiple executions.
func ExpensiveComputation(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	count := executionCounter.Add(1)

	fmt.Printf("  [EXECUTING] ExpensiveComputation for input %s (execution #%d)\n",
		input.Value, count)

	// Simulate expensive work
	time.Sleep(100 * time.Millisecond)

	return &pb.LetterCount{
		Count: int32(len(input.Value)),
	}, nil
}

func main() {
	fmt.Println("=== Exactly-Once Semantics Demo ===\n")
	fmt.Println("This example demonstrates that with ScopeGlobal persistence,")
	fmt.Println("a step executes exactly once for the same inputs, even across")
	fmt.Println("multiple independent executions.\n")

	// Create an in-memory persistence store
	store := docket.NewInMemoryStore()

	// Test input
	input := &pb.InputString{Value: "Alice"}

	fmt.Println("--- First Execution ---")
	fmt.Println("Expected: Step will execute (cache miss)\n")

	// Create the first graph with ScopeGlobal persistence
	graph1 := docket.NewGraph()
	graph1.Register(
		ExpensiveComputation,
		docket.WithName("expensive_computation"),
		docket.WithPersistence(store, docket.ScopeGlobal, 5*time.Minute),
	)
	if err := graph1.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	result1, err := docket.Execute[*pb.LetterCount](ctx, graph1, "exec-001", input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nResult: Count = %d\n", result1.Count)
	fmt.Printf("Execution count: %d\n\n", executionCounter.Load())

	fmt.Println("--- Second Execution (Same Input) ---")
	fmt.Println("Expected: Step will NOT execute (cache hit - exactly once guarantee)\n")

	// Create a NEW graph instance to simulate a fresh execution
	graph2 := docket.NewGraph()
	graph2.Register(
		ExpensiveComputation,
		docket.WithName("expensive_computation"),
		docket.WithPersistence(store, docket.ScopeGlobal, 5*time.Minute),
	)
	if err := graph2.Validate(); err != nil {
		log.Fatal(err)
	}

	result2, err := docket.Execute[*pb.LetterCount](ctx, graph2, "exec-002", input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nResult: Count = %d\n", result2.Count)
	fmt.Printf("Execution count: %d (should still be 1!)\n\n", executionCounter.Load())

	// Verify exactly-once guarantee
	if executionCounter.Load() == 1 {
		fmt.Println("✓ SUCCESS: Exactly-once semantics verified!")
		fmt.Println("  The step executed only once despite two graph executions.")
	} else {
		fmt.Printf("✗ FAILURE: Step executed %d times (expected 1)\n", executionCounter.Load())
	}

	fmt.Println("\n--- Third Execution (Different Input) ---")
	fmt.Println("Expected: Step WILL execute (different inputs = different cache key)\n")

	differentInput := &pb.InputString{Value: "Bob"}

	graph3 := docket.NewGraph()
	graph3.Register(
		ExpensiveComputation,
		docket.WithName("expensive_computation"),
		docket.WithPersistence(store, docket.ScopeGlobal, 5*time.Minute),
	)
	if err := graph3.Validate(); err != nil {
		log.Fatal(err)
	}

	result3, err := docket.Execute[*pb.LetterCount](ctx, graph3, "exec-003", differentInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nResult: Count = %d\n", result3.Count)
	fmt.Printf("Execution count: %d (should be 2 now)\n\n", executionCounter.Load())

	// Verify results are consistent
	if proto.Equal(result1, result2) {
		fmt.Println("✓ Results from first two executions are identical (cached)")
	}

	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total step executions: %d\n", executionCounter.Load())
	fmt.Println("Total graph executions: 3")
	fmt.Println("\nKey Takeaway:")
	fmt.Println("  With ScopeGlobal persistence, Docket guarantees exactly-once")
	fmt.Println("  execution for each unique set of inputs, providing strong")
	fmt.Println("  idempotency guarantees for expensive or side-effecting operations.")
}
