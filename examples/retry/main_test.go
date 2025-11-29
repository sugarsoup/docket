package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func TestRetryLogic(t *testing.T) {
	g := protograph.NewGraph()
	attempts := 0

	// Step fails twice, succeeds on third attempt
	err := g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("transient error")
			}
			return &pb.LetterCount{Count: 1}, nil
		},
		protograph.WithRetry(5, protograph.FixedBackoff{Delay: 1 * time.Millisecond}),
	)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "test-retry", &pb.InputString{Value: "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if result.Count != 1 {
		t.Errorf("expected count 1, got %d", result.Count)
	}
}
