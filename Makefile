BINARY_NAME=agentenv
DIST_DIR=dist
GO=/usr/local/go/bin/go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test lint clean install release-dry-run release-snapshot

build:
	$(GO) build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/agentenv

test:
	$(GO) test ./... -v

lint:
	golangci-lint run

clean:
	rm -rf $(DIST_DIR)/

install:
	$(GO) install -ldflags="$(LDFLAGS)" ./cmd/agentenv

release-dry-run:
	goreleaser build --snapshot --clean

release-snapshot:
	goreleaser release --snapshot --clean
