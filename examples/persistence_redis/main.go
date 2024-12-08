package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// 1. Setup Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer rdb.Close()

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w (is Redis running?)", err)
	}
	fmt.Println("Connected to Redis successfully")

	// Create RedisStore with custom prefix
	store := docket.NewRedisStore(rdb, "docket:example:")

	// Clean up any existing keys from previous runs
	if err := cleanupKeys(ctx, rdb, "docket:example:*"); err != nil {
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

	input := &pb.InputString{Value: "hello redis"}

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

	// Verify key exists in Redis
	keys, err := rdb.Keys(ctx, "docket:example:*").Result()
	if err != nil {
		return fmt.Errorf("failed to query redis keys: %w", err)
	}
	if len(keys) == 0 {
		return fmt.Errorf("verification failed: expected keys in redis, got 0")
	}
	fmt.Printf("Verified: Found %d key(s) in Redis\n", len(keys))

	// 4. Second Run (Warm - should hit cache)
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

	// 5. Test TTL expiration (optional - using shorter TTL)
	fmt.Println("\n--- Run 3 (TTL Test) ---")
	g2 := docket.NewGraph()
	g2.Register(
		func(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
			log.Printf("  [Computing with short TTL] Count for %q", input.Value)
			return &pb.LetterCount{Count: int32(len(input.Value))}, nil
		},
		docket.WithName("ShortTTLCount"),
		docket.WithPersistence(store, docket.ScopeGlobal, 100*time.Millisecond),
	)
	if err := g2.Validate(); err != nil {
		return err
	}

	input2 := &pb.InputString{Value: "short ttl test"}

	// First call
	_, err = docket.Execute[*pb.LetterCount](ctx, g2, "exec-ttl-1", input2)
	if err != nil {
		return err
	}
	fmt.Println("First call with short TTL done")

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Second call - should recompute
	fmt.Println("Waiting for TTL to expire... (150ms)")
	log.Printf("Second call after TTL expiration:")
	_, err = docket.Execute[*pb.LetterCount](ctx, g2, "exec-ttl-2", input2)
	if err != nil {
		return err
	}

	// Cleanup
	if err := cleanupKeys(ctx, rdb, "docket:example:*"); err != nil {
		log.Printf("Warning: cleanup failed: %v", err)
	}

	fmt.Println("\nSUCCESS: Redis cache test passed!")
	return nil
}

func cleanupKeys(ctx context.Context, rdb *redis.Client, pattern string) error {
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return rdb.Del(ctx, keys...).Err()
	}
	return nil
}
