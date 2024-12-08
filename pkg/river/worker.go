// Package river provides integration between Docket and River queue.
//
// This package provides a generic worker adapter that executes Docket graphs
// as River jobs. It handles:
//   - Mapping River job IDs to Docket execution IDs
//   - Context propagation for graceful shutdown
//   - Error classification for River's retry logic
package river

import (
	"context"
	"errors"
	"fmt"

	"github.com/riverqueue/river"
	"google.golang.org/protobuf/proto"

	"docket/pkg/docket"
)

// GraphWorker is a River worker that executes a Docket graph.
// It implements river.Worker for a specific JobArgs type.
//
// Type parameters:
//   - Args: The River job args type (must implement JobArgs)
//   - Output: The Docket output proto type
type GraphWorker[Args JobArgs, Output proto.Message] struct {
	river.WorkerDefaults[Args]

	// Graph is the pre-validated Docket graph to execute
	Graph *docket.Graph

	// InputMapper converts River job args to Docket input protos
	InputMapper func(args Args) []proto.Message
}

// JobArgs is the interface that River job args must implement.
// It extends river.JobArgs with a method to get a stable job identifier.
type JobArgs interface {
	river.JobArgs
}

// Work executes the Docket graph for the given job.
// The River job ID is used as the Docket execution ID for traceability.
func (w *GraphWorker[Args, Output]) Work(ctx context.Context, job *river.Job[Args]) error {
	// Map job args to proto inputs
	inputs := w.InputMapper(job.Args)

	// Use River job ID directly as execution ID
	executionID := fmt.Sprintf("%d", job.ID)

	// Execute the graph
	_, err := docket.Execute[Output](ctx, w.Graph, executionID, inputs...)
	if err != nil {
		// Classify error for River's retry logic
		return classifyError(err)
	}

	return nil
}

// BatchGraphWorker is a River worker that executes a Docket batch graph.
type BatchGraphWorker[Args BatchJobArgs[Input], Input, Output proto.Message] struct {
	river.WorkerDefaults[Args]

	// Graph is the pre-validated Docket graph to execute
	Graph *docket.Graph

	// InputMapper extracts the batch items from job args
	InputMapper func(args Args) []Input
}

// BatchJobArgs is the interface for batch job arguments.
type BatchJobArgs[Input proto.Message] interface {
	river.JobArgs
}

// Work executes the Docket batch graph for the given job.
func (w *BatchGraphWorker[Args, Input, Output]) Work(ctx context.Context, job *river.Job[Args]) error {
	// Map job args to batch inputs
	inputs := w.InputMapper(job.Args)

	// Use River job ID directly as execution ID
	executionID := fmt.Sprintf("%d", job.ID)

	// Execute the batch graph
	_, err := docket.ExecuteBatch[Input, Output](ctx, w.Graph, executionID, inputs)
	if err != nil {
		return classifyError(err)
	}

	return nil
}

// classifyError converts Docket errors to River-appropriate errors.
// This helps River decide whether to retry or discard the job.
func classifyError(err error) error {
	var execErr *docket.ExecutionError
	if errors.As(err, &execErr) {
		// Check for panic errors - these might indicate bugs, but could be transient
		var panicErr *docket.StepPanicError
		if errors.As(err, &panicErr) {
			// Wrap with context but allow retry (panics could be due to bad data)
			return fmt.Errorf("step panic in %s: %w", panicErr.StepName, err)
		}

		// Check for dependency errors
		var depErr *docket.DependencyError
		if errors.As(err, &depErr) {
			return fmt.Errorf("dependency failed for %s: %w", depErr.StepName, err)
		}
	}

	// Context cancellation - don't retry, job was cancelled
	if errors.Is(err, context.Canceled) {
		return river.JobCancel(err)
	}

	// Deadline exceeded - allow retry with backoff
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	// Default: return error as-is, let River retry
	return err
}

// NewGraphWorker creates a new GraphWorker with the given configuration.
func NewGraphWorker[Args JobArgs, Output proto.Message](
	graph *docket.Graph,
	inputMapper func(args Args) []proto.Message,
) *GraphWorker[Args, Output] {
	return &GraphWorker[Args, Output]{
		Graph:       graph,
		InputMapper: inputMapper,
	}
}

// NewBatchGraphWorker creates a new BatchGraphWorker with the given configuration.
func NewBatchGraphWorker[Args BatchJobArgs[Input], Input, Output proto.Message](
	graph *docket.Graph,
	inputMapper func(args Args) []Input,
) *BatchGraphWorker[Args, Input, Output] {
	return &BatchGraphWorker[Args, Input, Output]{
		Graph:       graph,
		InputMapper: inputMapper,
	}
}
