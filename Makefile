# Makefile for the docket project

.PHONY: help proto build run test run-example

help:
	@echo "Available commands:"
	@echo "  make proto         - Compile protobuf files"
	@echo "  make build         - Build the application"
	@echo "  make run           - Run the application"
	@echo "  make test          - Run tests"
	@echo "  make coverage      - Run tests and generate coverage report"
	@echo "  make example-persistence-postgres - Run the postgres persistence example (requires Docker)"
	@echo "  make example-river               - Run the River queue integration example (requires Docker)"

proto:
	@echo "Compiling protobufs..."
	@export PATH="$(HOME)/go/bin:$(PATH)" && \
	find proto -name "*.proto" -exec protoc --go_out=. --go_opt=paths=source_relative {} +

build:
	@echo "Building the application..."
	@go build -o ./build/docket ./cmd/docket

run: build
	@echo "Running the application..."
	@./build/docket

test:
	@echo "Running tests..."
	@go test ./...

coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

run-example:
	@echo "Running example $(EXAMPLE)..."
	@go run ./examples/$(EXAMPLE)

example-persistence-postgres:
	@./examples/persistence_postgres/run.sh

example-river:
	@./examples/river/run.sh
