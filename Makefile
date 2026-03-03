BINARY      := bin/clarion
CMD         := ./cmd/clarion
VERSION     := v0.1.0
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILT       := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.built=$(BUILT)

.PHONY: build test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
