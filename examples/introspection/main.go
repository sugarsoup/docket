// Example: Graph Introspection
//
// This example demonstrates how to inspect a protograph graph
// to understand its structure, dependencies, and execution order.
package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"protograph/pkg/protograph"
	pb "protograph/proto/examples/parallel"
)

func main() {
	fmt.Println("=== Protograph Graph Introspection ===")
	fmt.Println()
	fmt.Println("This example demonstrates APIs for inspecting graph structure.")
	fmt.Println()

	g := buildExampleGraph()

	// ============ EXAMPLE 1: List All Steps ============
	fmt.Println("📌 Example 1: Listing All Registered Steps")
	listSteps(g)

	// ============ EXAMPLE 2: Find Leaf Types ============
	fmt.Println("📌 Example 2: Finding Leaf Input Types")
	findLeafTypes(g)

	// ============ EXAMPLE 3: Execution Plans ============
	fmt.Println("📌 Example 3: Getting Execution Plans")
	showExecutionPlans(g)

	// ============ EXAMPLE 4: Visualizing the Graph ============
	fmt.Println("📌 Example 4: Graph Visualization")
	visualizeGraph(g)
}

func buildExampleGraph() *protograph.Graph {
	g := protograph.NewGraph()

	// Register several steps with dependencies
	g.Register(
		func(ctx context.Context, userID *pb.UserID) (*pb.UserProfile, error) {
			return &pb.UserProfile{Id: userID.Id, Name: "User"}, nil
		},
		protograph.WithName("FetchProfile"),
	)

	g.Register(
		func(ctx context.Context, userID *pb.UserID) (*pb.UserPreferences, error) {
			return &pb.UserPreferences{UserId: userID.Id}, nil
		},
		protograph.WithName("FetchPreferences"),
	)

	g.Register(
		func(ctx context.Context, userID *pb.UserID) (*pb.UserActivity, error) {
			return &pb.UserActivity{UserId: userID.Id}, nil
		},
		protograph.WithName("FetchActivity"),
	)

	g.Register(
		func(ctx context.Context, profile *pb.UserProfile, prefs *pb.UserPreferences, activity *pb.UserActivity) (*pb.EnrichedUser, error) {
			return &pb.EnrichedUser{Profile: profile, Preferences: prefs, Activity: activity}, nil
		},
		protograph.WithName("EnrichUser"),
	)

	if err := g.Validate(); err != nil {
		log.Fatal(err)
	}

	return g
}

func listSteps(g *protograph.Graph) {
	steps := g.Steps()
	fmt.Printf("   Found %d registered steps:\n\n", len(steps))

	for _, step := range steps {
		aggregate := ""
		if step.IsAggregate {
			aggregate = " [AGGREGATE]"
		}
		fmt.Printf("   📦 %s%s\n", step.Name, aggregate)
		fmt.Printf("      Output: %s\n", formatType(step.OutputType))
		if len(step.Dependencies) > 0 {
			fmt.Printf("      Dependencies:\n")
			for _, dep := range step.Dependencies {
				fmt.Printf("         • %s\n", formatType(dep))
			}
		} else {
			fmt.Printf("      Dependencies: (none - only needs leaf inputs)\n")
		}
		fmt.Println()
	}
}

func findLeafTypes(g *protograph.Graph) {
	leaves := g.LeafTypes()
	fmt.Printf("   Leaf input types (must be provided at execution):\n")
	for _, leaf := range leaves {
		fmt.Printf("      • %s\n", formatType(leaf))
	}
	fmt.Println()
	fmt.Println("   These types are not produced by any step - they must be")
	fmt.Println("   provided as inputs when calling Execute().")
	fmt.Println()
}

func showExecutionPlans(g *protograph.Graph) {
	// Plan for EnrichedUser
	plan1, _ := g.GetExecutionPlan(&pb.EnrichedUser{})
	fmt.Println("   Execution plan for EnrichedUser:")
	for i, step := range plan1 {
		fmt.Printf("      %d. %s\n", i+1, step)
	}
	fmt.Println()

	// Plan for just UserProfile (simpler)
	plan2, _ := g.GetExecutionPlan(&pb.UserProfile{})
	fmt.Println("   Execution plan for UserProfile (simpler subgraph):")
	for i, step := range plan2 {
		fmt.Printf("      %d. %s\n", i+1, step)
	}
	fmt.Println()
}

func visualizeGraph(g *protograph.Graph) {
	fmt.Println("   ASCII visualization of the dependency graph:")
	fmt.Println()
	fmt.Println("   ┌──────────┐")
	fmt.Println("   │  UserID  │  ← Leaf input")
	fmt.Println("   └────┬─────┘")
	fmt.Println("        │")
	fmt.Println("        ├──────────────────┬──────────────────┐")
	fmt.Println("        │                  │                  │")
	fmt.Println("        ▼                  ▼                  ▼")
	fmt.Println("   ┌──────────┐      ┌───────────┐      ┌──────────┐")
	fmt.Println("   │  Profile │      │   Prefs   │      │ Activity │")
	fmt.Println("   └────┬─────┘      └─────┬─────┘      └────┬─────┘")
	fmt.Println("        │                  │                  │")
	fmt.Println("        └──────────────────┼──────────────────┘")
	fmt.Println("                           │")
	fmt.Println("                           ▼")
	fmt.Println("                    ┌─────────────┐")
	fmt.Println("                    │ EnrichedUser│  ← Final output")
	fmt.Println("                    └─────────────┘")
	fmt.Println()

	fmt.Println("💡 Key Points:")
	fmt.Println("   • Steps() returns all registered steps with their metadata")
	fmt.Println("   • LeafTypes() identifies required inputs (not produced by steps)")
	fmt.Println("   • GetExecutionPlan() shows step order for a given output type")
	fmt.Println("   • Use these APIs for debugging, documentation, or visualization")
}

// formatType extracts a readable name from a reflect.Type
func formatType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	}
	return t.Name()
}
