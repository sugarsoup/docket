package protograph

import (
	"context"
	"testing"

	pb "protograph/proto/examples/lettercount"
)

// ============ Invalid Function Signature Tests ============

func TestRegister_NoContext(t *testing.T) {
	g := NewGraph()

	// Function without context parameter
	err := g.Register(func(input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for function without context.Context, got nil")
	}
}

func TestRegister_WrongContextType(t *testing.T) {
	g := NewGraph()

	// First parameter is not context.Context
	err := g.Register(func(notCtx string, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for wrong first parameter type, got nil")
	}
}

func TestRegister_NoReturnValues(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input *pb.InputString) {
		// No return values
	})

	if err == nil {
		t.Error("expected error for no return values, got nil")
	}
}

func TestRegister_OneReturnValue(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input *pb.InputString) *pb.LetterCount {
		return nil
	})

	if err == nil {
		t.Error("expected error for single return value, got nil")
	}
}

func TestRegister_ThreeReturnValues(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error, int) {
		return nil, nil, 0
	})

	if err == nil {
		t.Error("expected error for three return values, got nil")
	}
}

func TestRegister_NonProtoOutput(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input *pb.InputString) (string, error) {
		return "", nil
	})

	if err == nil {
		t.Error("expected error for non-proto.Message return type, got nil")
	}
}

func TestRegister_NonErrorSecondReturn(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, string) {
		return nil, ""
	})

	if err == nil {
		t.Error("expected error for non-error second return, got nil")
	}
}

func TestRegister_NonProtoDependency(t *testing.T) {
	g := NewGraph()

	err := g.Register(func(ctx context.Context, input string) (*pb.LetterCount, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for non-proto.Message dependency, got nil")
	}
}

func TestRegister_NoDependencies(t *testing.T) {
	g := NewGraph()

	// Function with only context (no dependencies) - should be valid
	err := g.Register(func(ctx context.Context) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: 42}, nil
	})

	if err != nil {
		t.Errorf("unexpected error for no-dependency function: %v", err)
	}
}

// ============ Struct Registration Tests ============

type NoComputeStruct struct{}

func TestRegister_StructWithoutCompute(t *testing.T) {
	g := NewGraph()

	err := g.Register(&NoComputeStruct{})

	if err == nil {
		t.Error("expected error for struct without Compute method, got nil")
	}
}

type WrongComputeStruct struct{}

func (s *WrongComputeStruct) Compute(notCtx string) (*pb.LetterCount, error) {
	return nil, nil
}

func TestRegister_StructWithWrongComputeSignature(t *testing.T) {
	g := NewGraph()

	err := g.Register(&WrongComputeStruct{})

	if err == nil {
		t.Error("expected error for struct with wrong Compute signature, got nil")
	}
}

type ValidComputeStruct struct {
	Value int
}

func (s *ValidComputeStruct) Compute(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	return &pb.LetterCount{Count: int32(s.Value)}, nil
}

func TestRegister_ValidStruct(t *testing.T) {
	g := NewGraph()

	err := g.Register(&ValidComputeStruct{Value: 42})

	if err != nil {
		t.Errorf("unexpected error for valid struct: %v", err)
	}

	g.Validate()

	result, err := Execute[*pb.LetterCount](context.Background(), g, "test", &pb.InputString{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Count != 42 {
		t.Errorf("expected Count 42, got %d", result.Count)
	}
}

// ============ Registration After Validation ============

func TestRegister_AfterValidation(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	g.Validate()

	// Try to register after validation
	err := g.Register(func(ctx context.Context, input *pb.InputString) (*pb.InputString, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for registration after validation, got nil")
	}
}

func TestRegisterAggregate_AfterValidation(t *testing.T) {
	g := NewGraph()

	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	g.Validate()

	// Try to register aggregate after validation
	err := g.RegisterAggregate(func(ctx context.Context, inputs []*pb.InputString) (*pb.InputString, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for aggregate registration after validation, got nil")
	}
}

// ============ Aggregate Registration Tests ============

func TestRegisterAggregate_NonSliceParameter(t *testing.T) {
	g := NewGraph()

	err := g.RegisterAggregate(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for non-slice parameter in aggregate, got nil")
	}
}

func TestRegisterAggregate_NonProtoSliceElement(t *testing.T) {
	g := NewGraph()

	err := g.RegisterAggregate(func(ctx context.Context, inputs []string) (*pb.LetterCount, error) {
		return nil, nil
	})

	if err == nil {
		t.Error("expected error for non-proto slice element in aggregate, got nil")
	}
}

func TestRegisterAggregate_NotAFunction(t *testing.T) {
	g := NewGraph()

	err := g.RegisterAggregate(&ValidComputeStruct{})

	if err == nil {
		t.Error("expected error for struct passed to RegisterAggregate, got nil")
	}
}

func TestRegisterAggregate_Valid(t *testing.T) {
	g := NewGraph()

	err := g.RegisterAggregate(func(ctx context.Context, inputs []*pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: int32(len(inputs))}, nil
	})

	if err != nil {
		t.Errorf("unexpected error for valid aggregate: %v", err)
	}

	steps := g.Steps()
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if !steps[0].IsAggregate {
		t.Error("expected step to be marked as aggregate")
	}
}

// ============ Not a Function or Struct ============

func TestRegister_Integer(t *testing.T) {
	g := NewGraph()

	err := g.Register(42)

	if err == nil {
		t.Error("expected error for integer passed to Register, got nil")
	}
}

func TestRegister_String(t *testing.T) {
	g := NewGraph()

	err := g.Register("not a function")

	if err == nil {
		t.Error("expected error for string passed to Register, got nil")
	}
}

func TestRegister_Nil(t *testing.T) {
	g := NewGraph()

	err := g.Register(nil)

	if err == nil {
		t.Error("expected error for nil passed to Register, got nil")
	}
}
