package river

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"google.golang.org/protobuf/proto"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

// TestJobArgs is a simple job args type for testing.
type TestJobArgs struct {
	Value string `json:"value"`
}

func (TestJobArgs) Kind() string { return "test_job" }

// Verify TestJobArgs implements JobArgs
var _ JobArgs = TestJobArgs{}

// newTestJob creates a test job with the given ID and args.
func newTestJob[T river.JobArgs](id int64, args T) *river.Job[T] {
	return &river.Job[T]{
		JobRow: &rivertype.JobRow{
			ID: id,
		},
		Args: args,
	}
}

func TestGraphWorker_Work(t *testing.T) {
	// Setup graph
	g := docket.NewGraph()
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	// Create worker
	worker := NewGraphWorker[TestJobArgs, *pb.LetterCount](
		g,
		func(args TestJobArgs) []proto.Message {
			return []proto.Message{&pb.InputString{Value: args.Value}}
		},
	)

	// Create a mock job
	job := newTestJob(123, TestJobArgs{Value: "hello"})

	// Execute
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("Work failed: %v", err)
	}
}

func TestGraphWorker_ContextCancellation(t *testing.T) {
	// Setup graph with a slow step
	g := docket.NewGraph()
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		select {
		case <-time.After(5 * time.Second):
			return &pb.LetterCount{Count: 1}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	worker := NewGraphWorker[TestJobArgs, *pb.LetterCount](
		g,
		func(args TestJobArgs) []proto.Message {
			return []proto.Message{&pb.InputString{Value: args.Value}}
		},
	)

	job := newTestJob(456, TestJobArgs{Value: "test"})

	// Cancel context quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := worker.Work(ctx, job)

	// Should return an error (either JobCancel wrapping context.Canceled, or DeadlineExceeded)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Verify it's a cancellation-related error
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context error, got: %v", err)
	}
}

func TestGraphWorker_StepError(t *testing.T) {
	stepErr := errors.New("step failed")

	g := docket.NewGraph()
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		return nil, stepErr
	})
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	worker := NewGraphWorker[TestJobArgs, *pb.LetterCount](
		g,
		func(args TestJobArgs) []proto.Message {
			return []proto.Message{&pb.InputString{Value: args.Value}}
		},
	)

	job := newTestJob(789, TestJobArgs{Value: "test"})

	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Error should be returned for retry
	if !errors.Is(err, stepErr) {
		t.Errorf("Expected step error in chain, got: %v", err)
	}
}

func TestGraphWorker_ExecutionID(t *testing.T) {
	var capturedExecID string

	g := docket.NewGraph()
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		capturedExecID = docket.ExecutionID(ctx)
		return &pb.LetterCount{Count: 1}, nil
	})
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	worker := NewGraphWorker[TestJobArgs, *pb.LetterCount](
		g,
		func(args TestJobArgs) []proto.Message {
			return []proto.Message{&pb.InputString{Value: args.Value}}
		},
	)

	job := newTestJob(42, TestJobArgs{Value: "test"})

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	// Verify execution ID matches River job ID
	expected := "42"
	if capturedExecID != expected {
		t.Errorf("Expected execution ID %q, got %q", expected, capturedExecID)
	}
}

// BatchTestJobArgs for batch worker testing
type BatchTestJobArgs struct {
	Values []string `json:"values"`
}

func (BatchTestJobArgs) Kind() string { return "batch_test_job" }

func TestBatchGraphWorker_Work(t *testing.T) {
	var callCount int

	g := docket.NewGraph()
	g.Register(func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
		callCount++
		return &pb.LetterCount{Count: int32(len(input.Value))}, nil
	})
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}

	worker := NewBatchGraphWorker[BatchTestJobArgs, *pb.InputString, *pb.LetterCount](
		g,
		func(args BatchTestJobArgs) []*pb.InputString {
			inputs := make([]*pb.InputString, len(args.Values))
			for i, v := range args.Values {
				inputs[i] = &pb.InputString{Value: v}
			}
			return inputs
		},
	)

	job := newTestJob(999, BatchTestJobArgs{Values: []string{"a", "bb", "ccc"}})

	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("Work failed: %v", err)
	}

	// Verify all items were processed
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}
