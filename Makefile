APP=tgn-relay
VERSION?=0.1.0
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-s -w -X github.com/m0zgen/tgn-relay/internal/version.Version=$(VERSION) -X github.com/m0zgen/tgn-relay/internal/version.Commit=$(COMMIT) -X github.com/m0zgen/tgn-relay/internal/version.Date=$(DATE)

.PHONY: build run test clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/$(APP)

run:
	go run ./cmd/$(APP) -config configs/config.example.yml

test:
	go test ./...

clean:
	rm -rf bin
