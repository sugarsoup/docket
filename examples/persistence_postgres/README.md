# Postgres Persistence Example

This example demonstrates how to use `docket` with a Postgres backing store using `pgxpool`.

## Prerequisites

- Go 1.23+
- Docker (for running the Postgres instance)

## Running the Example

### Automated

We provide a script that sets up a temporary Postgres container, runs the example, and cleans up:

```bash
./run.sh
```

### Manual

1. Start Postgres:
   ```bash
   docker run --name proto-pg -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres
   ```

2. Run the example:
   ```bash
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
   go run main.go
   ```

3. Cleanup:
   ```bash
   docker rm -f proto-pg
   ```

