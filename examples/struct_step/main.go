// Example: Struct with Compute Method
//
// This example demonstrates registering a struct with a Compute method
// instead of a function. This pattern is useful for:
// - Injecting dependencies (DB clients, API clients)
// - Sharing configuration across executions
// - Testing with mock dependencies
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"docket/pkg/docket"
	pb "docket/proto/examples/lettercount"
)

// ============ Struct-based Steps ============

// LetterCounter is a step that counts occurrences of a configurable letter.
// It demonstrates how to inject configuration at graph setup time.
type LetterCounter struct {
	TargetLetter  string
	CaseSensitive bool
}

// Compute implements the step logic. The method signature defines dependencies.
func (lc *LetterCounter) Compute(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	text := input.Value
	target := lc.TargetLetter

	if !lc.CaseSensitive {
		text = strings.ToLower(text)
		target = strings.ToLower(target)
	}

	count := strings.Count(text, target)

	return &pb.LetterCount{
		OriginalString: input.Value,
		TargetLetter:   lc.TargetLetter,
		Count:          int32(count),
	}, nil
}

// ============ Dependency Injection Example ============

// APIClient simulates an external API client
type APIClient struct {
	BaseURL string
}

func (c *APIClient) Lookup(word string) int {
	// Simulated API call - in reality this would be a network request
	return len(word) * 10 // Fake "score" based on length
}

// WordScorer is a step that uses an injected API client.
type WordScorer struct {
	Client *APIClient
	Prefix string
}

func (ws *WordScorer) Compute(ctx context.Context, input *pb.InputString) (*pb.LetterCount, error) {
	// Use the injected client
	score := ws.Client.Lookup(input.Value)

	return &pb.LetterCount{
		OriginalString: ws.Prefix + input.Value,
		Count:          int32(score),
	}, nil
}

func main() {
	fmt.Println("=== Docket Struct Step Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates registering structs with Compute methods.")
	fmt.Println()

	// ============ EXAMPLE 1: Configurable Step ============
	fmt.Println("📌 Example 1: Configurable Letter Counter")
	runConfigurableStep()

	// ============ EXAMPLE 2: Dependency Injection ============
	fmt.Println("📌 Example 2: Dependency Injection")
	runDependencyInjection()

	// ============ EXAMPLE 3: Multiple Configurations ============
	fmt.Println("📌 Example 3: Reusing Step Types with Different Configs")
	runMultipleConfigs()
}

func runConfigurableStep() {
	g := docket.NewGraph()

	// Create step with specific configuration
	counter := &LetterCounter{
		TargetLetter:  "e",
		CaseSensitive: false,
	}

	err := g.Register(counter, docket.WithName("CountE"))
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	result, err := docket.Execute[*pb.LetterCount](ctx, g, "struct-001", &pb.InputString{Value: "Excellence"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   Input: %q\n", result.OriginalString)
	fmt.Printf("   Target letter: %q (case insensitive: %v)\n", counter.TargetLetter, !counter.CaseSensitive)
	fmt.Printf("   Count: %d\n\n", result.Count)
}

func runDependencyInjection() {
	g := docket.NewGraph()

	// Create a "production" API client
	client := &APIClient{BaseURL: "https://api.example.com"}

	// Inject the client into the step
	scorer := &WordScorer{
		Client: client,
		Prefix: "scored: ",
	}

	err := g.Register(scorer, docket.WithName("ScoreWord"))
	if err != nil {
		log.Fatal(err)
	}

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	result, err := docket.Execute[*pb.LetterCount](ctx, g, "struct-002", &pb.InputString{Value: "docket"})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("   Input: %q\n", "docket")
	fmt.Printf("   Output: %q with score %d\n", result.OriginalString, result.Count)
	fmt.Println("   (In tests, you could inject a mock APIClient)")
	fmt.Println()
}

func runMultipleConfigs() {
	// Create two graphs with different configurations of the same step type

	// Graph 1: Count letter 'a'
	g1 := docket.NewGraph()
	g1.Register(&LetterCounter{TargetLetter: "a", CaseSensitive: true}, docket.WithName("CountA"))
	g1.Validate()

	// Graph 2: Count letter 'o' (case insensitive)
	g2 := docket.NewGraph()
	g2.Register(&LetterCounter{TargetLetter: "o", CaseSensitive: false}, docket.WithName("CountO"))
	g2.Validate()

	ctx := context.Background()
	input := &pb.InputString{Value: "ABRACADABRA"}

	result1, _ := docket.Execute[*pb.LetterCount](ctx, g1, "multi-001", input)
	result2, _ := docket.Execute[*pb.LetterCount](ctx, g2, "multi-002", input)

	fmt.Printf("   Input: %q\n", input.Value)
	fmt.Printf("   Count 'a' (case sensitive): %d\n", result1.Count)
	fmt.Printf("   Count 'o' (case insensitive): %d\n\n", result2.Count)

	fmt.Println("💡 Key Points:")
	fmt.Println("   • Struct steps have a Compute method matching the function signature")
	fmt.Println("   • Struct fields are graph-lifetime dependencies (set once at setup)")
	fmt.Println("   • Great for injecting DB clients, API clients, configuration")
	fmt.Println("   • Makes testing easy - inject mocks at setup time")
}
