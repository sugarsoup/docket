package protograph

import (
	"context"
	"strings"
	"testing"

	pb "protograph/proto/examples/lettercount"
)

// ============ Empty Graph Validation ============

func TestValidate_EmptyGraph(t *testing.T) {
	g := NewGraph()

	err := g.Validate()

	if err == nil {
		t.Error("expected error for empty graph, got nil")
	}
	if !strings.Contains(err.Error(), "no registered steps") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ============ Cycle Detection Tests ============

// For cycle detection, we need multiple proto types. Since we only have InputString and LetterCount,
// we'll test the simpler cases and verify the cycle detection logic works.

func TestValidate_SingleStep_NoCycle(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	err := g.Validate()
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValidate_StepDependsOnOwnOutput(t *testing.T) {
	g := NewGraph()

	// A step that says it produces LetterCount but also needs LetterCount as input
	// This would be a self-cycle
	g.Register(func(ctx context.Context, input *pb.LetterCount) (*pb.LetterCount, error) {
		return input, nil
	})

	err := g.Validate()
	if err == nil {
		t.Error("expected cycle detection error for self-referential step")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

// ============ Multiple Validation Calls ============

func TestValidate_MultipleCallsIdempotent(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	// First validation
	err := g.Validate()
	if err != nil {
		t.Fatalf("first Validate failed: %v", err)
	}

	// Second validation should succeed (idempotent)
	err = g.Validate()
	if err != nil {
		t.Errorf("second Validate should succeed: %v", err)
	}
}

// ============ Validated Flag Tests ============

func TestValidate_SetsValidatedFlag(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	if g.validated {
		t.Error("graph should not be validated before Validate()")
	}

	g.Validate()

	if !g.validated {
		t.Error("graph should be validated after Validate()")
	}
}

// ============ GetExecutionPlan Tests ============

func TestGetExecutionPlan_BeforeValidation(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	// Should fail before validation
	_, err := g.GetExecutionPlan(&pb.LetterCount{})
	if err == nil {
		t.Error("expected error for GetExecutionPlan before validation")
	}
}

func TestGetExecutionPlan_SingleStep(t *testing.T) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			return nil, nil
		},
		WithName("CountLetters"),
	)

	g.Validate()

	plan, err := g.GetExecutionPlan(&pb.LetterCount{})
	if err != nil {
		t.Fatalf("GetExecutionPlan failed: %v", err)
	}

	if len(plan) != 1 {
		t.Errorf("expected 1 step in plan, got %d", len(plan))
	}
	if plan[0] != "CountLetters" {
		t.Errorf("expected 'CountLetters', got %q", plan[0])
	}
}

func TestGetExecutionPlan_LeafType(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	g.Validate()

	// Get plan for a type that's not produced by any step (leaf input)
	plan, err := g.GetExecutionPlan(&pb.InputString{})
	if err != nil {
		t.Fatalf("GetExecutionPlan failed: %v", err)
	}

	// Should return empty plan (no steps needed, it's an input)
	if len(plan) != 0 {
		t.Errorf("expected empty plan for leaf type, got %d steps", len(plan))
	}
}

// ============ Steps() Introspection Tests ============

func TestSteps_ReturnsAllSteps(t *testing.T) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			return nil, nil
		},
		WithName("Step1"),
	)

	steps := g.Steps()
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
}

func TestSteps_IncludesDependencies(t *testing.T) {
	g := NewGraph()

	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			return nil, nil
		},
		WithName("Step1"),
	)

	steps := g.Steps()
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	if len(steps[0].Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(steps[0].Dependencies))
	}
}

func TestSteps_ReturnsDefensiveCopy(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	steps1 := g.Steps()
	steps2 := g.Steps()

	// Modifying one shouldn't affect the other
	if len(steps1) > 0 && len(steps2) > 0 {
		steps1[0].Name = "MODIFIED"
		if steps2[0].Name == "MODIFIED" {
			t.Error("Steps() should return defensive copies")
		}
	}
}

// ============ LeafTypes() Introspection Tests ============

func TestLeafTypes_IdentifiesInputTypes(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	leaves := g.LeafTypes()
	if len(leaves) != 1 {
		t.Errorf("expected 1 leaf type, got %d", len(leaves))
	}
}

func TestLeafTypes_EmptyForNoDependencies(t *testing.T) {
	g := NewGraph()

	// Step with no dependencies
	g.Register(func(ctx context.Context) (*pb.LetterCount, error) {
		return &pb.LetterCount{}, nil
	})

	leaves := g.LeafTypes()
	if len(leaves) != 0 {
		t.Errorf("expected 0 leaf types, got %d", len(leaves))
	}
}
