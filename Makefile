## Build, test, and generate code for various parts of Alloy.
##
## At least Go 1.22, git, and a moderately recent version of Docker is required
## to be able to use the Makefile. This list isn't exhaustive and there are other
## dependencies for the generate-* targets. If you do not have the full list of
## build dependencies, you may set USE_CONTAINER=1 to proxy build commands to a
## build container.
##
## Other environment variables can be used to tweak behaviors of targets.
## See the bottom of this help section for the full list of supported
## environment variables.
##
## Usage:
##   make <target>
##
## Targets for running tests:
##
##   test                  Run tests
##   lint                  Lint code
##   integration-test      Run integration tests
##   integration-test-k8s  Run Kubernetes integration tests
##
## Targets for building binaries:
##
##   binaries       Compiles all binaries.
##   alloy          Compiles Alloy to $(ALLOY_BINARY)
##   alloy-service  Compiles internal/cmd/alloy-service to $(SERVICE_BINARY)
##
## Targets for building Docker images:
##
##   images               Builds all (Linux) Docker images.
##   images-windows       Builds all (Windows) Docker images.
##   alloy-image          Builds alloy Docker image.
##   alloy-image-windows  Builds alloy Docker image for Windows.
##
## Targets for packaging:
##
##   dist                 Produce release assets for everything.
##   dist-alloy-binaries  Produce release-ready Alloy binaries.
##   dist-alloy-packages  Produce release-ready DEB and RPM packages.
##   dist-alloy-installer Produce a Windows installer for Alloy.
##
## Targets for generating assets:
##
##   generate                  Generate everything.
##   generate-helm-docs        Generate Helm chart documentation.
##   generate-helm-tests       Generate Helm chart tests.
##   generate-ui               Generate the UI assets.
##   generate-versioned-files  Generate versioned files.
##   generate-winmanifest      Generate the Windows application manifest.
##   generate-snmp             Generate SNMP modules from prometheus/snmp_exporter for prometheus.exporter.snmp and bumps SNMP version in _index.md.t.
##   sync-module-dependencies  Generate replace directives from dependency-replacements.yaml and inject them into go.mod and builder-config.yaml.
##
## Other targets:
##
##   build-container-cache  Create a cache for the build container to speed up
##                          subsequent proxied builds
##   clean                  Clean caches and built binaries
##   help                   Displays this message
##   info                   Print Makefile-specific environment variables
##
## Environment variables:
##
##   USE_CONTAINER        Set to 1 to enable proxying commands to build container
##   ALLOY_IMAGE          Image name:tag built by `make alloy-image`
##   ALLOY_IMAGE_WINDOWS  Image name:tag built by `make alloy-image-windows`
##   BUILD_IMAGE          Image name:tag used by USE_CONTAINER=1
##   ALLOY_BINARY         Output path of `make alloy` (default build/alloy)
##   SERVICE_BINARY       Output path of `make alloy-service` (default build/alloy-service)
##   GOOS                 Override OS to build binaries for
##   GOARCH               Override target architecture to build binaries for
##   GOARM                Override ARM version (6 or 7) when GOARCH=arm
##   CGO_ENABLED          Set to 0 to disable Cgo for binaries.
##   RELEASE_BUILD        Set to 1 to build release binaries.
##   VERSION              Version to inject into built binaries.
##   GO_TAGS              Extra tags to use when building.
##   DOCKER_PLATFORM      Overrides platform to build Docker images for (defaults to host platform).
##   GOEXPERIMENT         Used to enable Go features behind feature flags.

include tools/make/*.mk

ALLOY_IMAGE          ?= grafana/alloy:latest
ALLOY_IMAGE_WINDOWS  ?= grafana/alloy:windowsservercore-ltsc2022
ALLOY_BINARY         ?= build/alloy
SERVICE_BINARY       ?= build/alloy-service
ALLOYLINT_BINARY     ?= build/alloylint
GOOS                 ?= $(shell go env GOOS)
GOARCH               ?= $(shell go env GOARCH)
GOARM                ?= $(shell go env GOARM)
CGO_ENABLED          ?= 1
RELEASE_BUILD        ?= 0
GOEXPERIMENT         ?= $(shell go env GOEXPERIMENT)

# Determine the golangci-lint binary path using Make functions where possible.
# Priority: GOBIN, GOPATH/bin, PATH (via shell), Fallback Name.
# Uses GNU Make's $(or ...) function for lazy evaluation based on priority.
# $(wildcard ...) checks for existence. PATH check still uses shell for practicality.
# Allows override via environment/command line using ?=
GOLANGCI_LINT_BINARY ?= $(or \
    $(if $(shell go env GOBIN),$(wildcard $(shell go env GOBIN)/golangci-lint)), \
    $(wildcard $(shell go env GOPATH)/bin/golangci-lint), \
    $(shell command -v golangci-lint 2>/dev/null), \
    golangci-lint \
)

# List of all environment variables which will propagate to the build
# container. USE_CONTAINER must _not_ be included to avoid infinite recursion.
PROPAGATE_VARS := \
    ALLOY_IMAGE ALLOY_IMAGE_WINDOWS \
    BUILD_IMAGE GOOS GOARCH GOARM CGO_ENABLED RELEASE_BUILD \
    ALLOY_BINARY \
    VERSION GO_TAGS GOEXPERIMENT GOLANGCI_LINT_BINARY \

#
# Constants for targets
#

GO_ENV := GOEXPERIMENT=$(GOEXPERIMENT) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED)

VERSION      ?= $(shell bash ./tools/image-tag)
GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH   := $(shell git rev-parse --abbrev-ref HEAD)
VPREFIX      := github.com/grafana/alloy/internal/build
VPREFIXSYNTAX := github.com/grafana/alloy/syntax/internal/stdlib
GO_LDFLAGS   := -X $(VPREFIX).Branch=$(GIT_BRANCH)                        \
                -X $(VPREFIX).Version=$(VERSION)                          \
		-X $(VPREFIXSYNTAX).Version=$(VERSION)                    \
                -X $(VPREFIX).Revision=$(GIT_REVISION)                    \
                -X $(VPREFIX).BuildUser=$(shell whoami)@$(shell hostname) \
                -X $(VPREFIX).BuildDate=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

DEFAULT_FLAGS    := $(GO_FLAGS)
DEBUG_GO_FLAGS   := -ldflags "$(GO_LDFLAGS)" -tags "$(GO_TAGS)"
RELEASE_GO_FLAGS := -ldflags "-s -w $(GO_LDFLAGS)" -tags "$(GO_TAGS)"

ifeq ($(RELEASE_BUILD),1)
GO_FLAGS := $(DEFAULT_FLAGS) $(RELEASE_GO_FLAGS)
else
GO_FLAGS := $(DEFAULT_FLAGS) $(DEBUG_GO_FLAGS)
endif

#
# Targets for running tests
#
# These targets currently don't support proxying to a build container.
#

.PHONY: lint
lint: alloylint
	find . -name go.mod | xargs dirname | xargs -I __dir__ $(GOLANGCI_LINT_BINARY) run -v --timeout=10m
	GOFLAGS="-tags=$(GO_TAGS)" $(ALLOYLINT_BINARY) ./...

.PHONY: run-alloylint
run-alloylint: alloylint
	GOFLAGS="-tags=$(GO_TAGS)" $(ALLOYLINT_BINARY) ./...

.PHONY: test
# We have to run test twice: once for all packages with -race and then once
# more for packages that exclude tests via //go:build !race due to known race detection issues. The
# final command runs tests for syntax module.
test:
	$(GO_ENV) go test $(GO_FLAGS) -race $(shell go list ./... | grep -v -E '/integration-tests/|/integration-tests-k8s/')
	$(GO_ENV) go test $(GO_FLAGS) ./internal/static/integrations/node_exporter
	$(GO_ENV) cd ./syntax && go test -race ./...

test-packages:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	docker pull $(BUILD_IMAGE)
	go test -tags=packaging -race ./internal/tools/packaging_test
endif

.PHONY: integration-test
integration-test:
	cd internal/cmd/integration-tests && $(GO_ENV) go run .

.PHONY: test-pyroscope
test-pyroscope:
	$(GO_ENV) go test $(GO_FLAGS) -race $(shell go list ./... | grep pyroscope)
	cd ./internal/component/pyroscope/util/internal/cmd/playground/ && \
		$(GO_ENV) go build .

.PHONY: integration-test-k8s
integration-test-k8s: alloy-image
	cd ./internal/cmd/integration-tests-k8s/ && \
		$(GO_ENV) go test -timeout 10m ./...

#
# Targets for building binaries
#

.PHONY: binaries alloy
binaries: alloy

alloy: sync-module-dependencies
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(GO_ENV) go build $(GO_FLAGS) -o $(ALLOY_BINARY) .
endif

# alloy-service is not included in binaries since it's Windows-only.
alloy-service:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(GO_ENV) go build $(GO_FLAGS) -o $(SERVICE_BINARY) ./internal/cmd/alloy-service
endif

alloylint:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	cd ./internal/cmd/alloylint && $(GO_ENV) go build $(GO_FLAGS) -o ../../../$(ALLOYLINT_BINARY) .
endif

#
# Targets for building Docker images
#

DOCKER_FLAGS := --build-arg RELEASE_BUILD=$(RELEASE_BUILD) --build-arg VERSION=$(VERSION)

ifneq ($(DOCKER_PLATFORM),)
DOCKER_FLAGS += --platform=$(DOCKER_PLATFORM)
endif

.PHONY: images alloy-image
images: alloy-image

alloy-image:
	DOCKER_BUILDKIT=1 docker build $(DOCKER_FLAGS) -t $(ALLOY_IMAGE) -f Dockerfile .

.PHONY: images-windows alloy-image-windows
images: alloy-image-windows

alloy-image-windows:
	docker build $(DOCKER_FLAGS) -t $(ALLOY_IMAGE_WINDOWS) -f Dockerfile.windows .

#
# Targets for generating assets
#

.PHONY: generate generate-helm-docs generate-helm-tests generate-ui generate-versioned-files generate-winmanifest generate-snmp sync-module-dependencies
generate: generate-helm-docs generate-helm-tests generate-ui generate-versioned-files generate-docs generate-winmanifest generate-snmp sync-module-dependencies

generate-helm-docs:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	cd operations/helm/charts/alloy && helm-docs
endif

generate-helm-tests:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	bash ./operations/helm/scripts/rebuild-tests.sh
endif

sync-module-dependencies:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	cd ./tools/sync-module-dependencies && $(GO_ENV) go generate
endif

generate-ui:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	cd ./internal/web/ui && npm install && npm run build
endif

generate-versioned-files:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	sh ./tools/gen-versioned-files/gen-versioned-files.sh
endif

generate-docs:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	go generate ./internal/tools/docs_generator/
endif

generate-winmanifest:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	go generate ./internal/winmanifest
endif

generate-snmp:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
# Fetch snmp.yml file of the same version as the snmp_exporter go module, use sed to update the file we need to fetch in common.go:
	@LATEST_SNMP_VERSION=$$(go list -f '{{ .Version }}' -m github.com/prometheus/snmp_exporter); \
	sed -i "s|snmp_exporter/[^/]*/snmp.yml|snmp_exporter/$$LATEST_SNMP_VERSION/snmp.yml|" internal/static/integrations/snmp_exporter/common/common.go; \
	go generate ./internal/static/integrations/snmp_exporter/common; \
	sed -i "s/SNMP_VERSION: v[0-9]\+\.[0-9]\+\.[0-9]\+/SNMP_VERSION: $$LATEST_SNMP_VERSION/" docs/sources/_index.md.t
endif

generate-gh-issue-templates:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
# This script requires bash 4.0 or higher or zsh to work properly
	bash ./.github/ISSUE_TEMPLATE/scripts/update-gh-issue-templates.sh
endif

#
# Other targets
#
# build-container-cache and clean-build-container-cache are defined in
# Makefile.build-container.

.PHONY: clean
clean: clean-dist clean-build-container-cache
	rm -rf ./build/*

.PHONY: info
info:
	@printf "USE_CONTAINER       = $(USE_CONTAINER)\n"
	@printf "ALLOY_IMAGE         = $(ALLOY_IMAGE)\n"
	@printf "ALLOY_IMAGE_WINDOWS = $(ALLOY_IMAGE_WINDOWS)\n"
	@printf "BUILD_IMAGE         = $(BUILD_IMAGE)\n"
	@printf "ALLOY_BINARY        = $(ALLOY_BINARY)\n"
	@printf "GOOS                = $(GOOS)\n"
	@printf "GOARCH              = $(GOARCH)\n"
	@printf "GOARM               = $(GOARM)\n"
	@printf "CGO_ENABLED         = $(CGO_ENABLED)\n"
	@printf "RELEASE_BUILD       = $(RELEASE_BUILD)\n"
	@printf "VERSION             = $(VERSION)\n"
	@printf "GO_TAGS             = $(GO_TAGS)\n"
	@printf "GOEXPERIMENT        = $(GOEXPERIMENT)\n"

# awk magic to print out the comment block at the top of this file.
.PHONY: help
help:
	@awk 'BEGIN {FS="## "} /^##\s*(.*)/ { print $$2 }' $(MAKEFILE_LIST)
