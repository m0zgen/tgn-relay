APP := tgn-relay

VERSION ?= 0.1.0
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

PKG_VERSION := github.com/m0zgen/tgn-relay/internal/version

LDFLAGS := -s -w \
	-X $(PKG_VERSION).Version=$(VERSION) \
	-X $(PKG_VERSION).Commit=$(COMMIT) \
	-X $(PKG_VERSION).Date=$(DATE)

.PHONY: build run test tidy clean snapshot check-release release status

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/$(APP)

run:
	go run ./cmd/$(APP) -config configs/config.example.yml

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist

status:
	git status --short

snapshot:
	go test ./...
	goreleaser check
	goreleaser release --snapshot --clean

check-release:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make check-release VERSION=v0.1.1"; \
		exit 1; \
	fi
	@if ! echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "Invalid VERSION: $(VERSION)"; \
		echo "Expected format: v0.1.1"; \
		exit 1; \
	fi
	@if [ -n "$$(git status --short)" ]; then \
		echo "Git working tree is dirty. Commit changes first:"; \
		git status --short; \
		exit 1; \
	fi
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "Tag already exists: $(VERSION)"; \
		exit 1; \
	fi
	@echo "Release check OK: $(VERSION)"

release:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release VERSION=v0.1.1"; \
		exit 1; \
	fi
	./tools/release.sh $(VERSION)
	