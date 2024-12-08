package main

import (
	"context"
	"strings"

	pb "docket/proto/examples/lettercount"
)

// CountLetterR counts occurrences of the letter 'r' in an input string.
// This is the step function registered with the graph.
//
// Input: *pb.InputString - the string to analyze
// Output: *pb.LetterCount - the count result
func CountLetterR(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	count := strings.Count(strings.ToLower(input.Value), "r")

	return &pb.LetterCount{
		OriginalString: input.Value,
		TargetLetter:   "r",
		Count:          int32(count),
	}, nil
}

// CountLetterRPlain is the plain Go implementation (no framework) for benchmarking.
func CountLetterRPlain(input *pb.InputString) *pb.LetterCount {
	count := strings.Count(strings.ToLower(input.Value), "r")
	return &pb.LetterCount{
		OriginalString: input.Value,
		TargetLetter:   "r",
		Count:          int32(count),
	}
}
