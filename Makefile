BINARY:=ipxe-bin
OSFLAG:= $(shell go env GOHOSTOS)
BUILD_ARGS:=GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -extldflags "-static"'

help: ## show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: test
test: ## Run unit tests
	go test -v -covermode=count ./...

.PHONY: darwin
darwin: ## compile for darwin
	GOOS=darwin ${BUILD_ARGS} -o bin/${BINARY}-darwin-amd64 main.go

.PHONY: linux
linux: ## compile for linux
	GOOS=linux ${BUILD_ARGS} -o bin/${BINARY}-linux-amd64 main.go

.PHONY: build
build: ## compile the binary for the native OS
ifeq (${OSFLAG},linux)
	@$(MAKE) linux
else
	@$(MAKE) darwin
endif

# BEGIN: lint-install /Users/jacobweinstock/repos/jacobweinstock/ipxe-bin
# http://github.com/tinkerbell/lint-install

GOLINT_VERSION ?= v1.42.1



LINT_OS := $(shell uname)
LINT_ARCH := $(shell uname -m)
LINT_ROOT := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# shellcheck and hadolint lack arm64 native binaries: rely on x86-64 emulation
ifeq ($(LINT_OS),Darwin)
	ifeq ($(LINT_ARCH),arm64)
		LINT_ARCH=x86_64
	endif
endif


GOLINT_CONFIG = $(LINT_ROOT)/.golangci.yml


.PHONY: lint
lint: out/linters/golangci-lint-$(GOLINT_VERSION)-$(LINT_ARCH) ## run linting checks
	find . -name go.mod -execdir "$(LINT_ROOT)/out/linters/golangci-lint-$(GOLINT_VERSION)-$(LINT_ARCH)" run -c "$(GOLINT_CONFIG)" \;

.PHONY: fix
fix: out/linters/golangci-lint-$(GOLINT_VERSION)-$(LINT_ARCH) ## fix linting errors
	find . -name go.mod -execdir "$(LINT_ROOT)/out/linters/golangci-lint-$(GOLINT_VERSION)-$(LINT_ARCH)" run -c "$(GOLINT_CONFIG)" --fix \;

out/linters/golangci-lint-$(GOLINT_VERSION)-$(LINT_ARCH):
	mkdir -p out/linters
	rm -rf out/linters/golangci-lint-*
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b out/linters $(GOLINT_VERSION)
	mv out/linters/golangci-lint out/linters/golangci-lint-$(GOLINT_VERSION)-$(LINT_ARCH)

# END: lint-install /Users/jacobweinstock/repos/jacobweinstock/ipxe-bin
