# Makefile.packaging adds release-packaging-specific targets.

PARENT_MAKEFILE := $(firstword $(MAKEFILE_LIST))

.PHONY: dist clean-dist
dist: dist-alloy-binaries              \
      dist-alloy-boringcrypto-binaries \
      dist-alloy-packages              \
      dist-alloy-installer-windows

clean-dist:
	rm -rf ./dist/* ./dist.temp/*

# Used for passing through environment variables to sub-makes.
#
# NOTE(rfratto): This *must* use `=` instead of `:=` so it's expanded at
# reference time. Earlier iterations of this file had each target explicitly
# list these, but it's too easy to forget to set on so this is used to ensure
# everything needed is always passed through.
PACKAGING_VARS = RELEASE_BUILD=1 GO_TAGS="$(GO_TAGS)" GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) GOEXPERIMENT=$(GOEXPERIMENT)

.PHONY: dist-alloy-mixin-zip
dist-alloy-mixin-zip:
	"mkdir" -p dist
	cd operations/alloy-mixin/rendered && zip "../../../dist/alloy-mixin-dashboards-$${RELEASE_TAG:-$(VERSION)}.zip" dashboards/*.json

#
# Alloy release binaries
#

dist-alloy-binaries: dist/alloy-linux-amd64                    \
                     dist/alloy-linux-arm64                    \
                     dist/alloy-linux-ppc64le                  \
                     dist/alloy-linux-s390x                    \
                     dist/alloy-darwin-amd64                   \
                     dist/alloy-darwin-arm64                   \
                     dist/alloy-windows-amd64.exe              \
                     dist/alloy-freebsd-amd64

dist/alloy-linux-amd64: GO_TAGS += netgo embedalloyui promtail_journal_enabled
dist/alloy-linux-amd64: GOOS    := linux
dist/alloy-linux-amd64: GOARCH  := amd64
dist/alloy-linux-amd64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

dist/alloy-linux-arm64: GO_TAGS += netgo embedalloyui promtail_journal_enabled
dist/alloy-linux-arm64: GOOS    := linux
dist/alloy-linux-arm64: GOARCH  := arm64
dist/alloy-linux-arm64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

dist/alloy-linux-ppc64le: GO_TAGS += netgo embedalloyui promtail_journal_enabled
dist/alloy-linux-ppc64le: GOOS    := linux
dist/alloy-linux-ppc64le: GOARCH  := ppc64le
dist/alloy-linux-ppc64le: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

dist/alloy-linux-s390x: GO_TAGS += netgo embedalloyui promtail_journal_enabled
dist/alloy-linux-s390x: GOOS    := linux
dist/alloy-linux-s390x: GOARCH  := s390x
dist/alloy-linux-s390x: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

dist/alloy-darwin-amd64: GO_TAGS += netgo embedalloyui
dist/alloy-darwin-amd64: GOOS    := darwin
dist/alloy-darwin-amd64: GOARCH  := amd64
dist/alloy-darwin-amd64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

dist/alloy-darwin-arm64: GO_TAGS += netgo embedalloyui
dist/alloy-darwin-arm64: GOOS    := darwin
dist/alloy-darwin-arm64: GOARCH  := arm64
dist/alloy-darwin-arm64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

# NOTE(rfratto): do not use netgo when building Windows binaries, which
# prevents DNS short names from being resovable. See grafana/agent#4665.
#
# TODO(rfratto): add netgo back to Windows builds if a version of Go is
# released which natively supports resolving DNS short names on Windows.
dist/alloy-windows-amd64.exe: GO_TAGS += embedalloyui
dist/alloy-windows-amd64.exe: GOOS    := windows
dist/alloy-windows-amd64.exe: GOARCH  := amd64
dist/alloy-windows-amd64.exe: generate-ui generate-winmanifest
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

# NOTE(rfratto): do not use netgo when building Windows binaries, which
# prevents DNS short names from being resovable. See grafana/agent#4665.
#
# TODO(rfratto): add netgo back to Windows builds if a version of Go is
# released which natively supports resolving DNS short names on Windows.
dist/alloy-freebsd-amd64: GO_TAGS += netgo embedalloyui
dist/alloy-freebsd-amd64: GOOS    := freebsd
dist/alloy-freebsd-amd64: GOARCH  := amd64
dist/alloy-freebsd-amd64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

#
# Alloy boringcrypto release binaries
#

dist-alloy-boringcrypto-binaries: dist/alloy-boringcrypto-linux-amd64 \
                                  dist/alloy-boringcrypto-linux-arm64

dist/alloy-boringcrypto-linux-amd64: GO_TAGS      += netgo embedalloyui promtail_journal_enabled
dist/alloy-boringcrypto-linux-amd64: GOEXPERIMENT := boringcrypto
dist/alloy-boringcrypto-linux-amd64: GOOS         := linux
dist/alloy-boringcrypto-linux-amd64: GOARCH       := amd64
dist/alloy-boringcrypto-linux-amd64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

dist/alloy-boringcrypto-linux-arm64: GO_TAGS      += netgo embedalloyui promtail_journal_enabled
dist/alloy-boringcrypto-linux-arm64: GOEXPERIMENT := boringcrypto
dist/alloy-boringcrypto-linux-arm64: GOOS         := linux
dist/alloy-boringcrypto-linux-arm64: GOARCH       := arm64
dist/alloy-boringcrypto-linux-arm64: generate-ui
	$(PACKAGING_VARS) ALLOY_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy

#
# alloy-service release binaries.
#
# alloy-service release binaries are intermediate build assets used for
# producing Windows system packages. As such, they are built in a dist.temp
# directory instead of the normal dist directory.
#
# Only targets needed for system packages are used here.
#

dist-alloy-service-binaries: dist.temp/alloy-service-windows-amd64.exe

dist.temp/alloy-service-windows-amd64.exe: GO_TAGS += embedalloyui
dist.temp/alloy-service-windows-amd64.exe: GOOS    := windows
dist.temp/alloy-service-windows-amd64.exe: GOARCH  := amd64
dist.temp/alloy-service-windows-amd64.exe: generate-ui generate-winmanifest
	$(PACKAGING_VARS) SERVICE_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) alloy-service

#
# DEB and RPM alloy packages.
#

ALLOY_ENVIRONMENT_FILE_rpm := /etc/sysconfig/alloy
ALLOY_ENVIRONMENT_FILE_deb := /etc/default/alloy

# generate_alloy_fpm(deb|rpm, package arch, Alloy arch, output file)
define generate_alloy_fpm =
	fpm -s dir -v $(ALLOY_PACKAGE_VERSION) -a $(2) \
		-n alloy --iteration $(ALLOY_PACKAGE_RELEASE) -f \
		--log error \
		--license "Apache 2.0" \
		--vendor "Grafana Labs" \
		--url "https://github.com/grafana/alloy" \
		--description "Grafana Alloy is an OpenTelemetry Collector distribution with programmable pipelines." \
		--rpm-digest sha256 \
		-t $(1) \
		--after-install packaging/$(1)/control/postinst \
		--before-remove packaging/$(1)/control/prerm \
		--config-files /etc/alloy/config.alloy \
		--config-files $(ALLOY_ENVIRONMENT_FILE_$(1)) \
		--rpm-rpmbuild-define "_build_id_links none" \
		--package $(4) \
			dist/alloy-linux-$(3)=/usr/bin/alloy \
			packaging/config.alloy=/etc/alloy/config.alloy \
			packaging/environment-file=$(ALLOY_ENVIRONMENT_FILE_$(1)) \
			packaging/$(1)/alloy.service=/usr/lib/systemd/system/alloy.service
endef

ALLOY_PACKAGE_VERSION := $(patsubst v%,%,$(VERSION))
ALLOY_PACKAGE_RELEASE := 1
ALLOY_PACKAGE_PREFIX  := dist/alloy-$(ALLOY_PACKAGE_VERSION)-$(ALLOY_PACKAGE_RELEASE)

.PHONY: dist-alloy-packages
dist-alloy-packages: dist-alloy-packages-amd64   \
                     dist-alloy-packages-arm64   \
                     dist-alloy-packages-ppc64le \
                     dist-alloy-packages-s390x

.PHONY: dist-alloy-packages-amd64
dist-alloy-packages-amd64: dist/alloy-linux-amd64
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_alloy_fpm,deb,amd64,amd64,$(ALLOY_PACKAGE_PREFIX).amd64.deb)
	$(call generate_alloy_fpm,rpm,x86_64,amd64,$(ALLOY_PACKAGE_PREFIX).amd64.rpm)
endif

.PHONY: dist-alloy-packages-arm64
dist-alloy-packages-arm64: dist/alloy-linux-arm64
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_alloy_fpm,deb,arm64,arm64,$(ALLOY_PACKAGE_PREFIX).arm64.deb)
	$(call generate_alloy_fpm,rpm,aarch64,arm64,$(ALLOY_PACKAGE_PREFIX).arm64.rpm)
endif

.PHONY: dist-alloy-packages-ppc64le
dist-alloy-packages-ppc64le: dist/alloy-linux-ppc64le
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_alloy_fpm,deb,ppc64el,ppc64le,$(ALLOY_PACKAGE_PREFIX).ppc64el.deb)
	$(call generate_alloy_fpm,rpm,ppc64le,ppc64le,$(ALLOY_PACKAGE_PREFIX).ppc64le.rpm)
endif

.PHONY: dist-alloy-packages-s390x
dist-alloy-packages-s390x: dist/alloy-linux-s390x
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_alloy_fpm,deb,s390x,s390x,$(ALLOY_PACKAGE_PREFIX).s390x.deb)
	$(call generate_alloy_fpm,rpm,s390x,s390x,$(ALLOY_PACKAGE_PREFIX).s390x.rpm)
endif

#
# Windows installer
#

.PHONY: dist-alloy-installer-windows
dist-alloy-installer-windows: dist/alloy-windows-amd64.exe dist.temp/alloy-service-windows-amd64.exe
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	# quotes around mkdir are mandatory. ref: https://github.com/grafana/agent/pull/5664#discussion_r1378796371
	"mkdir" -p dist
	makensis -V4 -DVERSION=$(VERSION) -DOUT="../../dist/alloy-installer-windows-amd64.exe" ./packaging/windows/install_script.nsis
endif
