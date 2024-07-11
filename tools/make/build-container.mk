# build-container.mk contains utilities used by Makefiles to proxy commands to
# build containers.
#
# The following terminology is used throughout comments:
#
#   callers  Makefiles including this file
#   users    Users invoking `make`
#
# Users can supply the USE_CONTAINER=1 environment variable to inform targets
# that they should proxy build commands to the build container.
#
# Targets must opt-in to proxying support. Callers which wish to support
# proxying a Make target to the build container should be written with the
# following pattern:
#
#   <make target>:
#   ifeq $($(USE_CONTAINER),1)
#     $(RERUN_IN_CONTAINER)
#   else
#     <logic>
#   endif
#
# For example, for a `example` target to proxy an echo to a build container:
#
#   example:
#   ifeq ($(USE_CONTAINER),1)
#     $(RERUN_IN_CONTAINER)
#   else
#     echo Hello, world!
#   endif
#
# By default, no environment variables are propagated to the build container.
# Callers should set the PROPAGATE_VARS global variable to specify which
# variable names should be passed through to the container.

USE_CONTAINER       ?= 0
BUILD_IMAGE_VERSION ?= v0.1.3
BUILD_IMAGE         ?= grafana/alloy-build-image:$(BUILD_IMAGE_VERSION)
DOCKER_OPTS         ?= -it

#
# Build container cache. `make build-container-cache` will create two Docker
# volumes which are used for caching downloaded Go modules and built code.
# This dramatically speeds up subseqent builds.
#
# The following code checks to see if the volumes exist and appends them to
# DOCKER_OPTS if they do.
#

GO_CACHE_VOLUME    := alloy-build-container-gocache
GO_MODCACHE_VOLUME := alloy-build-container-gomodcache

define volume_exists
$(shell docker volume inspect $(1) >/dev/null 2>&1 && echo 1 || echo "")
endef

# Figure out if our build container should be using a build cache for Go.
ifneq ($(USE_CONTAINER),0)
GO_CACHE_VOLUME_EXISTS    := $(call volume_exists,$(GO_CACHE_VOLUME))
GO_MODCACHE_VOLUME_EXISTS := $(call volume_exists,$(GO_MODCACHE_VOLUME))
GO_CACHE_EXISTS           := $(and $(GO_CACHE_VOLUME_EXISTS),$(GO_MODCACHE_VOLUME_EXISTS))

ifeq ($(GO_CACHE_EXISTS),1)
DOCKER_OPTS += -v $(GO_CACHE_VOLUME):/root/.cache/go-build -v $(GO_MODCACHE_VOLUME):/go/pkg/mod
endif
endif

.PHONY: build-container-cache
build-container-cache:
	docker volume create $(GO_CACHE_VOLUME)
	docker volume create $(GO_MODCACHE_VOLUME)

clean-build-container-cache:
ifneq (, $(shell which docker))
	docker volume rm $(GO_CACHE_VOLUME) 2>/dev/null || true
	docker volume rm $(GO_MODCACHE_VOLUME) 2>/dev/null || true
endif

#
# Miscellaneous
#

# Name of the makefile
PARENT_MAKEFILE := $(firstword $(MAKEFILE_LIST))

# Creates a build container and runs Make inside of it.
#
# Callers can use PROPAGATE_VARS to set which environment variables get passed
# through to the container. USE_CONTAINER should never get propagated.
define RERUN_IN_CONTAINER
	docker run $(DOCKER_OPTS) --init --rm $(DOCKER_OPTS)       \
		-e "CC=viceroycc"                                        \
		-v "$(shell pwd):/src"                                   \
		-w "/src"                                                \
		-v "/var/run/docker.sock:/var/run/docker.sock"           \
		$(foreach var, $(PROPAGATE_VARS), -e "$(var)=$(subst ",\",$($(var)))") \
		$(BUILD_IMAGE) make -f $(PARENT_MAKEFILE) $@
endef
