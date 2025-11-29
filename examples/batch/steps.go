package main

import (
	"context"

	pb "protograph/proto/examples/batch"
)

// ComputeBatchStats is an aggregate step (fan-in) that computes statistics
// across all movies in the batch. It runs ONCE for the entire batch.
//
// Input: []*pb.Movie - all movies in the batch
// Output: *pb.BatchStats - aggregate statistics
func ComputeBatchStats(ctx context.Context, movies []*pb.Movie) (*pb.BatchStats, error) {
	if len(movies) == 0 {
		return &pb.BatchStats{}, nil
	}

	var totalRating float64
	var totalRuntime int32
	maxRating := movies[0].Rating
	minRating := movies[0].Rating

	for _, movie := range movies {
		totalRating += movie.Rating
		totalRuntime += movie.RuntimeMinutes
		if movie.Rating > maxRating {
			maxRating = movie.Rating
		}
		if movie.Rating < minRating {
			minRating = movie.Rating
		}
	}

	count := len(movies)
	return &pb.BatchStats{
		Count:          int32(count),
		AverageRating:  totalRating / float64(count),
		AverageRuntime: float64(totalRuntime) / float64(count),
		MaxRating:      maxRating,
		MinRating:      minRating,
		TotalRuntime:   totalRuntime,
	}, nil
}

// EnrichMovie is a per-item step that enriches each movie with computed fields
// based on how it compares to the batch statistics.
//
// Input: *pb.Movie - the movie to enrich
// Input: *pb.BatchStats - aggregate statistics from ComputeBatchStats
// Output: *pb.EnrichedMovie - the movie with additional computed fields
func EnrichMovie(ctx context.Context, movie *pb.Movie, stats *pb.BatchStats) (*pb.EnrichedMovie, error) {
	aboveAvgRating := movie.Rating > stats.AverageRating
	aboveAvgRuntime := float64(movie.RuntimeMinutes) > stats.AverageRuntime

	// Calculate rating percentile (where this movie ranks in the batch)
	ratingRange := stats.MaxRating - stats.MinRating
	var percentile float64
	if ratingRange > 0 {
		percentile = ((movie.Rating - stats.MinRating) / ratingRange) * 100
	}

	return &pb.EnrichedMovie{
		Original:            movie,
		AboveAverageRating:  aboveAvgRating,
		AboveAverageRuntime: aboveAvgRuntime,
		RatingPercentile:    percentile,
	}, nil
}

