package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// 1. Setup MySQL Connection
	// Format: username:password@tcp(host:port)/database
	dsn := "root@tcp(localhost:3306)/docket_test?parseTime=true"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open mysql: %w", err)
	}
	defer db.Close()

	// Test connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to connect to mysql: %w (is MySQL running?)", err)
	}
	fmt.Println("Connected to MySQL successfully")

	// Create database if it doesn't exist
	if err := createDatabaseIfNotExists(db); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Use SQLStore with DialectMySQL
	store := docket.NewSQLStore(db, "step_cache", docket.DialectMySQL)
	if err := store.InitSchema(ctx); err != nil {
		return fmt.Errorf("failed to init schema: %w", err)
	}
	fmt.Println("Database schema initialized")

	// Clean up existing data
	if _, err := db.ExecContext(ctx, "DELETE FROM step_cache"); err != nil {
		log.Printf("Warning: cleanup failed: %v", err)
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

	input := &pb.InputString{Value: "hello mysql"}

	// 3. First Run (Cold)
	fmt.Println("\n--- Run 1 (Cold) ---")
	start := time.Now()
	result1, err := docket.Execute[*pb.LetterCount](ctx, g, "exec-1", input)
	if err != nil {
		return err
	}
	elapsed1 := time.Since(start)
	fmt.Printf("Result: Count = %d\n", result1.Count)
	fmt.Printf("Duration: %v\n", elapsed1)

	// Verify row created in DB
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM step_cache").Scan(&count); err != nil {
		return fmt.Errorf("failed to query cache table: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("verification failed: expected rows in step_cache, got 0")
	}
	fmt.Printf("Verified: Found %d row(s) in database\n", count)

	// 4. Second Run (Warm)
	fmt.Println("\n--- Run 2 (Warm) ---")
	start = time.Now()
	result2, err := docket.Execute[*pb.LetterCount](ctx, g, "exec-2", input)
	if err != nil {
		return err
	}
	elapsed2 := time.Since(start)
	fmt.Printf("Result: Count = %d\n", result2.Count)
	fmt.Printf("Duration: %v\n", elapsed2)

	// Verification
	if elapsed2 > 50*time.Millisecond {
		return fmt.Errorf("FAILED: Cache hit was too slow (%v)", elapsed2)
	}

	// 5. Test Persistence Across Connections
	fmt.Println("\n--- Run 3 (New Connection) ---")

	// Create a new database connection to simulate app restart
	db2, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open second mysql connection: %w", err)
	}
	defer db2.Close()

	store2 := docket.NewSQLStore(db2, "step_cache", docket.DialectMySQL)

	g2 := docket.NewGraph()
	g2.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			log.Printf("  [Computing] This should NOT execute (using cache)")
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		docket.WithName("SlowCount"),
		docket.WithPersistence(store2, docket.ScopeGlobal, 1*time.Hour),
	)
	if err := g2.Validate(); err != nil {
		return err
	}

	start = time.Now()
	result3, err := docket.Execute[*pb.LetterCount](ctx, g2, "exec-3", input)
	if err != nil {
		return err
	}
	elapsed3 := time.Since(start)
	fmt.Printf("Result: Count = %d\n", result3.Count)
	fmt.Printf("Duration: %v (should be fast - using persisted cache)\n", elapsed3)

	if elapsed3 > 50*time.Millisecond {
		return fmt.Errorf("FAILED: Cache hit from new connection was too slow (%v)", elapsed3)
	}

	// Cleanup
	if _, err := db.ExecContext(ctx, "DELETE FROM step_cache"); err != nil {
		log.Printf("Warning: cleanup failed: %v", err)
	}

	fmt.Println("\nSUCCESS: MySQL persistence test passed!")
	return nil
}

// createDatabaseIfNotExists ensures the database exists
func createDatabaseIfNotExists(db *sql.DB) error {
	// This assumes root has permission to create databases
	// For production, the database should be created ahead of time
	_, err := db.Exec("CREATE DATABASE IF NOT EXISTS docket_test")
	if err != nil {
		return err
	}
	_, err = db.Exec("USE docket_test")
	return err
}
