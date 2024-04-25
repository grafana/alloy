## Build, test, and generate code for various parts of Alloy.
##
## At least Go 1.19, git, and a moderately recent version of Docker is required
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
##   test              Run tests
##   lint              Lint code
##   integration-test  Run integration tests
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
##   dist-packages        Produce release-ready DEB and RPM packages.
##   dist-alloy-installer Produce a Windows installer for Alloy.
##
## Targets for generating assets:
##
##   generate                  Generate everything.
##   generate-drone            Generate the Drone YAML from Jsonnet.
##   generate-helm-docs        Generate Helm chart documentation.
##   generate-helm-tests       Generate Helm chart tests.
##   generate-ui               Generate the UI assets.
##   generate-versioned-files  Generate versioned files.
##   generate-winmanifest      Generate the Windows application manifest.
##
## Other targets:
##
##   build-container-cache  Create a cache for the build container to speed up
##                          subsequent proxied builds
##   drone                  Sign Drone CI config (maintainers only)
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
ALLOY_IMAGE_WINDOWS  ?= grafana/alloy:nanoserver-1809
ALLOY_BINARY         ?= build/alloy
SERVICE_BINARY       ?= build/alloy-service
ALLOYLINT_BINARY     ?= build/alloylint
GOOS                 ?= $(shell go env GOOS)
GOARCH               ?= $(shell go env GOARCH)
GOARM                ?= $(shell go env GOARM)
CGO_ENABLED          ?= 1
RELEASE_BUILD        ?= 0
GOEXPERIMENT         ?= $(shell go env GOEXPERIMENT)

# List of all environment variables which will propagate to the build
# container. USE_CONTAINER must _not_ be included to avoid infinite recursion.
PROPAGATE_VARS := \
    ALLOY_IMAGE ALLOY_IMAGE_WINDOWS \
    BUILD_IMAGE GOOS GOARCH GOARM CGO_ENABLED RELEASE_BUILD \
    ALLOY_BINARY \
    VERSION GO_TAGS GOEXPERIMENT

#
# Constants for targets
#

GO_ENV := GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED)

VERSION      ?= $(shell bash ./tools/image-tag)
GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH   := $(shell git rev-parse --abbrev-ref HEAD)
VPREFIX      := github.com/grafana/alloy/internal/build
GO_LDFLAGS   := -X $(VPREFIX).Branch=$(GIT_BRANCH)                        \
                -X $(VPREFIX).Version=$(VERSION)                          \
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
	find . -name go.mod -execdir golangci-lint run -v --timeout=10m \;
	$(ALLOYLINT_BINARY) ./...

.PHONY: test
# We have to run test twice: once for all packages with -race and then once
# more without -race for packages that have known race detection issues. The
# final command runs tests for all other submodules.
test:
	$(GO_ENV) go test $(GO_FLAGS) -race $(shell go list ./... | grep -v /integration-tests/)
	$(GO_ENV) go test $(GO_FLAGS) ./internal/static/integrations/node_exporter ./internal/static/logs ./internal/component/otelcol/processor/tail_sampling ./internal/component/loki/source/file ./internal/component/loki/source/docker
	$(GO_ENV) find . -name go.mod -not -path "./go.mod" -execdir go test -race ./... \;

test-packages:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	docker pull $(BUILD_IMAGE)
	go test -tags=packaging  ./internal/tools/packaging_test
endif

.PHONY: integration-test
integration-test:
	cd internal/cmd/integration-tests && $(GO_ENV) go run .

#
# Targets for building binaries
#

.PHONY: binaries alloy
binaries: alloy

alloy:
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

.PHONY: generate generate-drone generate-helm-docs generate-helm-tests generate-ui generate-versioned-files generate-winmanifest
generate: generate-drone generate-helm-docs generate-helm-tests generate-ui generate-versioned-files generate-docs generate-winmanifest

generate-drone:
	drone jsonnet -V BUILD_IMAGE_VERSION=$(BUILD_IMAGE_VERSION) --stream --format --source .drone/drone.jsonnet --target .drone/drone.yml

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

generate-ui:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	cd ./internal/web/ui && yarn --network-timeout=1200000 && yarn run build
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
	go generate ./docs
endif

generate-winmanifest:
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	go generate ./internal/winmanifest
endif
#
# Other targets
#
# build-container-cache and clean-build-container-cache are defined in
# Makefile.build-container.

# Drone signs the yaml, you will need to specify DRONE_TOKEN, which can be
# found by logging into your profile in Drone.
#
# This will only work for maintainers.
.PHONY: drone
drone: generate-drone
	drone lint .drone/drone.yml --trusted
	drone --server https://drone.grafana.net sign --save grafana/alloy .drone/drone.yml

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
