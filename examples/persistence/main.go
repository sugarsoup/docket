package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func main() {
	// 1. Setup Persistence
	// In a real app, you might use SQLite:
	//
	// import _ "github.com/mattn/go-sqlite3"
	//
	// db, _ := sql.Open("sqlite3", "cache.db")
	// store := protograph.NewSQLStore(db, "step_cache", protograph.DialectSQLite)
	// if err := store.InitSchema(context.Background()); err != nil {
	//     panic(err)
	// }

	// For this example, we use in-memory storage:
	store := protograph.NewInMemoryStore()
	fmt.Println("Store initialized (In-Memory)")

	// 2. Define Graph
	g := protograph.NewGraph()

	// 3. Register a "slow" step with caching
	// We use ScopeGlobal so the result is shared across different execution IDs.
	// We set a TTL of 1 minute.
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			log.Printf("  [SlowStep] Computing count for %q (Simulated 500ms latency)...", input.Value)
			time.Sleep(500 * time.Millisecond)
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		protograph.WithName("SlowCount"),
		protograph.WithPersistence(store, protograph.ScopeGlobal, 1*time.Minute),
	)

	if err := g.Validate(); err != nil {
		log.Fatalf("Graph validation failed: %v", err)
	}

	ctx := context.Background()
	input := &pb.InputString{Value: "persistence_demo"}

	// 4. First Execution: Cache Miss
	fmt.Println("\n--- Run 1 (Cold Cache) ---")
	start := time.Now()
	result1, err := protograph.Execute[*pb.LetterCount](ctx, g, "exec-001", input)
	if err != nil {
		log.Fatal(err)
	}
	duration1 := time.Since(start)
	fmt.Printf("Result: %d, Duration: %v\n", result1.Count, duration1)

	// 5. Second Execution: Cache Hit
	// Different execution ID, but same input + ScopeGlobal = Hit
	fmt.Println("\n--- Run 2 (Warm Cache) ---")
	start = time.Now()
	result2, err := protograph.Execute[*pb.LetterCount](ctx, g, "exec-002", input)
	if err != nil {
		log.Fatal(err)
	}
	duration2 := time.Since(start)
	fmt.Printf("Result: %d, Duration: %v\n", result2.Count, duration2)

	if duration2 > 10*time.Millisecond {
		fmt.Println("⚠️  Warning: Cache hit was slower than expected!")
	} else {
		fmt.Println("✅ Cache hit confirmed (execution was instant)")
	}
}
