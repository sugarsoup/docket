package main

import (
	"context"
	"reflect"
	"testing"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/parallel"
)

func TestIntrospectionLogic(t *testing.T) {
	g := protograph.NewGraph()

	g.Register(
		func(ctx context.Context, id *pb.UserID) (*pb.UserProfile, error) { return nil, nil },
		protograph.WithName("ProfileStep"),
	)

	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Verify introspection APIs
	steps := g.Steps()
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Name != "ProfileStep" {
		t.Errorf("expected name 'ProfileStep', got %q", steps[0].Name)
	}

	leaves := g.LeafTypes()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf type, got %d", len(leaves))
	}
	expectedType := reflect.TypeOf(&pb.UserID{})
	if leaves[0] != expectedType {
		t.Errorf("expected leaf type %v, got %v", expectedType, leaves[0])
	}
}
