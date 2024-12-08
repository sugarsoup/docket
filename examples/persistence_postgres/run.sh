#!/bin/bash
set -e

CONTAINER_NAME="docket-pg-example"
DB_NAME="protograph_example"
DB_USER="postgres"
DB_PASS="postgres"
DB_PORT="5432"

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
# Wait for pg_isready inside the container
for i in {1..30}; do
    if docker exec $CONTAINER_NAME pg_isready -U $DB_USER -d $DB_NAME >/dev/null 2>&1; then
        echo "✅ Postgres is ready!"
        break
    fi
    echo -n "."
    sleep 1
done

echo "🏃 Running example..."
export DATABASE_URL="postgres://$DB_USER:$DB_PASS@localhost:$DB_PORT/$DB_NAME?sslmode=disable"
go run main.go

echo "✨ Done!"

