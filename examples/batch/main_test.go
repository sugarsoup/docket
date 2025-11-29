package main

import (
	"context"
	"testing"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/batch"
)

func TestBatchLogic(t *testing.T) {
	// Setup graph
	g := protograph.NewGraph()
	
	if err := g.RegisterAggregate(ComputeBatchStats, protograph.WithName("ComputeBatchStats")); err != nil {
		t.Fatalf("RegisterAggregate failed: %v", err)
	}
	if err := g.Register(EnrichMovie, protograph.WithName("EnrichMovie")); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Create test data: 3 movies with ratings 10, 8, 6
	// Average should be 8.0
	movies := []*pb.Movie{
		{Title: "Good", Rating: 10.0, RuntimeMinutes: 100},
		{Title: "Average", Rating: 8.0, RuntimeMinutes: 100},
		{Title: "Bad", Rating: 6.0, RuntimeMinutes: 100},
	}

	ctx := context.Background()
	results, err := protograph.ExecuteBatch[*pb.Movie, *pb.EnrichedMovie](ctx, g, "test-batch", movies)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify logic: "Good" (10.0) > 8.0 (Average)
	if !results[0].AboveAverageRating {
		t.Error("expected 'Good' movie to be above average")
	}
	if results[0].RatingPercentile != 100.0 {
		t.Errorf("expected 'Good' to be 100th percentile, got %.1f", results[0].RatingPercentile)
	}

	// Verify logic: "Bad" (6.0) < 8.0 (Average)
	if results[2].AboveAverageRating {
		t.Error("expected 'Bad' movie to be below average")
	}
	if results[2].RatingPercentile != 0.0 {
		t.Errorf("expected 'Bad' to be 0th percentile, got %.1f", results[2].RatingPercentile)
	}
}
