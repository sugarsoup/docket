package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

var (
	step1Executions atomic.Int32
	step2Executions atomic.Int32
	step3Executions atomic.Int32
	shouldFailStep2 atomic.Bool
)

// Step1: Data ingestion (always succeeds)
func IngestData(ctx context.Context, input *pb.InputString) (*pb.InputString, error) {
	count := step1Executions.Add(1)
	fmt.Printf("  [STEP 1] IngestData executing (run #%d)\n", count)
	time.Sleep(50 * time.Millisecond)

	return &pb.InputString{Value: "ingested_" + input.Value}, nil
}

// Step2: Data processing (can fail on first attempt)
func ProcessData(ctx context.Context, ingested *pb.InputString) (*pb.LetterCount, error) {
	count := step2Executions.Add(1)
	fmt.Printf("  [STEP 2] ProcessData executing (run #%d)\n", count)

	// Simulate a crash on first attempt
	if shouldFailStep2.Load() {
		fmt.Println("    ⚠️  CRASH: Simulated failure in ProcessData")
		return nil, errors.New("simulated crash during processing")
	}

	time.Sleep(100 * time.Millisecond)

	return &pb.LetterCount{
		Count: int32(len(ingested.Value)),
	}, nil
}

// Step3: Data enrichment (depends on Step2)
func EnrichData(ctx context.Context, processed *pb.LetterCount) (*pb.LetterCount, error) {
	count := step3Executions.Add(1)
	fmt.Printf("  [STEP 3] EnrichData executing (run #%d)\n", count)
	time.Sleep(50 * time.Millisecond)

	return &pb.LetterCount{
		Count: processed.Count * 2, // Enrich by doubling
	}, nil
}

func runExecution(attemptNum int, store docket.Store, failStep2 bool, input *pb.InputString) error {
	fmt.Printf("\n=== Attempt #%d ===\n", attemptNum)
	shouldFailStep2.Store(failStep2)

	if failStep2 {
		fmt.Println("Configuration: Step2 will FAIL (simulating crash)")
	} else {
		fmt.Println("Configuration: All steps should succeed (recovery mode)")
	}
	fmt.Println()

	// Create graph with persistence
	graph := docket.NewGraph()

	// Register steps with persistence
	graph.Register(IngestData,
		docket.WithName("ingest"),
		docket.WithPersistence(store, docket.ScopeGlobal, 10*time.Minute),
	)
	graph.Register(ProcessData,
		docket.WithName("process"),
		docket.WithPersistence(store, docket.ScopeGlobal, 10*time.Minute),
	)
	graph.Register(EnrichData,
		docket.WithName("enrich"),
		docket.WithPersistence(store, docket.ScopeGlobal, 10*time.Minute),
	)

	if err := graph.Validate(); err != nil {
		return err
	}

	// Try to execute the pipeline
	ctx := context.Background()
	result, err := docket.Execute[*pb.LetterCount](ctx, graph, "crash-recovery-demo", input)

	if err != nil {
		fmt.Printf("\n❌ Execution failed: %v\n", err)
		return err
	}

	fmt.Printf("\n✓ Execution succeeded!\n")
	fmt.Printf("Result: Count = %d\n", result.Count)
	return nil
}

func main() {
	fmt.Println("=== Crash Recovery Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates how Docket's persistence enables")
	fmt.Println("crash recovery. When a step fails, previously completed steps")
	fmt.Println("are restored from checkpoints on retry.")
	fmt.Println()

	// Create shared persistence store
	store := docket.NewInMemoryStore()

	// Reset counters
	step1Executions.Store(0)
	step2Executions.Store(0)
	step3Executions.Store(0)

	// Input data
	input := &pb.InputString{Value: "test_data"}

	fmt.Println("Pipeline: IngestData -> ProcessData -> EnrichData")
	fmt.Println("            Step1         Step2         Step3")
	fmt.Println()

	// Attempt 1: Simulate crash in Step2
	fmt.Println("--- First Attempt: Simulating Crash ---")
	err := runExecution(1, store, true, input)
	if err == nil {
		log.Fatal("Expected failure but execution succeeded")
	}

	fmt.Println("\nCheckpoint Status:")
	fmt.Printf("  ✓ Step1 (IngestData): Completed and checkpointed\n")
	fmt.Printf("  ✗ Step2 (ProcessData): Failed - no checkpoint\n")
	fmt.Printf("  - Step3 (EnrichData): Never attempted\n")

	// Attempt 2: Recover from crash
	fmt.Println("\n\n--- Second Attempt: Recovery ---")
	fmt.Println("Expected behavior:")
	fmt.Println("  - Step1 should NOT re-execute (restored from checkpoint)")
	fmt.Println("  - Step2 should retry and succeed")
	fmt.Println("  - Step3 should execute for the first time")
	fmt.Println()

	err = runExecution(2, store, false, input)
	if err != nil {
		log.Fatal(err)
	}

	// Verify crash recovery behavior
	fmt.Println("\n\n=== Execution Summary ===")
	fmt.Printf("Step1 (IngestData) executions:  %d (expected: 1)\n", step1Executions.Load())
	fmt.Printf("Step2 (ProcessData) executions: %d (expected: 2)\n", step2Executions.Load())
	fmt.Printf("Step3 (EnrichData) executions:  %d (expected: 1)\n", step3Executions.Load())

	step1Count := step1Executions.Load()
	step2Count := step2Executions.Load()
	step3Count := step3Executions.Load()

	fmt.Println("\n=== Verification ===")

	if step1Count == 1 {
		fmt.Println("✓ Step1: Executed once (checkpoint restored on retry)")
	} else {
		fmt.Printf("✗ Step1: Executed %d times (expected 1)\n", step1Count)
	}

	if step2Count == 2 {
		fmt.Println("✓ Step2: Executed twice (failed first, succeeded on retry)")
	} else {
		fmt.Printf("✗ Step2: Executed %d times (expected 2)\n", step2Count)
	}

	if step3Count == 1 {
		fmt.Println("✓ Step3: Executed once (only after Step2 succeeded)")
	} else {
		fmt.Printf("✗ Step3: Executed %d times (expected 1)\n", step3Count)
	}

	if step1Count == 1 && step2Count == 2 && step3Count == 1 {
		fmt.Println("\n🎉 SUCCESS: Crash recovery working correctly!")
		fmt.Println("\nKey Insight:")
		fmt.Println("  Persistence enabled efficient recovery by avoiding")
		fmt.Println("  re-execution of expensive Step1. Only the failed")
		fmt.Println("  step (Step2) needed to retry.")
	} else {
		fmt.Println("\n⚠️  Unexpected execution counts detected")
	}
}
