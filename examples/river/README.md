# River Integration Example

This example demonstrates integrating Docket with [River](https://riverqueue.com), a robust Postgres-based job queue for Go.

## What This Shows

1. **Workflow as Job**: A River job executes a complete Docket graph
2. **Shared Postgres**: Both River and Docket persistence use the same `pgxpool`
3. **Execution ID Linking**: River job ID becomes the Docket execution ID for traceability
4. **Caching Across Jobs**: Same inputs hit the Docket cache, even across different River jobs
5. **Graceful Shutdown**: Context cancellation propagates from River to Docket

## Prerequisites

- Go 1.23+
- Docker

## Running

```bash
./run.sh
```

This will:
1. Start a temporary Postgres container
2. Create River tables
3. Run the example (enqueue jobs, process them, show cache hits)
4. Clean up

## Expected Output

```
🚀 Starting Postgres container...
✅ Postgres is ready!
📦 Running River migrations...
🏃 Running example...

--- Enqueueing Jobs ---
Enqueued job for "hello"
Enqueued job for "world"
Enqueued job for "hello"

--- Processing Jobs ---
[Worker] Computing letter count for "hello"
[Worker] Computing letter count for "world"
(Note: second "hello" hits cache - no log)

--- Enqueueing Duplicate (should hit cache) ---
Enqueued duplicate job for "hello"
(Cache hit - no computation log)

--- Shutting Down ---
SUCCESS: River + Docket integration complete!
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Postgres                            │
│  ┌─────────────────────┐  ┌──────────────────────────────┐  │
│  │    river_job        │  │     docket_cache         │  │
│  │  (job queue)        │  │   (step result cache)        │  │
│  └─────────────────────┘  └──────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │    pgxpool.Pool   │
                    └─────────┬─────────┘
          ┌───────────────────┼───────────────────┐
          │                   │                   │
    ┌─────┴─────┐      ┌──────┴──────┐    ┌───────┴───────┐
    │   River   │      │ Docket  │    │ PostgresStore │
    │  Client   │─────▶│   Graph     │───▶│   (cache)     │
    └───────────┘      └─────────────┘    └───────────────┘
```

