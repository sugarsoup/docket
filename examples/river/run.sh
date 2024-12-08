#!/bin/bash
set -e

CONTAINER_NAME="docket-river-example"
DB_NAME="river_example"
DB_USER="postgres"
DB_PASS="postgres"
DB_PORT="5433"  # Use different port to avoid conflicts

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up..."
    docker rm -f $CONTAINER_NAME >/dev/null 2>&1 || true
}

# Trap cleanup on exit/error
trap cleanup EXIT

# Ensure cleanup of any previous run
cleanup

echo "🚀 Starting Postgres container..."
docker run --name $CONTAINER_NAME \
    -e POSTGRES_PASSWORD=$DB_PASS \
    -e POSTGRES_DB=$DB_NAME \
    -d -p $DB_PORT:5432 \
    postgres >/dev/null

echo "⏳ Waiting for Postgres to be ready..."
for i in {1..30}; do
    if docker exec $CONTAINER_NAME pg_isready -U $DB_USER -d $DB_NAME >/dev/null 2>&1; then
        echo "✅ Postgres is ready!"
        break
    fi
    echo -n "."
    sleep 1
done

echo "📦 Running River migrations..."
export DATABASE_URL="postgres://$DB_USER:$DB_PASS@localhost:$DB_PORT/$DB_NAME?sslmode=disable"

# Create River tables using their migration
docker exec $CONTAINER_NAME psql -U $DB_USER -d $DB_NAME -c "
CREATE TABLE IF NOT EXISTS river_job (
    id bigserial PRIMARY KEY,
    args jsonb NOT NULL DEFAULT '{}',
    attempt smallint NOT NULL DEFAULT 0,
    attempted_at timestamptz,
    attempted_by text[],
    created_at timestamptz NOT NULL DEFAULT now(),
    errors jsonb[],
    finalized_at timestamptz,
    kind text NOT NULL,
    max_attempts smallint NOT NULL DEFAULT 25,
    metadata jsonb NOT NULL DEFAULT '{}',
    priority smallint NOT NULL DEFAULT 1,
    queue text NOT NULL DEFAULT 'default',
    scheduled_at timestamptz NOT NULL DEFAULT now(),
    state text NOT NULL DEFAULT 'available',
    tags text[] NOT NULL DEFAULT '{}',
    unique_key bytea,
    unique_states bit(8)
);

CREATE TABLE IF NOT EXISTS river_leader (
    name text PRIMARY KEY,
    leader_id text NOT NULL,
    elected_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS river_queue (
    name text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}',
    paused_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);
" >/dev/null

echo "🏃 Running example..."
go run main.go

echo "✨ Done!"

