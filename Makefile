# thine — developer tasks. Every target runs the same commands CI runs.

BIN      ?= bin/thine
PKG      := ./...
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)
LDFLAGS  := -s -w \
	-X github.com/YourAKShaw/thinerc/internal/buildinfo.Version=$(VERSION) \
	-X github.com/YourAKShaw/thinerc/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/YourAKShaw/thinerc/internal/buildinfo.Date=$(DATE)

export CGO_ENABLED := 0

.PHONY: all build install test race vet fmt fmt-check lint cross check clean demo

all: check build

build: ## Build the binary into ./bin
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/thine

install: ## Install into ~/.local/bin (no sudo)
	mkdir -p $(HOME)/.local/bin
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(HOME)/.local/bin/thine ./cmd/thine
	@echo "installed $(HOME)/.local/bin/thine — now run: thine setup"

test: ## Unit and integration tests
	go test $(PKG)

race: ## Tests with the race detector (needs cgo)
	CGO_ENABLED=1 go test -race $(PKG)

vet:
	go vet $(PKG)

fmt:
	gofmt -s -w .

fmt-check:
	@out="$$(gofmt -s -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

lint: fmt-check vet ## Formatting and static checks
	@scripts/check-boundaries.sh
	@scripts/trace-invariants.sh

cross: ## Cross-compile every supported target
	@for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; \
		echo "→ $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/thine-$$os-$$arch ./cmd/thine || exit 1; \
	done

check: lint test cross ## Everything CI checks

demo: build ## Play the animation showcase
	$(BIN) fx all

clean:
	rm -rf bin dist coverage.out
