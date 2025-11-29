package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func main() {
	// 1. Setup Postgres Store
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/protograph_example?sslmode=disable"
		fmt.Println("No DATABASE_URL set, using default:", connStr)
		fmt.Println("(Ensure you have a running Postgres instance: docker run -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres)")
	}

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	// Use PostgresStore (native pgx)
	store := protograph.NewPostgresStore(pool, "step_cache")
	if err := store.InitSchema(context.Background()); err != nil {
		// Just log error but continue, might fail if DB down
		log.Printf("Failed to init schema (DB might be unreachable): %v", err)
		if os.Getenv("SKIP_DB_CHECK") == "" {
			os.Exit(1)
		}
	}

	// 2. Setup Graph
	g := protograph.NewGraph()

	// Register a slow step
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			log.Printf("  [Computing] Count for %q (sleeping 200ms)...", input.Value)
			time.Sleep(200 * time.Millisecond)
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		protograph.WithName("SlowCount"),
		// Cache globally for 1 hour
		protograph.WithPersistence(store, protograph.ScopeGlobal, 1*time.Hour),
	)

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	input := &pb.InputString{Value: "hello postgres"}

	// 3. First Run (Cold)
	fmt.Println("\n--- Run 1 (Cold) ---")
	start := time.Now()
	_, err = protograph.Execute[*pb.LetterCount](ctx, g, "exec-pg-1", input)
	if err != nil {
		log.Fatal(err)
	}
	elapsed1 := time.Since(start)
	fmt.Printf("Duration: %v\n", elapsed1)

	// 4. Second Run (Warm)
	fmt.Println("\n--- Run 2 (Warm) ---")
	start = time.Now()
	_, err = protograph.Execute[*pb.LetterCount](ctx, g, "exec-pg-2", input)
	if err != nil {
		log.Fatal(err)
	}
	elapsed2 := time.Since(start)
	fmt.Printf("Duration: %v\n", elapsed2)

	// Verification
	if elapsed2 > 50*time.Millisecond {
		log.Printf("⚠️  Warning: Cache hit was slower than expected (%v)", elapsed2)
	} else {
		fmt.Println("\nSUCCESS: Cache hit confirmed!")
	}
}

