# Redis Persistence Example

This example demonstrates using **Redis** as a persistence backend for Docket's caching layer.

## What This Demonstrates

- Using `RedisStore` to cache step results in Redis
- Fast, in-memory caching with automatic TTL expiration
- Cross-execution caching with `ScopeGlobal`
- Verifying cache hits and TTL behavior

## Prerequisites

**Redis must be running locally:**

```bash
# Using Docker
docker run -d -p 6379:6379 redis:7

# Or using Homebrew (macOS)
brew install redis
brew services start redis

# Or using apt (Linux)
sudo apt install redis-server
sudo systemctl start redis
```

Verify Redis is running:
```bash
redis-cli ping
# Should return: PONG
```

## Running the Example

```bash
go run examples/persistence_redis/main.go
```

## Expected Output

```
Connected to Redis successfully

--- Run 1 (Cold) ---
  [Computing] Count for "hello redis" (sleeping 200ms)...
Result: Count = 11
Duration: 200ms
Verified: Found 1 key(s) in Redis

--- Run 2 (Warm) ---
Result: Count = 11
Duration: 2ms

--- Run 3 (TTL Test) ---
  [Computing with short TTL] Count for "short ttl test"
First call with short TTL done
Waiting for TTL to expire... (150ms)
  [Computing with short TTL] Count for "short ttl test"

SUCCESS: Redis cache test passed!
```

## Key Features

### RedisStore

```go
// Create a Redis client
rdb := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

// Create RedisStore with optional key prefix
store := docket.NewRedisStore(rdb, "docket:example:")

// Or create from URL
store, err := docket.NewRedisStoreFromURL("redis://localhost:6379/0", "myapp:")
```

### Key Prefixing

Redis stores all keys in a global namespace. Use prefixes to:
- Avoid key collisions between applications
- Organize keys by environment (e.g., `prod:docket:`, `staging:docket:`)
- Easily delete all keys for a specific app

```go
store := docket.NewRedisStore(rdb, "myapp:prod:docket:")
```

### Automatic TTL

Redis natively supports TTL (time-to-live) for keys:
- Keys automatically expire after the specified duration
- No manual cleanup needed
- Saves memory by removing stale entries

```go
// Results cached for 1 hour, then automatically deleted
docket.WithPersistence(store, docket.ScopeGlobal, 1*time.Hour)
```

### Performance

Redis is **significantly faster** than SQL-based stores:
- **In-memory storage** - microsecond latency
- **Single roundtrip** - no query parsing overhead
- **Native protobuf support** - efficient binary storage

Typical latencies:
- Local Redis: 1-5ms
- SQLite: 5-20ms
- Remote Redis (same datacenter): 1-10ms
- Remote Postgres (same datacenter): 10-50ms

## When to Use Redis

✅ **Good for:**
- High-throughput applications needing fast caching
- Temporary/ephemeral data (TTL is important)
- Distributed systems (multiple servers share cache)
- Real-time applications requiring low latency

❌ **Not ideal for:**
- Critical data requiring ACID guarantees (use Postgres/MySQL)
- Long-term persistence (Redis is primarily in-memory)
- Complex queries across cached data
- When Redis infrastructure isn't available

## Redis Configuration Tips

### Production Setup

```go
rdb := redis.NewClient(&redis.Options{
    Addr:         "redis.example.com:6379",
    Password:     os.Getenv("REDIS_PASSWORD"),
    DB:           0,
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
    PoolSize:     10,
})
```

### Redis Cluster

For high availability, use Redis Cluster:

```go
rdb := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{
        "redis-1:6379",
        "redis-2:6379",
        "redis-3:6379",
    },
    Password: os.Getenv("REDIS_PASSWORD"),
})

// Note: RedisStore works with both Client and ClusterClient
// since they implement the same interface
```

### Monitoring

Check Redis memory usage:
```bash
redis-cli info memory
```

View keys created by your app:
```bash
redis-cli KEYS "docket:example:*"
```

## Comparison with Other Stores

```
┌─────────────┬────────────┬─────────────┬──────────────┬──────────────┐
│ Backend     │ Latency    │ Durability  │ Scalability  │ Complexity   │
├─────────────┼────────────┼─────────────┼──────────────┼──────────────┤
│ InMemory    │ Fastest    │ None        │ Single node  │ Simplest     │
│ Redis       │ Very Fast  │ Configurable│ Distributed  │ Easy         │
│ SQLite      │ Fast       │ Full        │ Single node  │ Easy         │
│ Postgres    │ Good       │ Full        │ Distributed  │ Moderate     │
│ MySQL       │ Good       │ Full        │ Distributed  │ Moderate     │
└─────────────┴────────────┴─────────────┴──────────────┴──────────────┘
```

## Related Examples

- `examples/persistence_sqlite` - SQLite-based caching
- `examples/persistence_postgres` - Postgres-based caching
- `examples/persistence_mysql` - MySQL-based caching
- `examples/scope_comparison` - ScopeWorkflow vs ScopeGlobal
- `examples/exactly_once` - Exactly-once semantics
