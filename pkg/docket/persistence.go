package docket

import (
	"context"

	pb "docket/proto"
)

// PersistenceScope defines how widely a result is shared.
type PersistenceScope int

const (
	// ScopeWorkflow results are visible only within the same execution ID.
	// Useful for checkpointing a specific long-running workflow.
	// Key = execution_id + step_name
	ScopeWorkflow PersistenceScope = iota

	// ScopeGlobal results are shared across all executions.
	// Useful for caching expensive, deterministic steps (e.g. "fetch movie info").
	// Key = step_name + hash(input_proto)
	ScopeGlobal
)

// Store is the interface for persisting step results.
// Implementations (SQLite, Postgres, Redis, Memory) must be thread-safe.
type Store interface {
	// Get retrieves a stored entry.
	// Returns (nil, nil) if key not found or expired.
	Get(ctx context.Context, key string) (*pb.PersistenceEntry, error)

	// Set stores an entry.
	Set(ctx context.Context, key string, entry *pb.PersistenceEntry) error

	// Delete removes an entry.
	Delete(ctx context.Context, key string) error
}

// StoreProvider creates a Store instance.
// Useful if we need to create stores per-request or manage connections.
type StoreProvider interface {
	GetStore(ctx context.Context) (Store, error)
}
