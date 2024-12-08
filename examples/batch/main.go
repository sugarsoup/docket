package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"docket/pkg/docket"
	pb "docket/proto/examples/batch"
)

func main() {
	fmt.Println("=== Docket Fan-In/Aggregate Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates the fan-in pattern where:")
	fmt.Println("  1. An AGGREGATE step processes all items to compute statistics")
	fmt.Println("  2. A PER-ITEM step enriches each item using those statistics")
	fmt.Println()

	// ============ SETUP PHASE ============

	g := docket.NewGraph()

	// Register the aggregate step (fan-in) using a named function
	err := g.RegisterAggregate(
		ComputeBatchStats, // Named function from steps.go
		docket.WithName("ComputeBatchStats"),
	)
	if err != nil {
		log.Fatalf("Failed to register ComputeBatchStats: %v", err)
	}

	// Register the per-item step using a named function
	err = g.Register(
		EnrichMovie, // Named function from steps.go
		docket.WithName("EnrichMovie"),
	)
	if err != nil {
		log.Fatalf("Failed to register EnrichMovie: %v", err)
	}

	if err := g.Validate(); err != nil {
		log.Fatalf("Graph validation failed: %v", err)
	}

	fmt.Println("✓ Graph setup complete")
	fmt.Println("  Steps:")
	fmt.Println("    1. ComputeBatchStats (aggregate/fan-in) - processes all movies at once")
	fmt.Println("    2. EnrichMovie (per-item) - uses batch stats to enrich each movie")
	fmt.Println()

	// ============ CREATE TEST DATA ============

	movies := createTestMovies()

	fmt.Printf("📥 Input: %d movies\n", len(movies))
	for _, movie := range movies {
		fmt.Printf("    %s (%d) - %.1f rating, %d min\n",
			movie.Title, movie.Year, movie.Rating, movie.RuntimeMinutes)
	}
	fmt.Println()

	// ============ EXECUTE BATCH ============

	ctx := context.Background()

	fmt.Println("⚙️  Executing batch pipeline...")
	startTime := time.Now()

	results, err := docket.ExecuteBatch[*pb.Movie, *pb.EnrichedMovie](
		ctx, g, "batch-001", movies,
	)
	if err != nil {
		log.Fatalf("Batch execution failed: %v", err)
	}

	elapsed := time.Since(startTime)

	fmt.Println()
	fmt.Println("✅ Batch execution successful!")
	fmt.Printf("   Time: %v\n", elapsed)
	fmt.Println()

	// ============ DISPLAY RESULTS ============

	displayResults(movies, results)
}

// createTestMovies returns a batch of movies for the example.
func createTestMovies() []*pb.Movie {
	return []*pb.Movie{
		{Id: "tt0111161", Title: "The Shawshank Redemption", Year: 1994, RuntimeMinutes: 142, Rating: 9.3, Genre: "Drama"},
		{Id: "tt0068646", Title: "The Godfather", Year: 1972, RuntimeMinutes: 175, Rating: 9.2, Genre: "Crime"},
		{Id: "tt0468569", Title: "The Dark Knight", Year: 2008, RuntimeMinutes: 152, Rating: 9.0, Genre: "Action"},
		{Id: "tt0071562", Title: "The Godfather Part II", Year: 1974, RuntimeMinutes: 202, Rating: 9.0, Genre: "Crime"},
		{Id: "tt0050083", Title: "12 Angry Men", Year: 1957, RuntimeMinutes: 96, Rating: 9.0, Genre: "Drama"},
	}
}

// displayResults shows the computed statistics and enriched movies.
func displayResults(movies []*pb.Movie, results []*pb.EnrichedMovie) {
	// Compute stats for display (mirrors what the aggregate step computed)
	var totalRating float64
	var totalRuntime int32
	for _, movie := range movies {
		totalRating += movie.Rating
		totalRuntime += movie.RuntimeMinutes
	}
	avgRating := totalRating / float64(len(movies))
	avgRuntime := float64(totalRuntime) / float64(len(movies))

	fmt.Println("📊 Batch Statistics (computed once via fan-in):")
	fmt.Printf("   Count:           %d movies\n", len(movies))
	fmt.Printf("   Average Rating:  %.2f\n", avgRating)
	fmt.Printf("   Average Runtime: %.0f min\n", avgRuntime)
	fmt.Printf("   Total Runtime:   %d min (%.1f hours)\n", totalRuntime, float64(totalRuntime)/60)
	fmt.Println()

	fmt.Println("📤 Enriched Movies (each compared to batch averages):")

	// Sort by rating percentile for display
	sortedResults := make([]*pb.EnrichedMovie, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].RatingPercentile > sortedResults[j].RatingPercentile
	})

	for i, enriched := range sortedResults {
		ratingIndicator := "  "
		if enriched.AboveAverageRating {
			ratingIndicator = "⬆️"
		}
		runtimeIndicator := ""
		if enriched.AboveAverageRuntime {
			runtimeIndicator = " (long)"
		}

		fmt.Printf("   %d. %s %s (%.1f rating, percentile: %.0f%%)%s\n",
			i+1,
			ratingIndicator,
			enriched.Original.Title,
			enriched.Original.Rating,
			enriched.RatingPercentile,
			runtimeIndicator,
		)
	}

	fmt.Println()
	fmt.Println("💡 Fan-In Pattern:")
	fmt.Println("   • ComputeBatchStats runs ONCE for all movies (aggregate/fan-in)")
	fmt.Println("   • EnrichMovie runs for EACH movie (per-item/fan-out)")
	fmt.Println("   • The aggregate result (BatchStats) is shared with per-item steps")
	fmt.Println("   • This enables comparisons like 'above average rating'")
}
