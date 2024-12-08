package docket

import (
	"context"
	"sync"
	"time"

	pb "docket/proto"

	"google.golang.org/protobuf/proto"
)

// InMemoryStore is a simple thread-safe map-based store for testing and local dev.
// It respects TTL but loses data on restart.
type InMemoryStore struct {
	mu   sync.RWMutex
	data map[string]*pb.PersistenceEntry
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]*pb.PersistenceEntry),
	}
}

func (s *InMemoryStore) Get(ctx context.Context, key string) (*pb.PersistenceEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok {
		return nil, nil
	}

	// Check expiration
	if entry.ExpiresAt != nil && entry.ExpiresAt.AsTime().Before(time.Now()) {
		return nil, nil
	}

	// Return a copy to prevent race conditions if caller modifies it
	return proto.Clone(entry).(*pb.PersistenceEntry), nil
}

func (s *InMemoryStore) Set(ctx context.Context, key string, entry *pb.PersistenceEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy
	clone := proto.Clone(entry).(*pb.PersistenceEntry)
	s.data[key] = clone
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}
