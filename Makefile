ifeq (,$(shell echo $$DEBUG))
else
SHELL = bash -x
endif

GIT_TAG ?= dirty-tag
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_HASH ?= $(shell git rev-parse HEAD)
GIT_TREESTATE = "clean"
DATE_FMT = +%Y-%m-%dT%H:%M:%SZ
SOURCE_DATE_EPOCH ?= $(shell git log -1 --no-show-signature --pretty=%ct)
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif

SRCS = $(shell find cmd -iname "*.go") $(shell find pkg -iname "*.go")

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

LDFLAGS=-buildid= -X sigs.k8s.io/release-utils/version.gitVersion=$(GIT_VERSION) \
        -X sigs.k8s.io/release-utils/version.gitCommit=$(GIT_HASH) \
        -X sigs.k8s.io/release-utils/version.gitTreeState=$(GIT_TREESTATE) \
        -X sigs.k8s.io/release-utils/version.buildDate=$(BUILD_DATE)

PLATFORMS=darwin linux
ARCHITECTURES=amd64 arm64
GOLANGCI_LINT_DIR = $(shell pwd)/bin
GOLANGCI_LINT_BIN = $(GOLANGCI_LINT_DIR)/golangci-lint

GO ?= go
TEST_FLAGS ?= -v -cover

.PHONY: all lint test clean pombump cross vendor
all: clean lint test pombump

clean:
	rm -rf pombump pombump-*

pombump:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $@ .

.PHONY: cross
cross:
	$(foreach GOOS, $(PLATFORMS),\
		$(foreach GOARCH, $(ARCHITECTURES), $(shell export GOOS=$(GOOS); export GOARCH=$(GOARCH); \
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o pombump-$(GOOS)-$(GOARCH) .; \
	shasum -a 256 pombump-$(GOOS)-$(GOARCH) > pombump-$(GOOS)-$(GOARCH).sha256 ))) \

.PHONY: test
test:
	$(GO) vet ./...
	$(GO) test ${TEST_FLAGS} ./...

.PHONY: test-integration
test-integration:
	$(GO) test ${TEST_FLAGS} -run TestIntegration ./pkg

.PHONY: bench
bench:
	$(GO) test -bench=. -benchtime=10s -run=^$$ ./pkg

.PHONY: coverage
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

golangci-lint:
	rm -f $(GOLANGCI_LINT_BIN) || :
	set -e ;\
	GOBIN=$(GOLANGCI_LINT_DIR) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

lint: golangci-lint
	$(GOLANGCI_LINT_BIN) run ./...

.PHONY: vendor
vendor:
	$(GO) mod vendor
	$(GO) mod tidy

.PHONY: fmt
fmt:
	$(GO) fmt ./...
	gofmt -s -w $(SRCS)

.PHONY: install
install: pombump
	install -D -m 755 pombump $(DESTDIR)$(PREFIX)/bin/pombump

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all              - Clean, lint, test, and build"
	@echo "  pombump          - Build the pombump binary"
	@echo "  clean            - Remove built binaries"
	@echo "  test             - Run all tests"
	@echo "  test-integration - Run integration tests only"
	@echo "  bench            - Run benchmarks"
	@echo "  coverage         - Generate test coverage report"
	@echo "  lint             - Run golangci-lint"
	@echo "  fmt              - Format code"
	@echo "  vendor           - Update vendor directory"
	@echo "  cross            - Build for multiple platforms"
	@echo "  install          - Install pombump to system"
	@echo "  help             - Show this help message"

.DEFAULT_GOAL := help