# lmmtfy build. Run `make help` to list targets.

GO      ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Static, reproducible binaries: no cgo, no local paths, stripped, version stamped.
export CGO_ENABLED := 0
BUILD_FLAGS := -trimpath -ldflags "-s -w -X lmmtfy/internal/version.Version=$(VERSION)"

# Platforms converters can be built for.
# Keep in sync with runners.Supported (checked by internal/runners tests).
RUNNER_TARGETS := windows/amd64 linux/amd64 linux/arm64 darwin/arm64 darwin/amd64
RUNNER_DIR     := internal/runners/bin

# Platforms the lmmtfy CLI itself is released for.
CLI_TARGETS := windows/amd64 linux/amd64 linux/arm64 darwin/arm64 darwin/amd64

DIST      := dist
HOST_OS   := $(shell $(GO) env GOOS)
HOST_ARCH := $(shell $(GO) env GOARCH)

.PHONY: all help runners build cross test vet fmt fmt-check golden-update smoke check clean

all: check build

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

runners: ## Cross-compile converter templates for every platform (gzipped, to embed)
	@for t in $(RUNNER_TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=; \
		if [ "$$os" = windows ]; then ext=.exe; fi; \
		echo "  runner  $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(BUILD_FLAGS) \
			-o $(RUNNER_DIR)/runner-$$os-$$arch$$ext ./cmd/runner || exit 1; \
		gzip -9 -n -f $(RUNNER_DIR)/runner-$$os-$$arch$$ext || exit 1; \
	done

build: runners ## Build the lmmtfy CLI for this machine into dist/
	$(GO) build $(BUILD_FLAGS) -o $(DIST)/lmmtfy ./cmd/lmmtfy

cross: runners ## Build the lmmtfy CLI for every platform into dist/
	@for t in $(CLI_TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=; \
		if [ "$$os" = windows ]; then ext=.exe; fi; \
		echo "  lmmtfy  $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(BUILD_FLAGS) \
			-o $(DIST)/lmmtfy-$$os-$$arch$$ext ./cmd/lmmtfy || exit 1; \
	done

test: ## Run all tests
	$(GO) test ./...

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format all Go files
	gofmt -w .

fmt-check: ## Fail if any Go file is not formatted
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "not gofmt'ed:"; echo "$$out"; exit 1; fi

golden-update: ## Rewrite golden expected files from actual results
	LMMTFY_UPDATE_GOLDEN=1 $(GO) test ./...

smoke: build ## Build a converter for this machine and check it converts like the CLI
	./$(DIST)/lmmtfy version
	./$(DIST)/lmmtfy build --mapping testdata/acme-orders/mapping.yaml --os $(HOST_OS) --arch $(HOST_ARCH) -o $(DIST)/smoke-converter
	./$(DIST)/smoke-converter --info
	./$(DIST)/smoke-converter testdata/acme-orders/input.csv $(DIST)/smoke-output.csv --log-level none
	cmp $(DIST)/smoke-output.csv testdata/acme-orders/output.csv
	@echo "  converter $(HOST_OS)/$(HOST_ARCH) output matches the golden file: ok"

check: fmt-check vet test ## Formatting, vet and tests

clean: ## Remove build output
	rm -rf $(DIST)
	find $(RUNNER_DIR) -type f ! -name .gitkeep -delete
