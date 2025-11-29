package protograph

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	pb "protograph/proto/examples/lettercount"
)

// ============ ExecutionError Tests ============

func TestExecutionError_Format_WithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := &ExecutionError{
		ExecutionID: "exec-123",
		StepName:    "MyStep",
		StepType:    reflect.TypeOf(&pb.LetterCount{}),
		Cause:       cause,
		Message:     "step failed",
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "exec-123") {
		t.Errorf("error should contain execution ID: %s", errStr)
	}
	if !strings.Contains(errStr, "MyStep") {
		t.Errorf("error should contain step name: %s", errStr)
	}
	if !strings.Contains(errStr, "underlying error") {
		t.Errorf("error should contain cause: %s", errStr)
	}
}

func TestExecutionError_Format_WithoutCause(t *testing.T) {
	err := &ExecutionError{
		ExecutionID: "exec-123",
		StepName:    "MyStep",
		Message:     "step failed",
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "exec-123") {
		t.Errorf("error should contain execution ID: %s", errStr)
	}
	if !strings.Contains(errStr, "MyStep") {
		t.Errorf("error should contain step name: %s", errStr)
	}
}

func TestExecutionError_Format_NoStepName(t *testing.T) {
	err := &ExecutionError{
		ExecutionID: "exec-123",
		StepType:    reflect.TypeOf(&pb.LetterCount{}),
		Message:     "step failed",
	}

	errStr := err.Error()

	// Should fall back to step type
	if !strings.Contains(errStr, "LetterCount") {
		t.Errorf("error should contain step type when no name: %s", errStr)
	}
}

func TestExecutionError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &ExecutionError{
		Cause: cause,
	}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should match cause through Unwrap")
	}

	var unwrapped error = err.Unwrap()
	if unwrapped != cause {
		t.Error("Unwrap should return cause")
	}
}

func TestExecutionError_Unwrap_NilCause(t *testing.T) {
	err := &ExecutionError{
		Message: "no cause",
	}

	if err.Unwrap() != nil {
		t.Error("Unwrap should return nil when cause is nil")
	}
}

// ============ RegistrationError Tests ============

func TestRegistrationError_Format_WithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := &RegistrationError{
		StepName:   "MyStep",
		OutputType: reflect.TypeOf(&pb.LetterCount{}),
		Cause:      cause,
		Message:    "invalid signature",
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "LetterCount") {
		t.Errorf("error should contain output type: %s", errStr)
	}
	if !strings.Contains(errStr, "invalid signature") {
		t.Errorf("error should contain message: %s", errStr)
	}
	if !strings.Contains(errStr, "underlying error") {
		t.Errorf("error should contain cause: %s", errStr)
	}
}

func TestRegistrationError_Format_WithoutCause(t *testing.T) {
	err := &RegistrationError{
		OutputType: reflect.TypeOf(&pb.LetterCount{}),
		Message:    "invalid signature",
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "invalid signature") {
		t.Errorf("error should contain message: %s", errStr)
	}
}

func TestRegistrationError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &RegistrationError{
		Cause: cause,
	}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should match cause through Unwrap")
	}
}

// ============ Sentinel Errors ============

func TestSentinelErrors_Defined(t *testing.T) {
	// Verify sentinel errors are properly defined and distinct
	sentinels := []error{
		ErrNotValidated,
		ErrNoStepForType,
		ErrCycleDetected,
		ErrDuplicateOutput,
	}

	for i, err := range sentinels {
		if err == nil {
			t.Errorf("sentinel error %d is nil", i)
		}
		// Ensure each is unique
		for j, other := range sentinels {
			if i != j && errors.Is(err, other) {
				t.Errorf("sentinel errors %d and %d should be distinct", i, j)
			}
		}
	}
}

func TestSentinelErrors_Messages(t *testing.T) {
	tests := []struct {
		err      error
		contains string
	}{
		{ErrNotValidated, "validated"},
		{ErrNoStepForType, "step"},
		{ErrCycleDetected, "cycle"},
		{ErrDuplicateOutput, "output"},
	}

	for _, tt := range tests {
		if !strings.Contains(tt.err.Error(), tt.contains) {
			t.Errorf("error %q should contain %q", tt.err.Error(), tt.contains)
		}
	}
}

// ============ Error Wrapping Chain ============

func TestErrorWrapping_Chain(t *testing.T) {
	root := errors.New("root cause")

	execErr := &ExecutionError{
		ExecutionID: "exec-1",
		Cause:       root,
		Message:     "execution failed",
	}

	// Should be able to unwrap to root
	if !errors.Is(execErr, root) {
		t.Error("should be able to match root cause through chain")
	}

	// Type assertion should work
	var ee *ExecutionError
	if !errors.As(execErr, &ee) {
		t.Error("errors.As should work for ExecutionError")
	}
}
