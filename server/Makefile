GO ?= go

.PHONY: all build run test lint fmt tidy db-up db-down

all: lint test build

build:
	$(GO) build ./...

run:
	$(GO) run ./cmd/api

test:
	$(GO) test ./... -race -count=1

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

tidy:
	$(GO) mod tidy

db-up:
	docker compose up -d postgres

db-down:
	docker compose down
