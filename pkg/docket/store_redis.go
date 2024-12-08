package docket

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	pb "docket/proto"
)

// RedisStore implements Store using Redis.
// It is designed to work with github.com/redis/go-redis/v9.
// Redis provides fast, in-memory caching with optional persistence.
type RedisStore struct {
	client *redis.Client
	prefix string // Optional key prefix (e.g., "docket:")
}

// NewRedisStore creates a new Redis-backed store.
// The prefix parameter allows namespacing keys to avoid conflicts.
// If prefix is empty, "docket:" is used by default.
func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	if prefix == "" {
		prefix = "docket:"
	}
	return &RedisStore{
		client: client,
		prefix: prefix,
	}
}

// NewRedisStoreFromURL creates a Redis store from a connection URL.
// Example: "redis://localhost:6379/0" or "redis://:password@localhost:6379/1"
func NewRedisStoreFromURL(url string, prefix string) (*RedisStore, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}
	client := redis.NewClient(opts)
	return NewRedisStore(client, prefix), nil
}

func (s *RedisStore) Get(ctx context.Context, key string) (*pb.PersistenceEntry, error) {
	fullKey := s.prefix + key

	data, err := s.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, nil // Key not found or expired
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	entry := &pb.PersistenceEntry{}
	if err := proto.Unmarshal(data, entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
	}

	// Double-check expiration (Redis TTL should handle this, but be defensive)
	if entry.ExpiresAt != nil && entry.ExpiresAt.AsTime().Before(time.Now()) {
		return nil, nil
	}

	return entry, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, entry *pb.PersistenceEntry) error {
	fullKey := s.prefix + key

	data, err := proto.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Calculate TTL from ExpiresAt
	var ttl time.Duration
	if entry.ExpiresAt != nil {
		ttl = time.Until(entry.ExpiresAt.AsTime())
		if ttl < 0 {
			// Already expired, but set it anyway (will be immediately expired)
			ttl = 1 * time.Millisecond
		}
	}

	// Set with TTL (0 means no expiration)
	return s.client.Set(ctx, fullKey, data, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	fullKey := s.prefix + key
	return s.client.Del(ctx, fullKey).Err()
}

// Ping checks if the Redis connection is alive.
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (s *RedisStore) Close() error {
	return s.client.Close()
}
