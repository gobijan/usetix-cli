.DEFAULT_GOAL := help

BINARY := $(CURDIR)/bin/usetix
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*' | sort)

.PHONY: help run build install clean fmt fmt-check tidy tidy-check test test-race \
	coverage vet surface surface-update check ci completions release-snapshot \
	release-version release-check release

help:
	@echo "Usetix CLI"
	@echo ""
	@echo "Usage:"
	@echo "  make run ARGS='events list'  Run the CLI from source"
	@echo "  make build                   Build bin/usetix"
	@echo "  make install                 Install the CLI with go install"
	@echo "  make clean                   Remove generated build and release artifacts"
	@echo ""
	@echo "  make fmt                     Format Go source files"
	@echo "  make fmt-check               Check Go formatting without changing files"
	@echo "  make tidy                    Tidy go.mod and go.sum"
	@echo "  make tidy-check              Check module tidiness without changing files"
	@echo "  make test                    Run all tests"
	@echo "  make test-race               Run all tests with the race detector"
	@echo "  make coverage                Write coverage.out and print coverage"
	@echo "  make vet                     Run go vet"
	@echo "  make check                   Run the normal local quality gate"
	@echo "  make ci                      Run the full CI gate, including race tests"
	@echo ""
	@echo "  make surface                 Verify the committed CLI surface snapshot"
	@echo "  make surface-update          Update and verify .surface intentionally"
	@echo "  make completions             Generate shell completions under dist/"
	@echo ""
	@echo "  make release-snapshot        Build an unpublished GoReleaser snapshot"
	@echo "  make release-check           Validate a clean, pushed main for release"
	@echo "  make release VERSION=v0.1.4  Validate, tag, and push a release"

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/usetix $(ARGS)

build:
	@mkdir -p $(dir $(BINARY))
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/usetix

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/usetix

clean:
	rm -rf "$(CURDIR)/bin" "$(CURDIR)/dist"
	rm -f "$(CURDIR)/coverage.out"

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files need formatting:"; \
		echo "$$unformatted"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi

tidy:
	go mod tidy

tidy-check:
	go mod tidy -diff

test:
	go test ./...

test-race:
	go test -race -count=1 ./...

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

vet:
	go vet ./...

surface:
	go test ./internal/cli -run '^TestSurface$$' -count=1

surface-update:
	go test ./internal/cli -run '^TestSurface$$' -count=1 -update-surface
	$(MAKE) surface

check: fmt-check tidy-check vet test build

ci: check test-race

completions: build
	@mkdir -p "$(CURDIR)/dist/completions"
	$(BINARY) completion bash > "$(CURDIR)/dist/completions/usetix.bash"
	$(BINARY) completion zsh > "$(CURDIR)/dist/completions/_usetix"
	$(BINARY) completion fish > "$(CURDIR)/dist/completions/usetix.fish"
	$(BINARY) completion powershell > "$(CURDIR)/dist/completions/usetix.ps1"

release-snapshot:
	@command -v goreleaser >/dev/null || { echo "goreleaser is required"; exit 1; }
	goreleaser release --snapshot --clean

release-version:
	@printf '%s\n' "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { \
		echo "VERSION must be a tag like v0.1.4"; \
		exit 1; \
	}

release-check: ci
	@test -z "$$(git status --porcelain)" || { \
		echo "Working tree is not clean"; \
		git status --short; \
		exit 1; \
	}
	@test "$$(git branch --show-current)" = "main" || { \
		echo "Releases must be cut from main"; \
		exit 1; \
	}
	git fetch --quiet origin main --tags
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || { \
		echo "Local main must exactly match origin/main"; \
		exit 1; \
	}

release: release-version release-check
	@if git rev-parse --verify --quiet "refs/tags/$(VERSION)" >/dev/null; then \
		echo "Tag $(VERSION) already exists"; \
		exit 1; \
	fi
	git tag "$(VERSION)"
	git push origin "$(VERSION)"
