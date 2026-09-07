BINARY ?= bin/gitbook-practice-synchronization
export CGO_ENABLED ?= 0

.PHONY: build test lint detect dry-run run clean tidy

tidy:
	go mod tidy

build: tidy
	go build -o $(BINARY) ./cmd/gitbook-practice-synchronization

test:
	go test ./...

detect: build
	./$(BINARY) detect

dry-run: build
	./$(BINARY) run --dry-run=true

run: build
	./$(BINARY) run

clean:
	rm -rf bin .work
	go clean -testcache
