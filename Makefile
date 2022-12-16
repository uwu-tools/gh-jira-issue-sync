.DEFAULT_GOAL: build

export GO15VENDOREXPERIMENT=1
export CGO_ENABLED:=0

PROJ=issue-sync
ORG_PATH=github.com/coreos
REPO_PATH=$(ORG_PATH)/$(PROJ)
VERSION=$(shell ./git-version)
BUILD_TIME=`date +%FT%T%z`
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
SOURCES := $(shell find . -name '*.go')
LD_FLAGS=-ldflags "-X $(REPO_PATH)/cmd.Version=$(VERSION)"

GOLANGCI_VERSION = 1.49.0

build: bin/$(PROJ)

.PHONY: test
test: ## Run Unit Tests
	@(go test -v -covermode=atomic -coverprofile=unit-coverage.out ./...)

bin/$(PROJ): $(SOURCES)
	@go build -o bin/$(PROJ) $(LD_FLAGS) $(REPO_PATH)

.PHONY: clean
clean:
	@rm bin/*

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint

bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -s -- -b ./bin/ v${GOLANGCI_VERSION}
	@mv bin/golangci-lint $@

.PHONY: lint
lint: bin/golangci-lint ## Run linter
	./bin/golangci-lint run -v

.PHONY: lint-fix
lint-fix: bin/golangci-lint ## Fix lint violations
	./bin/golangci-lint run --fix
