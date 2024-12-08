package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

func main() {
	if err := run("cache_example.db"); err != nil {
		log.Fatal(err)
	}
}

func run(dbFile string) error {
	// 1. Setup SQLite Store
	_ = os.Remove(dbFile) // Start fresh
	defer os.Remove(dbFile)

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return fmt.Errorf("failed to open sqlite: %w", err)
	}
	defer db.Close()

	// Use generic SQLStore with DialectSQLite
	store := docket.NewSQLStore(db, "step_cache", docket.DialectSQLite)
	if err := store.InitSchema(context.Background()); err != nil {
		return fmt.Errorf("failed to init schema: %w", err)
	}

	// 2. Setup Graph
	g := docket.NewGraph()

	// Register a slow step
	g.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			log.Printf("  [Computing] Count for %q (sleeping 200ms)...", input.Value)
			time.Sleep(200 * time.Millisecond)
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		docket.WithName("SlowCount"),
		// Cache globally for 1 hour
		docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour),
	)

	if err := g.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	input := &pb.InputString{Value: "hello sqlite"}

	// 3. First Run (Cold)
	fmt.Println("--- Run 1 (Cold) ---")
	start := time.Now()
	_, err = docket.Execute[*pb.LetterCount](ctx, g, "exec-1", input)
	if err != nil {
		return err
	}
	elapsed1 := time.Since(start)
	fmt.Printf("Duration: %v\n", elapsed1)

	// Verify row created in DB
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM step_cache").Scan(&count); err != nil {
		return fmt.Errorf("failed to query cache table: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("verification failed: expected rows in step_cache, got 0")
	}
	fmt.Printf("Verified: Found %d row(s) in database\n", count)

	// 4. Second Run (Warm)
	fmt.Println("\n--- Run 2 (Warm) ---")
	start = time.Now()
	_, err = docket.Execute[*pb.LetterCount](ctx, g, "exec-2", input)
	if err != nil {
		return err
	}
	elapsed2 := time.Since(start)
	fmt.Printf("Duration: %v\n", elapsed2)

	// Verification
	if elapsed2 > 50*time.Millisecond {
		return fmt.Errorf("FAILED: Cache hit was too slow (%v)", elapsed2)
	}
	fmt.Println("\nSUCCESS: Cache hit confirmed!")
	return nil
}
