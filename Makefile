BINARY_NAME=agentenv
DIST_DIR=dist
GO=/usr/local/go/bin/go

.PHONY: build test lint clean install release-dry-run release-snapshot

build:
	$(GO) build -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/agentenv

test:
	$(GO) test ./... -v

lint:
	golangci-lint run

clean:
	rm -rf $(DIST_DIR)/

install:
	$(GO) install ./cmd/agentenv

release-dry-run:
	goreleaser build --snapshot --clean

release-snapshot:
	goreleaser release --snapshot --clean
