// Example: Parallel Dependency Resolution
//
// This example demonstrates how protograph automatically resolves
// independent dependencies in parallel, reducing total execution time.
package main

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/parallel"
)

// Track concurrent execution for demonstration
var concurrentCalls atomic.Int32
var maxConcurrent atomic.Int32

func trackConcurrency(name string) func() {
	current := concurrentCalls.Add(1)
	for {
		old := maxConcurrent.Load()
		if current <= old || maxConcurrent.CompareAndSwap(old, current) {
			break
		}
	}
	fmt.Printf("   → %s started (concurrent: %d)\n", name, current)
	return func() {
		concurrentCalls.Add(-1)
		fmt.Printf("   ← %s finished\n", name)
	}
}

func main() {
	fmt.Println("=== Protograph Parallel Dependency Resolution ===")
	fmt.Println()
	fmt.Println("This example demonstrates parallel execution of independent steps.")
	fmt.Println()

	runParallelExample()
}

func runParallelExample() {
	g := protograph.NewGraph()

	// Step 1: Fetch user profile (100ms latency)
	g.Register(
		func(ctx context.Context, userID *pb.UserID) (*pb.UserProfile, error) {
			done := trackConcurrency("FetchProfile")
			defer done()

			time.Sleep(100 * time.Millisecond)
			return &pb.UserProfile{
				Id:    userID.Id,
				Name:  "Alice",
				Email: "alice@example.com",
			}, nil
		},
		protograph.WithName("FetchProfile"),
	)

	// Step 2: Fetch user preferences (100ms latency) - INDEPENDENT of profile
	g.Register(
		func(ctx context.Context, userID *pb.UserID) (*pb.UserPreferences, error) {
			done := trackConcurrency("FetchPreferences")
			defer done()

			time.Sleep(100 * time.Millisecond)
			return &pb.UserPreferences{
				UserId:   userID.Id,
				Theme:    "dark",
				Language: "en",
			}, nil
		},
		protograph.WithName("FetchPreferences"),
	)

	// Step 3: Fetch user activity (100ms latency) - INDEPENDENT of profile and preferences
	g.Register(
		func(ctx context.Context, userID *pb.UserID) (*pb.UserActivity, error) {
			done := trackConcurrency("FetchActivity")
			defer done()

			time.Sleep(100 * time.Millisecond)
			return &pb.UserActivity{
				UserId:             userID.Id,
				LoginCount:         42,
				LastLoginTimestamp: time.Now().Unix(),
			}, nil
		},
		protograph.WithName("FetchActivity"),
	)

	// Step 4: Combine all data (depends on all three above)
	g.Register(
		func(ctx context.Context, profile *pb.UserProfile, prefs *pb.UserPreferences, activity *pb.UserActivity) (*pb.EnrichedUser, error) {
			done := trackConcurrency("EnrichUser")
			defer done()

			return &pb.EnrichedUser{
				Profile:     profile,
				Preferences: prefs,
				Activity:    activity,
			}, nil
		},
		protograph.WithName("EnrichUser"),
	)

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	// Show the execution plan
	plan, _ := g.GetExecutionPlan(&pb.EnrichedUser{})
	fmt.Println("📋 Execution Plan:")
	for i, step := range plan {
		fmt.Printf("   %d. %s\n", i+1, step)
	}
	fmt.Println()

	// Execute and measure time
	fmt.Println("⚙️  Executing (watch for parallel execution):")
	ctx := context.Background()
	start := time.Now()

	result, err := protograph.Execute[*pb.EnrichedUser](ctx, g, "parallel-001", &pb.UserID{Id: "user-123"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Println()

	// Display results
	fmt.Println("📊 Results:")
	fmt.Printf("   Profile: %s <%s>\n", result.Profile.Name, result.Profile.Email)
	fmt.Printf("   Theme: %s, Language: %s\n", result.Preferences.Theme, result.Preferences.Language)
	fmt.Printf("   Logins: %d\n", result.Activity.LoginCount)
	fmt.Println()

	// Show timing analysis
	fmt.Println("⏱️  Performance Analysis:")
	fmt.Printf("   Total time: %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("   Max concurrent: %d\n", maxConcurrent.Load())
	fmt.Println()

	sequentialTime := 400 * time.Millisecond // 4 steps × 100ms each
	fmt.Printf("   If sequential: ~%v\n", sequentialTime)
	fmt.Printf("   Actual (parallel): ~%v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("   Speedup: %.1fx\n", float64(sequentialTime)/float64(elapsed))
	fmt.Println()

	fmt.Println("💡 Key Points:")
	fmt.Println("   • FetchProfile, FetchPreferences, and FetchActivity are independent")
	fmt.Println("   • They run in PARALLEL (all start simultaneously)")
	fmt.Println("   • EnrichUser waits for all three, then runs")
	fmt.Println("   • Total time ≈ 100ms (parallel) vs 400ms (sequential)")
	fmt.Println("   • No code changes needed - parallelism is automatic!")
}
