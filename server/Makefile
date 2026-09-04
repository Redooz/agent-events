GO ?= go

.PHONY: all build run test lint fmt tidy

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
