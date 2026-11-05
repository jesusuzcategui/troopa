.PHONY: build test lint

build:
	go build -o bin/troopa ./cmd/troopa

test:
	go test ./...

lint:
	go vet ./...
