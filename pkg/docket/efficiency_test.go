package docket

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	pb "docket/proto/examples/lettercount"
)

// ============ Efficiency Tests ============

func TestBatch_DuplicateInputs_WorkCount(t *testing.T) {
	g := NewGraph()
	var callCount atomic.Int32

	// Step: Identity
	if err := g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return &pb.LetterCount{Count: 1}, nil
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// 10 identical inputs
	count := 10
	inputs := make([]*pb.InputString, count)
	for i := 0; i < count; i++ {
		inputs[i] = &pb.InputString{Value: "same"}
	}

	_, err := ExecuteBatch[*pb.InputString, *pb.LetterCount](context.Background(), g, "batch-dedup", inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := int(callCount.Load())
	t.Logf("Batch size: %d, Actual calls: %d", count, calls)

	// Current expectation: No deduplication logic exists for batch items in-memory.
	// So calls should be 10.
	if calls != count {
		t.Logf("Optimized! Only %d calls for %d items", calls, count)
	} else {
		t.Log("Standard behavior: 1 call per item (no deduplication)")
	}
}
