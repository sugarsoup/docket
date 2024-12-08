// Package main demonstrates integrating Docket with River queue.
//
// This example shows:
// 1. Setting up a River client with a Docket worker
// 2. Enqueueing jobs that execute Docket graphs
// 3. Using persistence to cache step results across job retries
//
// To run this example, you need a running Postgres instance.
// See run.sh for automated setup.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"google.golang.org/protobuf/proto"

	"docket/pkg/docket"
	riveradapter "docket/pkg/river"
	pb "docket/proto/examples/lettercount"
)

// CountLettersArgs are the arguments for the letter counting job.
type CountLettersArgs struct {
	Text string `json:"text"`
}

func (CountLettersArgs) Kind() string { return "count_letters" }

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	// 1. Connect to Postgres
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/river_example?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer pool.Close()

	// 2. Setup Docket with Postgres persistence
	store := docket.NewPostgresStore(pool, "protograph_cache")
	if err := store.InitSchema(ctx); err != nil {
		return fmt.Errorf("failed to init docket schema: %w", err)
	}

	// 3. Build the graph
	graph := docket.NewGraph()
	graph.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			log.Printf("[Worker] Computing letter count for %q", input.Value)
			// Simulate slow computation
			time.Sleep(100 * time.Millisecond)
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		docket.WithName("CountLetters"),
		docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
	)
	if err := graph.Validate(); err != nil {
		return fmt.Errorf("graph validation failed: %w", err)
	}

	// 4. Create the River worker
	worker := riveradapter.NewGraphWorker[CountLettersArgs, *pb.LetterCount](
		graph,
		func(args CountLettersArgs) []proto.Message {
			return []proto.Message{&pb.InputString{Value: args.Text}}
		},
	)

	// 5. Setup River client
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers: workers,
	})
	if err != nil {
		return fmt.Errorf("failed to create river client: %w", err)
	}

	// 6. Start the client
	if err := riverClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start river client: %w", err)
	}

	// 7. Enqueue some jobs
	fmt.Println("\n--- Enqueueing Jobs ---")
	texts := []string{"hello", "world", "hello"} // Note: "hello" appears twice

	for _, text := range texts {
		_, err := riverClient.Insert(ctx, CountLettersArgs{Text: text}, nil)
		if err != nil {
			return fmt.Errorf("failed to insert job: %w", err)
		}
		fmt.Printf("Enqueued job for %q\n", text)
	}

	// 8. Wait for jobs to complete (or timeout)
	fmt.Println("\n--- Processing Jobs ---")
	time.Sleep(2 * time.Second)

	// 9. Enqueue the same text again to demonstrate cache hit
	fmt.Println("\n--- Enqueueing Duplicate (should hit cache) ---")
	_, err = riverClient.Insert(ctx, CountLettersArgs{Text: "hello"}, nil)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	fmt.Println("Enqueued duplicate job for \"hello\"")

	time.Sleep(500 * time.Millisecond)

	// 10. Graceful shutdown
	fmt.Println("\n--- Shutting Down ---")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Give a moment for final processing
	select {
	case <-sigCh:
		fmt.Println("Received shutdown signal")
	case <-time.After(1 * time.Second):
		fmt.Println("Auto-shutdown after demo")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := riverClient.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("failed to stop river client: %w", err)
	}

	fmt.Println("SUCCESS: River + Docket integration complete!")
	return nil
}
