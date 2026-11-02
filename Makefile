VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BINARY    := rfarm-be
CMD       := ./cmd/app
LDFLAGS   := -s -w -X main.buildVersion=$(VERSION)

## build: compile the binary
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

## run: build and run locally
run: build
	./$(BINARY)

## dev: run with live reload (requires: go install github.com/air-verse/air@latest)
dev:
	air

## test: run unit + handler tests (fast, no Docker)
test:
	go test ./... -v -race -count=1

## lint: run golangci-lint (doesn't work yet)
lint:
	golangci-lint run ./...

## tidy: tidy and verify modules
tidy:
	go mod tidy
	go mod verify

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
