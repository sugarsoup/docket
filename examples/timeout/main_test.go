package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func TestTimeoutLogic(t *testing.T) {
	g := protograph.NewGraph()

	// Step that takes longer than timeout
	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return &pb.LetterCount{Count: 1}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		protograph.WithTimeout(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	_, err = protograph.Execute[*pb.LetterCount](ctx, g, "test-timeout", &pb.InputString{Value: "test"})

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}
