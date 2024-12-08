# MySQL Persistence Example

This example demonstrates using **MySQL** as a persistence backend for Docket's caching layer.

## What This Demonstrates

- Using `SQLStore` with `DialectMySQL` to cache step results in MySQL
- Durable, ACID-compliant caching with automatic TTL handling
- Cross-execution and cross-connection caching with `ScopeGlobal`
- Production-ready persistence for distributed systems

## Prerequisites

**MySQL must be running locally:**

```bash
# Using Docker
docker run -d \
  --name mysql-docket \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=docket_test \
  mysql:8

# Or using Homebrew (macOS)
brew install mysql
brew services start mysql

# Or using apt (Linux)
sudo apt install mysql-server
sudo systemctl start mysql
```

**Create the test database:**

```bash
# Connect to MySQL
mysql -u root -p

# Create database
CREATE DATABASE IF NOT EXISTS docket_test;
exit;
```

## Running the Example

```bash
go run examples/persistence_mysql/main.go
```

**Note:** Update the DSN in `main.go` if you're using a password:
```go
dsn := "root:yourpassword@tcp(localhost:3306)/docket_test?parseTime=true"
```

## Expected Output

```
Connected to MySQL successfully
Database schema initialized

--- Run 1 (Cold) ---
  [Computing] Count for "hello mysql" (sleeping 200ms)...
Result: Count = 11
Duration: 205ms
Verified: Found 1 row(s) in database

--- Run 2 (Warm) ---
Result: Count = 11
Duration: 3ms

--- Run 3 (New Connection) ---
Result: Count = 11
Duration: 4ms (should be fast - using persisted cache)

SUCCESS: MySQL persistence test passed!
```

## Key Features

### SQLStore with MySQL Dialect

```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "docket/pkg/docket"
)

// Open MySQL connection
db, err := sql.Open("mysql", "user:pass@tcp(host:port)/database?parseTime=true")

// Create store with MySQL dialect
store := docket.NewSQLStore(db, "step_cache", docket.DialectMySQL)

// Initialize schema
if err := store.InitSchema(ctx); err != nil {
    log.Fatal(err)
}
```

### Connection String Format

```
[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
```

Common examples:
```go
// Local development (no password)
"root@tcp(localhost:3306)/docket_test?parseTime=true"

// With password
"myuser:mypass@tcp(localhost:3306)/docket_test?parseTime=true"

// Remote server
"app_user:secret@tcp(db.example.com:3306)/production?parseTime=true"

// With SSL/TLS
"user:pass@tcp(host:3306)/db?tls=true&parseTime=true"

// Connection pooling params
"user:pass@tcp(host:3306)/db?parseTime=true&maxAllowedPacket=0&maxOpenConns=10"
```

**Important:** Always include `?parseTime=true` to handle timestamp fields correctly.

### Schema

The MySQL table structure:

```sql
CREATE TABLE IF NOT EXISTS step_cache (
    key TEXT PRIMARY KEY,
    value BLOB,
    error TEXT,
    created_at DATETIME,
    expires_at DATETIME,
    metadata TEXT
);
```

Differences from Postgres/SQLite:
- Uses `DATETIME` instead of `TIMESTAMP`/`TIMESTAMPTZ`
- Uses `ON DUPLICATE KEY UPDATE` instead of `ON CONFLICT`
- Compatible with MySQL 5.7+ and MariaDB 10.2+

### Production Configuration

```go
import "database/sql"

db, err := sql.Open("mysql", dsn)
if err != nil {
    log.Fatal(err)
}

// Configure connection pool
db.SetMaxOpenConns(25)           // Max open connections
db.SetMaxIdleConns(5)            // Max idle connections
db.SetConnMaxLifetime(5 * time.Minute)  // Max connection reuse time
db.SetConnMaxIdleTime(1 * time.Minute)  // Max idle time

// Test the connection
if err := db.PingContext(ctx); err != nil {
    log.Fatal(err)
}
```

## When to Use MySQL

✅ **Good for:**
- Applications already using MySQL as primary database
- Need ACID guarantees for cached data
- Multi-server deployments (shared cache via network)
- Long-term persistence of expensive computations
- Exactly-once semantics for critical operations

❌ **Not ideal for:**
- Extremely high-throughput caching (use Redis instead)
- Single-server applications (SQLite is simpler)
- Temporary/ephemeral data (Redis is faster)
- When sub-5ms latency is required

## Performance Tips

### 1. Add Index on expires_at

For faster expiration queries:

```sql
CREATE INDEX idx_expires_at ON step_cache(expires_at);
```

### 2. Periodic Cleanup

Remove expired entries to save space:

```sql
-- Run periodically via cron or scheduled job
DELETE FROM step_cache
WHERE expires_at IS NOT NULL
  AND expires_at < NOW();
```

Or use MySQL's built-in event scheduler:

```sql
-- Enable event scheduler
SET GLOBAL event_scheduler = ON;

-- Create cleanup event
CREATE EVENT cleanup_expired_cache
ON SCHEDULE EVERY 1 HOUR
DO
  DELETE FROM step_cache
  WHERE expires_at IS NOT NULL
    AND expires_at < NOW();
```

### 3. Optimize Table

Regularly optimize the table to reclaim space:

```bash
# Run periodically
mysql -u root -p -e "OPTIMIZE TABLE docket_test.step_cache;"
```

## MySQL vs Other Stores

```
┌─────────────┬────────────┬─────────────┬──────────────┬──────────────┐
│ Backend     │ Latency    │ Durability  │ Scalability  │ Use Case     │
├─────────────┼────────────┼─────────────┼──────────────┼──────────────┤
│ MySQL       │ 10-50ms    │ Full ACID   │ Distributed  │ Production   │
│ Postgres    │ 10-50ms    │ Full ACID   │ Distributed  │ Production   │
│ SQLite      │ 5-20ms     │ Full ACID   │ Single node  │ Development  │
│ Redis       │ 1-5ms      │ Configurable│ Distributed  │ Speed        │
│ InMemory    │ <1ms       │ None        │ Single node  │ Testing      │
└─────────────┴────────────┴─────────────┴──────────────┴──────────────┘
```

### MySQL vs Postgres

Both are excellent choices. Choose based on your existing infrastructure:

**MySQL:**
- Simpler replication setup
- Better for read-heavy workloads
- More deployment options (RDS, Azure, GCP)

**Postgres:**
- More advanced features (JSONB, arrays, etc.)
- Better for complex queries
- Stronger standards compliance

For Docket caching, **performance is nearly identical** - choose what you already know.

## Docker Compose Example

```yaml
version: '3.8'
services:
  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: docket_test
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql

  app:
    build: .
    depends_on:
      - mysql
    environment:
      MYSQL_DSN: "root:rootpass@tcp(mysql:3306)/docket_test?parseTime=true"

volumes:
  mysql_data:
```

## Monitoring

### Check table size:

```sql
SELECT
    table_name,
    ROUND((data_length + index_length) / 1024 / 1024, 2) AS size_mb,
    table_rows
FROM information_schema.TABLES
WHERE table_schema = 'docket_test'
  AND table_name = 'step_cache';
```

### View cached entries:

```sql
SELECT
    key,
    created_at,
    expires_at,
    TIMESTAMPDIFF(SECOND, NOW(), expires_at) AS ttl_seconds
FROM step_cache
ORDER BY created_at DESC
LIMIT 10;
```

### Count expired entries:

```sql
SELECT COUNT(*) as expired_count
FROM step_cache
WHERE expires_at IS NOT NULL
  AND expires_at < NOW();
```

## Related Examples

- `examples/persistence_sqlite` - SQLite-based caching
- `examples/persistence_postgres` - Postgres-based caching
- `examples/persistence_redis` - Redis-based caching
- `examples/scope_comparison` - ScopeWorkflow vs ScopeGlobal
- `examples/exactly_once` - Exactly-once semantics
