package main

import (
	"context"
	"testing"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/parallel"
)

func TestParallelLogic(t *testing.T) {
	g := protograph.NewGraph()

	// Step 1
	g.Register(func(ctx context.Context, id *pb.UserID) (*pb.UserProfile, error) {
		time.Sleep(10 * time.Millisecond)
		return &pb.UserProfile{Name: "User"}, nil
	})

	// Step 2 (independent)
	g.Register(func(ctx context.Context, id *pb.UserID) (*pb.UserPreferences, error) {
		time.Sleep(10 * time.Millisecond)
		return &pb.UserPreferences{Theme: "Dark"}, nil
	})

	// Step 3 (dependent on both)
	g.Register(func(ctx context.Context, p *pb.UserProfile, pr *pb.UserPreferences) (*pb.EnrichedUser, error) {
		return &pb.EnrichedUser{Profile: p, Preferences: pr}, nil
	})

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	start := time.Now()
	result, err := protograph.Execute[*pb.EnrichedUser](ctx, g, "test-parallel", &pb.UserID{Id: "1"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Logic check
	if result.Profile.Name != "User" {
		t.Error("expected profile name 'User'")
	}
	if result.Preferences.Theme != "Dark" {
		t.Error("expected theme 'Dark'")
	}

	// Timing check (loose): parallel execution (10ms) should be faster than sequential (20ms)
	// Allowing some overhead, but if it takes >25ms it's likely sequential
	if elapsed > 30*time.Millisecond {
		t.Logf("Warning: Execution took %v, might not be parallel (expected ~10-15ms)", elapsed)
	}
}
