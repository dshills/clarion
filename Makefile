BINARY      := bin/clarion
CMD         := ./cmd/clarion
VERSION     := v0.1.0
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILT       := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.built=$(BUILT)

.PHONY: build test lint clean golden release

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

test:
	go test ./... ./internal/testdata/

golden:
	go test ./internal/testdata/ -update

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

release:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/clarion-linux-amd64   $(CMD)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/clarion-linux-arm64   $(CMD)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/clarion-darwin-amd64  $(CMD)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/clarion-darwin-arm64  $(CMD)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/clarion-windows-amd64.exe $(CMD)
