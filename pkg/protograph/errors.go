package protograph

import (
	"errors"
	"fmt"
	"reflect"
)

// ExecutionError represents an error that occurred during graph execution.
type ExecutionError struct {
	// ExecutionID is the unique identifier for this execution
	ExecutionID string

	// StepName is the name of the step that failed (if available)
	StepName string

	// StepType is the output type of the step that failed
	StepType reflect.Type

	// Cause is the underlying error
	Cause error

	// Message provides additional context
	Message string
}

func (e *ExecutionError) Error() string {
	stepID := e.StepName
	if stepID == "" && e.StepType != nil {
		stepID = e.StepType.String()
	}

	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s (%s): %v", e.ExecutionID, e.Message, stepID, e.Cause)
	}
	return fmt.Sprintf("[%s] %s (%s)", e.ExecutionID, e.Message, stepID)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// StepPanicError represents a panic that occurred within a step execution.
type StepPanicError struct {
	*ExecutionError
	PanicValue any
	Stack      []byte
}

func (e *StepPanicError) Error() string {
	return fmt.Sprintf("panic in step %s: %v", e.StepName, e.PanicValue)
}

func (e *StepPanicError) Unwrap() error {
	return e.ExecutionError
}

// DependencyError represents a failure to resolve a dependency.
type DependencyError struct {
	*ExecutionError
	DependencyType reflect.Type
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency failed for step %s: required %v: %v", e.StepName, e.DependencyType, e.Cause)
}

func (e *DependencyError) Unwrap() error {
	return e.ExecutionError
}

// Common sentinel errors
var (
	// ErrNotValidated is returned when Execute is called on an unvalidated graph
	ErrNotValidated = errors.New("graph must be validated before execution")

	// ErrNoStepForType is returned when no step is registered for a required type
	ErrNoStepForType = errors.New("no step registered for type")

	// ErrCycleDetected is returned when the graph contains a dependency cycle
	ErrCycleDetected = errors.New("cycle detected in dependency graph")

	// ErrDuplicateOutput is returned when multiple steps produce the same output type
	ErrDuplicateOutput = errors.New("output type already registered")
)

// RegistrationError represents an error during step registration.
type RegistrationError struct {
	// StepName is the name of the step (if available)
	StepName string

	// OutputType is the output type being registered
	OutputType reflect.Type

	// Cause is the underlying error
	Cause error

	// Message provides additional context
	Message string
}

func (e *RegistrationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("registration error for %v: %s: %v", e.OutputType, e.Message, e.Cause)
	}
	return fmt.Sprintf("registration error for %v: %s", e.OutputType, e.Message)
}

func (e *RegistrationError) Unwrap() error {
	return e.Cause
}
