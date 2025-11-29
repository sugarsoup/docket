package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/lettercount"
)

func main() {
	// 1. Setup SQLite Store
	dbFile := "cache_example.db"
	_ = os.Remove(dbFile) // Start fresh
	defer os.Remove(dbFile)

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("Failed to open sqlite: %v", err)
	}
	defer db.Close()

	// Use generic SQLStore with DialectSQLite
	store := protograph.NewSQLStore(db, "step_cache", protograph.DialectSQLite)
	if err := store.InitSchema(context.Background()); err != nil {
		log.Fatalf("Failed to init schema: %v", err)
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
	input := &pb.InputString{Value: "hello sqlite"}

	// 3. First Run (Cold)
	fmt.Println("--- Run 1 (Cold) ---")
	start := time.Now()
	_, err = protograph.Execute[*pb.LetterCount](ctx, g, "exec-1", input)
	if err != nil {
		log.Fatal(err)
	}
	elapsed1 := time.Since(start)
	fmt.Printf("Duration: %v\n", elapsed1)

	// 4. Second Run (Warm)
	fmt.Println("\n--- Run 2 (Warm) ---")
	start = time.Now()
	_, err = protograph.Execute[*pb.LetterCount](ctx, g, "exec-2", input)
	if err != nil {
		log.Fatal(err)
	}
	elapsed2 := time.Since(start)
	fmt.Printf("Duration: %v\n", elapsed2)

	// Verification
	if elapsed2 > 50*time.Millisecond {
		log.Fatalf("FAILED: Cache hit was too slow (%v)", elapsed2)
	} else {
		fmt.Println("\nSUCCESS: Cache hit confirmed!")
	}
}

