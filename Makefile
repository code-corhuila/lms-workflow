.PHONY: dev test test-cover build lint

dev:
	go run ./cmd/worker/...

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

build:
	go build -o bin/worker ./cmd/worker/...

lint:
	golangci-lint run ./...

# Migrations live in migrations/ in this repo (not a separate lms-workflow-db)
# — see README's "Migration scope" for why.
