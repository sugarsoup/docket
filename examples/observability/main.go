package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	fmt.Println("=== Docket Observability Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates multiple observability patterns:")
	fmt.Println("1. Prometheus metrics")
	fmt.Println("2. Structured logging (slog)")
	fmt.Println("3. Multiple observers combined")
	fmt.Println()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Setup Prometheus
	registry := prometheus.NewRegistry()
	prometheusObserver := docket.NewPrometheusObserver("example", registry)

	// Setup slog observer
	slogObserver := docket.NewSlogObserver(logger, slog.LevelDebug)

	// Combine multiple observers
	multiObserver := &docket.MultiObserver{
		Observers: []docket.Observer{
			prometheusObserver,
			slogObserver,
		},
	}

	// Create graph with observability
	g := docket.NewGraph(
		docket.WithObserver(multiObserver),
		docket.WithDefaultTimeout(10*time.Second),
	)

	// Register a step that simulates work
	g.Register(
		SlowCountWithRetry,
		docket.WithName("SlowCount"),
		docket.WithRetry(docket.RetryConfig{
			MaxAttempts: 3,
			Backoff: docket.ExponentialBackoff{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Factor:       2.0,
			},
		}),
		docket.WithTimeout(5*time.Second),
		docket.WithPersistence(
			docket.NewInMemoryStore(),
			docket.ScopeGlobal,
			5*time.Minute,
		),
	)

	if err := g.Validate(); err != nil {
		return err
	}

	// Start Prometheus HTTP server
	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		fmt.Println("\n📊 Prometheus metrics available at: http://localhost:2112/metrics")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()

	// Give the HTTP server a moment to start
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n--- Execution 1: First run (will be slow) ---")
	ctx := context.Background()
	result1, err := docket.Execute[*pb.LetterCount](ctx, g, "exec-1", &pb.InputString{Value: "hello"})
	if err != nil {
		return err
	}
	fmt.Printf("Result: %d letters\n", result1.Count)

	fmt.Println("\n--- Execution 2: Cached (will be fast) ---")
	result2, err := docket.Execute[*pb.LetterCount](ctx, g, "exec-2", &pb.InputString{Value: "hello"})
	if err != nil {
		return err
	}
	fmt.Printf("Result: %d letters\n", result2.Count)

	fmt.Println("\n--- Execution 3: With retry (might fail and retry) ---")
	result3, err := docket.Execute[*pb.LetterCount](ctx, g, "exec-3", &pb.InputString{Value: "world"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %d letters\n", result3.Count)
	}

	fmt.Println("\n✅ Check the logs above for structured logging output")
	fmt.Println("📊 Visit http://localhost:2112/metrics to see Prometheus metrics")
	fmt.Println("\nPress Ctrl+C to exit...")

	// Keep running so you can check metrics
	select {}
}

// SlowCountWithRetry simulates a step that sometimes fails and needs retry.
func SlowCountWithRetry(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	// Simulate work
	time.Sleep(200 * time.Millisecond)

	// Randomly fail 30% of the time (to demonstrate retries)
	if rand.Float32() < 0.3 {
		return nil, fmt.Errorf("random failure for demonstration")
	}

	return &pb.LetterCount{
		Count: int32(len(input.Value)),
	}, nil
}
