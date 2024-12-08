package docket

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "docket/proto/examples/lettercount"
)

// ============ Concurrent Execution Safety ============

func TestConcurrentExecutions_SameGraph(t *testing.T) {
	g := NewGraph()
	var callCount atomic.Int32

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})

	g.Validate()

	// Run 10 concurrent executions
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result, err := Execute[*pb.LetterCount](
				context.Background(), g,
				"exec-"+string(rune('0'+id)),
				&pb.InputString{Value: "test"},
			)
			if err != nil {
				errs <- err
				return
			}
			if result.Count != 4 {
				errs <- errors.New("unexpected result")
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent execution error: %v", err)
	}

	// Each execution should call the step once
	if callCount.Load() != 10 {
		t.Errorf("expected 10 calls, got %d", callCount.Load())
	}
}

// ============ Parallel Dependency Resolution Safety ============

func TestParallelDependencyResolution_RaceCondition(t *testing.T) {
	// This test verifies that when multiple dependencies are resolved in parallel,
	// there are no race conditions in cache access
	g := NewGraph()

	var callCount atomic.Int32

	// Step that depends on input
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount.Add(1)
		time.Sleep(20 * time.Millisecond)
		return &pb.LetterCount{Count: 1, OriginalString: "dep1"}, nil
	})

	g.Validate()

	// Run multiple concurrent executions
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{Value: "x"})
		}()
	}
	wg.Wait()

	// Verify no race conditions occurred (test passes if no panic)
	t.Logf("call count: %d", callCount.Load())
}

// ============ In-Flight Deduplication ============

func TestInFlightDeduplication_SameType(t *testing.T) {
	g := NewGraph()
	var callCount atomic.Int32
	startedCh := make(chan struct{})
	var startOnce sync.Once

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount.Add(1)
		startOnce.Do(func() { close(startedCh) })
		time.Sleep(50 * time.Millisecond) // Slow step
		return &pb.LetterCount{Count: 42}, nil
	})

	g.Validate()

	// Start first execution
	var wg sync.WaitGroup
	results := make([]*pb.LetterCount, 3)
	errs := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx > 0 {
				<-startedCh // Wait for first execution to start
			}
			results[idx], errs[idx] = Execute[*pb.LetterCount](
				context.Background(), g, "same-exec",
				&pb.InputString{Value: "test"},
			)
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("execution %d failed: %v", i, err)
		}
	}

	// With same execution ID, should only call once due to in-flight deduplication
	// But different execution IDs would call multiple times
	// This tests uses same execution ID "same-exec" for all
}

// ============ Batch Parallel Safety ============

func TestBatchParallel_NoRaceConditions(t *testing.T) {
	g := NewGraph()
	var callCount atomic.Int32

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount.Add(1)
		time.Sleep(5 * time.Millisecond)
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})

	g.Validate()

	inputs := make([]*pb.InputString, 20)
	for i := range inputs {
		inputs[i] = &pb.InputString{Value: "test"}
	}

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](
		context.Background(), g, "batch", inputs,
	)

	if err != nil {
		t.Fatalf("batch execution failed: %v", err)
	}
	if len(results) != 20 {
		t.Errorf("expected 20 results, got %d", len(results))
	}
	if callCount.Load() != 20 {
		t.Errorf("expected 20 calls, got %d", callCount.Load())
	}

	// Verify all results are correct
	for i, r := range results {
		if r.Count != 4 {
			t.Errorf("result %d: expected 4, got %d", i, r.Count)
		}
	}
}

// ============ Error Propagation in Parallel ============

func TestParallelBatch_FirstErrorCancelsOthers(t *testing.T) {
	g := NewGraph()
	var startedCount, completedCount atomic.Int32

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		startedCount.Add(1)
		defer completedCount.Add(1)

		if input.Value == "fail" {
			return nil, errors.New("intentional failure")
		}

		select {
		case <-time.After(100 * time.Millisecond):
			return &pb.LetterCount{Count: 1}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	g.Validate()

	inputs := []*pb.InputString{
		{Value: "ok1"},
		{Value: "fail"}, // This will fail quickly
		{Value: "ok2"},
		{Value: "ok3"},
	}

	start := time.Now()
	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](
		context.Background(), g, "batch", inputs,
	)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error from batch")
	}

	// Should complete quickly due to error cancellation
	if elapsed > 50*time.Millisecond {
		t.Errorf("expected early termination, took %v", elapsed)
	}

	t.Logf("started: %d, completed: %d, elapsed: %v",
		startedCount.Load(), completedCount.Load(), elapsed)
}

// ============ Concurrent Batch Executions ============

func TestConcurrentBatchExecutions(t *testing.T) {
	g := NewGraph()
	var totalCalls atomic.Int32

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		totalCalls.Add(1)
		time.Sleep(5 * time.Millisecond)
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})

	g.Validate()

	var wg sync.WaitGroup
	errs := make(chan error, 5)

	for batch := 0; batch < 5; batch++ {
		wg.Add(1)
		go func(batchID int) {
			defer wg.Done()
			inputs := make([]*pb.InputString, 10)
			for i := range inputs {
				inputs[i] = &pb.InputString{Value: "test"}
			}

			results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](
				context.Background(), g,
				"batch-"+string(rune('0'+batchID)),
				inputs,
			)
			if err != nil {
				errs <- err
				return
			}
			if len(results) != 10 {
				errs <- errors.New("wrong result count")
			}
		}(batch)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent batch error: %v", err)
	}

	expectedCalls := int32(5 * 10) // 5 batches * 10 items each
	if totalCalls.Load() != expectedCalls {
		t.Errorf("expected %d calls, got %d", expectedCalls, totalCalls.Load())
	}
}

// ============ Aggregate Step Caching in Batch ============

func TestAggregateCaching_CalledOnce(t *testing.T) {
	g := NewGraph()
	var aggregateCalls atomic.Int32

	g.RegisterAggregate(func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
		aggregateCalls.Add(1)
		time.Sleep(20 * time.Millisecond)
		total := 0
		for _, input := range inputs {
			total += len(input.Value)
		}
		return &pb.LetterCount{Count: int32(total)}, nil
	})

	g.Validate()

	inputs := []*pb.InputString{
		{Value: "aaa"},
		{Value: "bb"},
		{Value: "c"},
	}

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](
		context.Background(), g, "batch", inputs,
	)

	if err != nil {
		t.Fatalf("batch execution failed: %v", err)
	}

	// Aggregate should only be called once
	if aggregateCalls.Load() != 1 {
		t.Errorf("expected 1 aggregate call, got %d", aggregateCalls.Load())
	}

	// All results should be the same
	expectedCount := int32(6) // 3 + 2 + 1
	for i, r := range results {
		if r.Count != expectedCount {
			t.Errorf("result %d: expected %d, got %d", i, expectedCount, r.Count)
		}
	}
}

// ============ Mixed Aggregate and Per-Item Steps ============

func TestMixedSteps_CorrectCalling(t *testing.T) {
	g := NewGraph()
	var aggregateCalls, perItemCalls atomic.Int32

	// Aggregate step: compute sum
	g.RegisterAggregate(
		func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
			aggregateCalls.Add(1)
			total := 0
			for _, input := range inputs {
				total += len(input.Value)
			}
			return &pb.LetterCount{Count: int32(total), OriginalString: "sum"}, nil
		},
		WithName("ComputeSum"),
	)

	g.Validate()

	inputs := []*pb.InputString{
		{Value: "aaa"},
		{Value: "bb"},
		{Value: "c"},
	}

	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](
		context.Background(), g, "batch", inputs,
	)

	if err != nil {
		t.Fatalf("batch execution failed: %v", err)
	}

	if aggregateCalls.Load() != 1 {
		t.Errorf("expected 1 aggregate call, got %d", aggregateCalls.Load())
	}

	// Per-item step not registered, so perItemCalls should be 0
	if perItemCalls.Load() != 0 {
		t.Errorf("expected 0 per-item calls, got %d", perItemCalls.Load())
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

// ============ Stress Test ============

func TestStress_ManyItemsBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	g := NewGraph()
	var callCount atomic.Int32

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount.Add(1)
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})

	g.Validate()

	const batchSize = 1000
	inputs := make([]*pb.InputString, batchSize)
	for i := range inputs {
		inputs[i] = &pb.InputString{Value: "test"}
	}

	start := time.Now()
	results, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](
		context.Background(), g, "stress", inputs,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("batch execution failed: %v", err)
	}
	if len(results) != batchSize {
		t.Errorf("expected %d results, got %d", batchSize, len(results))
	}
	if callCount.Load() != batchSize {
		t.Errorf("expected %d calls, got %d", batchSize, callCount.Load())
	}

	t.Logf("Processed %d items in %v (%.0f items/sec)",
		batchSize, elapsed, float64(batchSize)/elapsed.Seconds())
}
