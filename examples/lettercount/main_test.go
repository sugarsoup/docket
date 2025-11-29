package main

import (
	"context"
	"testing"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func TestLetterCountLogic(t *testing.T) {
	// Setup the graph exactly as main() does
	g := protograph.NewGraph()
	err := g.Register(
		CountLetterR,
		protograph.WithName("CountLetterR"),
	)
	if err != nil {
		t.Fatalf("Failed to register step: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("Graph validation failed: %v", err)
	}

	// Test cases
	tests := []struct {
		input    string
		expected int32
	}{
		{"strawberry", 3}, // r, r, r
		{"raspberry", 3},  // r, r, r
		{"apple", 0},      // no r
		{"RACEAR", 2},     // R, R (case insensitive)
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			input := &pb.InputString{Value: tt.input}
			result, err := protograph.Execute[*pb.LetterCount](ctx, g, "test-"+tt.input, input)
			if err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			if result.Count != tt.expected {
				t.Errorf("expected count %d, got %d", tt.expected, result.Count)
			}
			if result.TargetLetter != "r" {
				t.Errorf("expected target letter 'r', got %q", result.TargetLetter)
			}
		})
	}
}

func TestLetterCountPlainLogic(t *testing.T) {
	// Verify the plain Go implementation matches expectations
	input := &pb.InputString{Value: "strawberry"}
	result := CountLetterRPlain(input)

	if result.Count != 3 {
		t.Errorf("expected 3, got %d", result.Count)
	}
}
