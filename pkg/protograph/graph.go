package protograph

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

// GraphOption is a functional option for configuring the Graph.
type GraphOption interface {
	apply(*Graph)
}

type graphOptionFunc func(*Graph)

func (f graphOptionFunc) apply(g *Graph) {
	f(g)
}

// WithDefaultTimeout sets a default timeout for graph executions.
// This timeout is applied if the execution context doesn't have a tighter deadline.
func WithDefaultTimeout(d time.Duration) GraphOption {
	return graphOptionFunc(func(g *Graph) {
		g.defaultTimeout = d
	})
}

// Graph is a directed acyclic graph of proto transformations.
// Each node is identified by a proto output type, and edges represent dependencies.
// Steps are registered as functions or structs with Compute methods.
//
// Thread Safety: Graph is NOT safe for concurrent modification during setup.
// All Register calls should complete before calling Validate or Execute.
// After validation, Graph is safe for concurrent Execute calls.
type Graph struct {
	// steps maps output type to step metadata
	steps map[reflect.Type]*stepMeta

	// validated tracks whether Validate() has been called successfully
	validated bool

	// defaultTimeout is the default deadline for executions
	defaultTimeout time.Duration
}

// stepMeta holds registration-time information about a step.
type stepMeta struct {
	// name is a human-readable identifier for debugging/logging
	name string

	// stepValue is the reflected value of the step (function or struct pointer)
	stepValue reflect.Value

	// isMethod indicates stepValue is a bound method (from a struct with Compute)
	isMethod bool

	// dependencies are the proto types this step requires as input
	dependencies []reflect.Type

	// isAggregate indicates this step processes batches of items at once (fan-in)
	// Aggregate steps receive []proto.Message for each dependency and produce
	// a single result that is shared across all items in the batch
	isAggregate bool

	// config holds retry, timeout, and other configuration
	config *StepConfig
}

// NewGraph creates a new empty graph with optional configuration.
func NewGraph(opts ...GraphOption) *Graph {
	g := &Graph{
		steps: make(map[reflect.Type]*stepMeta),
	}
	for _, opt := range opts {
		opt.apply(g)
	}
	return g
}

// Register adds a step to the graph.
//
// The step can be:
//   - A function: func(context.Context, ...protos) (proto, error)
//   - A struct pointer with a Compute method: (*T).Compute(context.Context, ...protos) (proto, error)
//
// The output type and dependencies are inferred from the function/method signature.
// The output type (first return value) becomes the graph node.
// Parameters after context.Context are the dependencies (edges to other nodes).
//
// Example:
//
//	g.Register(func(ctx context.Context, movie *pb.Movie) (*pb.DirectorProfile, error) {
//	    return fetchDirector(ctx, movie.DirectorId)
//	})
func (g *Graph) Register(step any, opts ...StepOption) error {
	meta, outputType, err := inspectStep(step)
	if err != nil {
		return fmt.Errorf("invalid step: %w", err)
	}
	return g.registerStep(meta, outputType, opts)
}

// RegisterAggregate adds an aggregate step that processes batches of items at once (fan-in).
//
// Unlike regular steps that process one item, aggregate steps receive slices
// of all items for each dependency type and produce a single result that is
// shared across all items. This implements the fan-in pattern where:
//   - Multiple items are processed together (fan-in)
//   - A single aggregate result is computed
//   - That result is available to downstream per-item steps
//
// The step signature should be:
//
//	func(ctx context.Context, items []*pb.InputType) (*pb.AggregateResult, error)
//
// Example:
//
//	g.RegisterAggregate(func(ctx context.Context, movies []*pb.Movie) (*pb.BatchStats, error) {
//	    var totalRuntime int32
//	    for _, movie := range movies {
//	        totalRuntime += movie.RuntimeMinutes
//	    }
//	    return &pb.BatchStats{
//	        Count:          int32(len(movies)),
//	        TotalRuntime:   totalRuntime,
//	        AverageRuntime: float64(totalRuntime) / float64(len(movies)),
//	    }, nil
//	})
func (g *Graph) RegisterAggregate(step any, opts ...StepOption) error {
	meta, outputType, err := inspectAggregateStep(step)
	if err != nil {
		return fmt.Errorf("invalid aggregate step: %w", err)
	}
	meta.isAggregate = true
	return g.registerStep(meta, outputType, opts)
}

// registerStep is the common implementation for Register and RegisterAggregate.
func (g *Graph) registerStep(meta *stepMeta, outputType reflect.Type, opts []StepOption) error {
	if g.validated {
		return fmt.Errorf("cannot register steps after validation")
	}

	// Check for duplicate output type
	if existing, exists := g.steps[outputType]; exists {
		return fmt.Errorf("output type %v already registered by step %q", outputType, existing.name)
	}

	// Apply options
	meta.config = &StepConfig{}
	for _, opt := range opts {
		opt.apply(meta.config)
	}

	// Use name from options if provided
	if meta.config.Name != "" {
		meta.name = meta.config.Name
	}

	g.steps[outputType] = meta
	return nil
}

// ============ Step Inspection ============

// Common interface types used for validation
var (
	ctxType      = reflect.TypeOf((*context.Context)(nil)).Elem()
	protoMsgType = reflect.TypeOf((*proto.Message)(nil)).Elem()
	errorType    = reflect.TypeOf((*error)(nil)).Elem()
)

// inspectStep extracts metadata from a function or struct with Compute method.
func inspectStep(step any) (*stepMeta, reflect.Type, error) {
	if step == nil {
		return nil, nil, fmt.Errorf("step cannot be nil")
	}

	v := reflect.ValueOf(step)
	t := v.Type()

	switch t.Kind() {
	case reflect.Func:
		return inspectFunc(v, t)
	case reflect.Ptr:
		return inspectStructWithCompute(v, t)
	default:
		return nil, nil, fmt.Errorf("step must be a function or pointer to struct with Compute method, got %v", t.Kind())
	}
}

// inspectStructWithCompute extracts metadata from a struct pointer with a Compute method.
func inspectStructWithCompute(v reflect.Value, t reflect.Type) (*stepMeta, reflect.Type, error) {
	method := v.MethodByName("Compute")
	if !method.IsValid() {
		return nil, nil, fmt.Errorf("struct %v has no Compute method", t)
	}

	outputType, deps, err := validateFuncSignature(method.Type(), false)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid Compute method: %w", err)
	}

	// Use struct type name as default
	name := t.Elem().Name()

	return &stepMeta{
		name:         name,
		stepValue:    method,
		isMethod:     true,
		dependencies: deps,
	}, outputType, nil
}

// inspectFunc extracts metadata from a function value.
func inspectFunc(fn reflect.Value, fnType reflect.Type) (*stepMeta, reflect.Type, error) {
	outputType, deps, err := validateFuncSignature(fnType, false)
	if err != nil {
		return nil, nil, err
	}

	name := extractFuncName(fn)

	return &stepMeta{
		name:         name,
		stepValue:    fn,
		dependencies: deps,
	}, outputType, nil
}

// inspectAggregateStep extracts metadata from an aggregate step function.
func inspectAggregateStep(step any) (*stepMeta, reflect.Type, error) {
	v := reflect.ValueOf(step)
	t := v.Type()

	if t.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("aggregate step must be a function, got %v", t.Kind())
	}

	outputType, deps, err := validateFuncSignature(t, true)
	if err != nil {
		return nil, nil, err
	}

	name := extractFuncName(v)

	return &stepMeta{
		name:         name,
		stepValue:    v,
		dependencies: deps,
	}, outputType, nil
}

// validateFuncSignature validates a function signature and extracts output type and dependencies.
// If isAggregate is true, parameters must be slices of proto.Message.
func validateFuncSignature(fnType reflect.Type, isAggregate bool) (outputType reflect.Type, deps []reflect.Type, err error) {
	// Must have at least context.Context parameter
	if fnType.NumIn() < 1 {
		return nil, nil, fmt.Errorf("function must have at least context.Context parameter")
	}

	// First param must be context.Context
	if !fnType.In(0).Implements(ctxType) {
		return nil, nil, fmt.Errorf("first parameter must be context.Context, got %v", fnType.In(0))
	}

	// Must return exactly 2 values
	if fnType.NumOut() != 2 {
		return nil, nil, fmt.Errorf("function must return (proto.Message, error), got %d return values", fnType.NumOut())
	}

	// First return must be proto.Message
	outputType = fnType.Out(0)
	if !outputType.Implements(protoMsgType) {
		return nil, nil, fmt.Errorf("first return value must be proto.Message, got %v", outputType)
	}

	// Second return must be error
	if !fnType.Out(1).Implements(errorType) {
		return nil, nil, fmt.Errorf("second return value must be error, got %v", fnType.Out(1))
	}

	// Extract dependencies (all params after context.Context)
	for i := 1; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		if isAggregate {
			// Aggregate steps take slices
			if paramType.Kind() != reflect.Slice {
				return nil, nil, fmt.Errorf("aggregate step parameter %d must be a slice, got %v", i, paramType)
			}
			elemType := paramType.Elem()
			if !elemType.Implements(protoMsgType) {
				return nil, nil, fmt.Errorf("aggregate step parameter %d must be []proto.Message, got %v", i, paramType)
			}
			deps = append(deps, elemType)
		} else {
			// Regular steps take proto.Message directly
			if !paramType.Implements(protoMsgType) {
				return nil, nil, fmt.Errorf("parameter %d must be proto.Message, got %v", i, paramType)
			}
			deps = append(deps, paramType)
		}
	}

	return outputType, deps, nil
}

// extractFuncName attempts to get a readable name for a function.
func extractFuncName(fn reflect.Value) string {
	if fn.Kind() != reflect.Func {
		return ""
	}

	ptr := fn.Pointer()
	if ptr == 0 {
		return ""
	}

	pc := runtime.FuncForPC(ptr)
	if pc == nil {
		return ""
	}

	fullName := pc.Name()
	// Extract just the function name from the full path
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		return fullName[idx+1:]
	}
	return fullName
}

// ============ Validation ============

// Validate checks the graph for errors:
//   - At least one step must be registered
//   - No cycles in the dependency graph
//   - All dependencies are resolvable (registered or will be provided as inputs)
//
// Must be called before Execute.
func (g *Graph) Validate() error {
	if len(g.steps) == 0 {
		return fmt.Errorf("graph has no registered steps")
	}

	if err := g.detectCycles(); err != nil {
		return err
	}

	if err := g.validateDependencies(); err != nil {
		return err
	}

	g.validated = true
	return nil
}

// validateDependencies checks that dependencies form a valid DAG structure.
// All dependencies are valid - they're either:
// 1. The output of another registered step, OR
// 2. A "leaf" type (must be provided at execution time)
//
// This method currently performs structural validation. Future versions
// may add warnings for suspicious patterns.
func (g *Graph) validateDependencies() error {
	// Currently all dependency patterns are valid.
	// Leaf types (dependencies not produced by any step) are provided at execution time.
	// Execution will fail if a required leaf input is missing.
	return nil
}

// detectCycles uses DFS to find cycles in the dependency graph.
func (g *Graph) detectCycles() error {
	visited := make(map[reflect.Type]bool)
	recursionStack := make(map[reflect.Type]bool)

	for outputType := range g.steps {
		if err := g.dfsCheckCycle(outputType, visited, recursionStack, nil); err != nil {
			return err
		}
	}
	return nil
}

func (g *Graph) dfsCheckCycle(
	current reflect.Type,
	visited, recursionStack map[reflect.Type]bool,
	path []reflect.Type,
) error {
	if recursionStack[current] {
		cyclePath := append(path, current)
		return fmt.Errorf("cycle detected: %s", formatTypePath(cyclePath))
	}

	if visited[current] {
		return nil
	}

	visited[current] = true
	recursionStack[current] = true
	path = append(path, current)

	if meta := g.steps[current]; meta != nil {
		for _, depType := range meta.dependencies {
			if err := g.dfsCheckCycle(depType, visited, recursionStack, path); err != nil {
				return err
			}
		}
	}

	recursionStack[current] = false
	return nil
}

func formatTypePath(types []reflect.Type) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.String()
	}
	return strings.Join(names, " -> ")
}

// ============ Introspection ============

// StepInfo provides read-only information about a registered step.
type StepInfo struct {
	// Name is the human-readable step name
	Name string

	// OutputType is the proto type this step produces
	OutputType reflect.Type

	// Dependencies are the proto types this step requires
	Dependencies []reflect.Type

	// IsAggregate indicates this is a fan-in step that processes batches
	IsAggregate bool
}

// Steps returns information about all registered steps.
// The returned slice is in no particular order.
func (g *Graph) Steps() []StepInfo {
	result := make([]StepInfo, 0, len(g.steps))
	for outputType, meta := range g.steps {
		// Copy dependencies to avoid exposing internal slice
		deps := make([]reflect.Type, len(meta.dependencies))
		copy(deps, meta.dependencies)

		result = append(result, StepInfo{
			Name:         meta.name,
			OutputType:   outputType,
			Dependencies: deps,
			IsAggregate:  meta.isAggregate,
		})
	}
	return result
}

// LeafTypes returns all types that are dependencies but not outputs of any step.
// These must be provided as inputs at execution time.
func (g *Graph) LeafTypes() []reflect.Type {
	outputs := make(map[reflect.Type]bool)
	deps := make(map[reflect.Type]bool)

	for outputType, meta := range g.steps {
		outputs[outputType] = true
		for _, depType := range meta.dependencies {
			deps[depType] = true
		}
	}

	var leaves []reflect.Type
	for depType := range deps {
		if !outputs[depType] {
			leaves = append(leaves, depType)
		}
	}
	return leaves
}

// GetExecutionPlan returns the ordered list of step names needed to compute outputType.
// Useful for debugging and visualization.
func (g *Graph) GetExecutionPlan(outputType proto.Message) ([]string, error) {
	if !g.validated {
		return nil, fmt.Errorf("graph must be validated before getting execution plan")
	}

	targetType := reflect.TypeOf(outputType)
	var plan []string
	visited := make(map[reflect.Type]bool)

	if err := g.buildPlan(targetType, &plan, visited); err != nil {
		return nil, err
	}

	return plan, nil
}

func (g *Graph) buildPlan(targetType reflect.Type, plan *[]string, visited map[reflect.Type]bool) error {
	if visited[targetType] {
		return nil
	}
	visited[targetType] = true

	meta := g.steps[targetType]
	if meta == nil {
		// This is a leaf input, not an error
		return nil
	}

	// Process dependencies first
	for _, depType := range meta.dependencies {
		if err := g.buildPlan(depType, plan, visited); err != nil {
			return err
		}
	}

	// Add this step
	name := meta.name
	if name == "" {
		name = targetType.String()
	}
	*plan = append(*plan, name)
	return nil
}
