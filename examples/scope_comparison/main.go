package main

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

var (
	workflowScopeExecutions atomic.Int32
	globalScopeExecutions   atomic.Int32
)

// ComputeWithWorkflowScope simulates a step in workflow-scoped graph
func ComputeWithWorkflowScope(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	count := workflowScopeExecutions.Add(1)
	fmt.Printf("    [WORKFLOW SCOPE] Executing for input %s (execution #%d)\n", input.Value, count)
	time.Sleep(50 * time.Millisecond)

	return &pb.LetterCount{
		Count: int32(len(input.Value)),
	}, nil
}

// ComputeWithGlobalScope simulates a step in global-scoped graph
func ComputeWithGlobalScope(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	count := globalScopeExecutions.Add(1)
	fmt.Printf("    [GLOBAL SCOPE] Executing for input %s (execution #%d)\n", input.Value, count)
	time.Sleep(50 * time.Millisecond)

	return &pb.LetterCount{
		Count: int32(len(input.Value)),
	}, nil
}

func runWorkflowScope(store docket.PersistenceStore, input *pb.InputString, executionNum int) error {
	fmt.Printf("\n  Execution #%d (Workflow Scope)\n", executionNum)

	graph := docket.NewGraph()
	graph.Register(
		ComputeWithWorkflowScope,
		docket.WithName("compute_workflow"),
		docket.WithPersistence(store, docket.ScopeWorkflow, 10*time.Minute),
	)
	if err := graph.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	result, err := docket.Execute[*pb.LetterCount](ctx, graph, fmt.Sprintf("workflow-exec-%d", executionNum), input)
	if err != nil {
		return err
	}

	fmt.Printf("    Result: Count = %d\n", result.Count)
	return nil
}

func runGlobalScope(store docket.PersistenceStore, input *pb.InputString, executionNum int) error {
	fmt.Printf("\n  Execution #%d (Global Scope)\n", executionNum)

	graph := docket.NewGraph()
	graph.Register(
		ComputeWithGlobalScope,
		docket.WithName("compute_global"),
		docket.WithPersistence(store, docket.ScopeGlobal, 10*time.Minute),
	)
	if err := graph.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	result, err := docket.Execute[*pb.LetterCount](ctx, graph, fmt.Sprintf("global-exec-%d", executionNum), input)
	if err != nil {
		return err
	}

	fmt.Printf("    Result: Count = %d\n", result.Count)
	return nil
}

func main() {
	fmt.Println("=== Persistence Scope Comparison ===\n")
	fmt.Println("This example demonstrates the difference between ScopeWorkflow")
	fmt.Println("and ScopeGlobal persistence scopes.\n")

	// Test input
	input := &pb.InputString{Value: "Alice"}

	// Create separate stores for each scope to isolate behavior
	workflowStore := docket.NewInMemoryStore()
	globalStore := docket.NewInMemoryStore()

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("PART 1: Multiple Executions with Same Input")
	fmt.Println("═══════════════════════════════════════════════════════════")

	fmt.Println("\n--- ScopeWorkflow: Cache per execution ---")
	fmt.Println("Expected: Each new graph execution triggers step execution")
	fmt.Println("(cache is execution-local, doesn't persist across graphs)")

	for i := 1; i <= 3; i++ {
		if err := runWorkflowScope(workflowStore, input, i); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("\nWorkflow Scope Executions: %d (expected: 3)\n", workflowScopeExecutions.Load())

	fmt.Println("\n--- ScopeGlobal: Cache across executions ---")
	fmt.Println("Expected: Only first execution triggers step execution")
	fmt.Println("(cache persists globally, subsequent executions use cached result)")

	for i := 1; i <= 3; i++ {
		if err := runGlobalScope(globalStore, input, i); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("\nGlobal Scope Executions: %d (expected: 1)\n", globalScopeExecutions.Load())

	// Summary for Part 1
	fmt.Println("\n" + string('─')*60)
	fmt.Println("COMPARISON:")
	fmt.Printf("  ScopeWorkflow: %d executions (no cross-execution caching)\n", workflowScopeExecutions.Load())
	fmt.Printf("  ScopeGlobal:   %d execution  (cached across executions)\n", globalScopeExecutions.Load())
	fmt.Println(string('─') * 60)

	// Part 2: Within-execution deduplication
	fmt.Println("\n\n═══════════════════════════════════════════════════════════")
	fmt.Println("PART 2: Within-Execution Deduplication")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("\nBoth scopes provide deduplication WITHIN a single execution.")
	fmt.Println("Let's verify this by resolving the same type multiple times")
	fmt.Println("in a single graph execution.")

	// Reset counters
	workflowScopeExecutions.Store(0)
	globalScopeExecutions.Store(0)

	fmt.Println("\n--- ScopeWorkflow: Within-execution deduplication ---")

	graph1 := docket.NewGraph()
	graph1.Register(
		ComputeWithWorkflowScope,
		docket.WithName("compute_workflow"),
		docket.WithPersistence(workflowStore, docket.ScopeWorkflow, 10*time.Minute),
	)
	if err := graph1.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Execute the same graph multiple times within single execution context
	// (Note: In practice, you'd resolve the same type multiple times, but
	// for demonstration we execute sequentially which should still show caching)
	fmt.Println("\n  Executing LetterCount 3 times in same execution ID:")
	for i := 1; i <= 3; i++ {
		fmt.Printf("    Execute #%d: ", i)
		_, err := docket.Execute[*pb.LetterCount](ctx, graph1, "workflow-single-exec", input)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Done")
	}

	fmt.Printf("\n  Workflow executions: %d (expected: 1 - deduplicated within execution)\n",
		workflowScopeExecutions.Load())

	fmt.Println("\n--- ScopeGlobal: Within-execution deduplication ---")

	graph2 := docket.NewGraph()
	graph2.Register(
		ComputeWithGlobalScope,
		docket.WithName("compute_global"),
		docket.WithPersistence(globalStore, docket.ScopeGlobal, 10*time.Minute),
	)
	if err := graph2.Validate(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n  Executing LetterCount 3 times in same execution ID:")
	for i := 1; i <= 3; i++ {
		fmt.Printf("    Execute #%d: ", i)
		_, err := docket.Execute[*pb.LetterCount](ctx, graph2, "global-single-exec", input)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Done")
	}

	fmt.Printf("\n  Global executions: %d (expected: 1 - deduplicated within execution)\n",
		globalScopeExecutions.Load())

	// Final Summary
	fmt.Println("\n\n═══════════════════════════════════════════════════════════")
	fmt.Println("SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════")

	fmt.Println("\n┌─────────────────────┬──────────────────┬──────────────────┐")
	fmt.Println("│ Scenario            │ ScopeWorkflow    │ ScopeGlobal      │")
	fmt.Println("├─────────────────────┼──────────────────┼──────────────────┤")
	fmt.Println("│ Within execution    │ ✓ Deduplicated   │ ✓ Deduplicated   │")
	fmt.Println("│ Across executions   │ ✗ Re-executes    │ ✓ Cached         │")
	fmt.Println("│ Cache key includes  │ execution_id     │ step_name only   │")
	fmt.Println("│ Use case            │ Temp results     │ Reusable results │")
	fmt.Println("└─────────────────────┴──────────────────┴──────────────────┘")

	fmt.Println("\n🔑 Key Takeaways:")
	fmt.Println("\n  ScopeWorkflow:")
	fmt.Println("    • Cache key: (execution_id, step_name, input_hash)")
	fmt.Println("    • Lifetime: Single execution only")
	fmt.Println("    • Use when: Results are execution-specific or transient")
	fmt.Println("    • Example: Temporary computation, one-off data processing")

	fmt.Println("\n  ScopeGlobal:")
	fmt.Println("    • Cache key: (step_name, input_hash)")
	fmt.Println("    • Lifetime: Until TTL expires or store cleared")
	fmt.Println("    • Use when: Results are reusable across executions")
	fmt.Println("    • Example: API calls, expensive computations, idempotent operations")
}
