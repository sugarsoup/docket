package main

import (
	"context"
	"testing"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func TestConfigurableStep(t *testing.T) {
	g := protograph.NewGraph()

	counter := &LetterCounter{
		TargetLetter:  "e",
		CaseSensitive: false,
	}

	if err := g.Register(counter, protograph.WithName("CountE")); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "test-conf", &pb.InputString{Value: "Excellence"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 4 {
		t.Errorf("expected count 4, got %d", result.Count)
	}
}

func TestDependencyInjection(t *testing.T) {
	g := protograph.NewGraph()

	client := &APIClient{BaseURL: "https://mock.api"}
	scorer := &WordScorer{
		Client: client,
		Prefix: "test:",
	}

	if err := g.Register(scorer, protograph.WithName("ScoreWord")); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	ctx := context.Background()
	result, err := protograph.Execute[*pb.LetterCount](ctx, g, "test-di", &pb.InputString{Value: "abc"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Score logic: len("abc") * 10 = 30
	if result.Count != 30 {
		t.Errorf("expected score 30, got %d", result.Count)
	}
	if result.OriginalString != "test:abc" {
		t.Errorf("expected prefix 'test:', got %q", result.OriginalString)
	}
}
