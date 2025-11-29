package protograph

import (
	"context"
	"testing"
	"time"

	pb "protograph/proto/examples/lettercount"
)

func TestPersistence_WorkflowScope(t *testing.T) {
	store := NewInMemoryStore()
	g := NewGraph()

	var callCount int
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			callCount++
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		WithPersistence(store, ScopeWorkflow, 1*time.Hour),
	)

	g.Validate()
	ctx := context.Background()

	// 1. First execution: Cache Miss
	_, err := Execute[*pb.LetterCount](ctx, g, "exec-1", &pb.InputString{Value: "test"})
	if err != nil {
		t.Fatalf("First exec failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// 2. Second execution (Same ID): Cache Hit
	// Even though we provide input, workflow scope keys on (execID, stepName) only.
	// NOTE: In reality, workflow scope implies we are resuming or retrying within same workflow.
	_, err = Execute[*pb.LetterCount](ctx, g, "exec-1", &pb.InputString{Value: "test"})
	if err != nil {
		t.Fatalf("Second exec failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call (cache hit), got %d", callCount)
	}

	// 3. Third execution (Different ID): Cache Miss
	_, err = Execute[*pb.LetterCount](ctx, g, "exec-2", &pb.InputString{Value: "test"})
	if err != nil {
		t.Fatalf("Third exec failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestPersistence_GlobalScope(t *testing.T) {
	store := NewInMemoryStore()
	g := NewGraph()

	var callCount int
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			callCount++
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		WithPersistence(store, ScopeGlobal, 1*time.Hour),
	)

	g.Validate()
	ctx := context.Background()

	// 1. First execution: Cache Miss
	_, err := Execute[*pb.LetterCount](ctx, g, "exec-A", &pb.InputString{Value: "hello"})
	if err != nil {
		t.Fatalf("First exec failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// 2. Second execution (Different ID, Same Input): Cache Hit
	// Global scope keys on (stepName, hash(input))
	_, err = Execute[*pb.LetterCount](ctx, g, "exec-B", &pb.InputString{Value: "hello"})
	if err != nil {
		t.Fatalf("Second exec failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call (cache hit), got %d", callCount)
	}

	// 3. Third execution (Different Input): Cache Miss
	_, err = Execute[*pb.LetterCount](ctx, g, "exec-C", &pb.InputString{Value: "world"})
	if err != nil {
		t.Fatalf("Third exec failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestPersistence_TTL(t *testing.T) {
	store := NewInMemoryStore()
	g := NewGraph()

	var callCount int
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			callCount++
			return &pb.LetterCount{Count: 1}, nil
		},
		WithPersistence(store, ScopeGlobal, 50*time.Millisecond),
	)

	g.Validate()
	ctx := context.Background()

	// 1. First execution
	Execute[*pb.LetterCount](ctx, g, "exec-1", &pb.InputString{Value: "a"})
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// 2. Second execution (Immediate): Hit
	Execute[*pb.LetterCount](ctx, g, "exec-2", &pb.InputString{Value: "a"})
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// 3. Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// 4. Third execution: Miss (Expired)
	Execute[*pb.LetterCount](ctx, g, "exec-3", &pb.InputString{Value: "a"})
	if callCount != 2 {
		t.Errorf("Expected 2 calls (expired), got %d", callCount)
	}
}
