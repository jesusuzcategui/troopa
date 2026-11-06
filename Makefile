.PHONY: build test lint site snapshot

build:
	go build -o bin/troopa ./cmd/troopa

test:
	go test ./...

lint:
	go vet ./...

site:
	cd github_page && npm ci && npm run build

snapshot:
	goreleaser release --snapshot --clean
