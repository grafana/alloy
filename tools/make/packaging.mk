# Makefile.packaging adds release-packaging-specific targets.

PARENT_MAKEFILE := $(firstword $(MAKEFILE_LIST))

.PHONY: dist clean-dist
dist: dist-agent-binaries dist-agent-flow-binaries dist-agentctl-binaries dist-agent-packages dist-agent-flow-packages dist-agent-installer dist-agent-flow-installer

clean-dist:
	rm -rf ./dist/* ./dist.temp/*

# Used for passing through environment variables to sub-makes.
#
# NOTE(rfratto): This *must* use `=` instead of `:=` so it's expanded at
# reference time. Earlier iterations of this file had each target explicitly
# list these, but it's too easy to forget to set on so this is used to ensure
# everything needed is always passed through.
PACKAGING_VARS = RELEASE_BUILD=1 GO_TAGS="$(GO_TAGS)" GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) GOEXPERIMENT=$(GOEXPERIMENT)

#
# agent release binaries
#

dist-agent-binaries: dist/grafana-agent-linux-amd64                    \
                     dist/grafana-agent-linux-arm64                    \
                     dist/grafana-agent-linux-ppc64le                  \
                     dist/grafana-agent-linux-s390x                    \
                     dist/grafana-agent-darwin-amd64                   \
                     dist/grafana-agent-darwin-arm64                   \
                     dist/grafana-agent-windows-amd64.exe              \
                     dist/grafana-agent-windows-boringcrypto-amd64.exe \
                     dist/grafana-agent-freebsd-amd64                  \
                     dist/grafana-agent-linux-arm64-boringcrypto

dist/grafana-agent-linux-amd64: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist/grafana-agent-linux-amd64: GOOS    := linux
dist/grafana-agent-linux-amd64: GOARCH  := amd64
dist/grafana-agent-linux-amd64: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

dist/grafana-agent-linux-arm64: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist/grafana-agent-linux-arm64: GOOS    := linux
dist/grafana-agent-linux-arm64: GOARCH  := arm64
dist/grafana-agent-linux-arm64: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

dist/grafana-agent-linux-ppc64le: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist/grafana-agent-linux-ppc64le: GOOS    := linux
dist/grafana-agent-linux-ppc64le: GOARCH  := ppc64le
dist/grafana-agent-linux-ppc64le: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

dist/grafana-agent-linux-s390x: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist/grafana-agent-linux-s390x: GOOS    := linux
dist/grafana-agent-linux-s390x: GOARCH  := s390x
dist/grafana-agent-linux-s390x: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

dist/grafana-agent-darwin-amd64: GO_TAGS += netgo builtinassets
dist/grafana-agent-darwin-amd64: GOOS    := darwin
dist/grafana-agent-darwin-amd64: GOARCH  := amd64
dist/grafana-agent-darwin-amd64: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

dist/grafana-agent-darwin-arm64: GO_TAGS += netgo builtinassets
dist/grafana-agent-darwin-arm64: GOOS    := darwin
dist/grafana-agent-darwin-arm64: GOARCH  := arm64
dist/grafana-agent-darwin-arm64: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

# NOTE(rfratto): do not use netgo when building Windows binaries, which
# prevents DNS short names from being resovable. See grafana/agent#4665.
#
# TODO(rfratto): add netgo back to Windows builds if a version of Go is
# released which natively supports resolving DNS short names on Windows.
dist/grafana-agent-windows-amd64.exe: GO_TAGS += builtinassets
dist/grafana-agent-windows-amd64.exe: GOOS    := windows
dist/grafana-agent-windows-amd64.exe: GOARCH  := amd64
dist/grafana-agent-windows-amd64.exe: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

# NOTE(rfratto): do not use netgo when building Windows binaries, which
# prevents DNS short names from being resovable. See grafana/agent#4665.
#
# TODO(rfratto): add netgo back to Windows builds if a version of Go is
# released which natively supports resolving DNS short names on Windows.
dist/grafana-agent-windows-boringcrypto-amd64.exe: GO_TAGS += builtinassets
dist/grafana-agent-windows-boringcrypto-amd64.exe: GOOS    := windows
dist/grafana-agent-windows-boringcrypto-amd64.exe: GOARCH  := amd64
dist/grafana-agent-windows-boringcrypto-amd64.exe: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent


dist/grafana-agent-freebsd-amd64: GO_TAGS += netgo builtinassets
dist/grafana-agent-freebsd-amd64: GOOS    := freebsd
dist/grafana-agent-freebsd-amd64: GOARCH  := amd64
dist/grafana-agent-freebsd-amd64: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent


dist/grafana-agent-linux-amd64-boringcrypto: GO_TAGS      += netgo builtinassets promtail_journal_enabled
dist/grafana-agent-linux-amd64-boringcrypto: GOOS         := linux
dist/grafana-agent-linux-amd64-boringcrypto: GOARCH       := amd64
dist/grafana-agent-linux-amd64-boringcrypto: GOEXPERIMENT := boringcrypto
dist/grafana-agent-linux-amd64-boringcrypto: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

dist/grafana-agent-linux-arm64-boringcrypto: GO_TAGS      += netgo builtinassets promtail_journal_enabled
dist/grafana-agent-linux-arm64-boringcrypto: GOOS         := linux
dist/grafana-agent-linux-arm64-boringcrypto: GOARCH       := arm64
dist/grafana-agent-linux-arm64-boringcrypto: GOEXPERIMENT := boringcrypto
dist/grafana-agent-linux-arm64-boringcrypto: generate-ui
	$(PACKAGING_VARS) AGENT_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent

#
# agentctl release binaries.
#

dist-agentctl-binaries: dist/grafana-agentctl-linux-amd64       \
                        dist/grafana-agentctl-linux-arm64       \
                        dist/grafana-agentctl-linux-ppc64le     \
                        dist/grafana-agentctl-linux-s390x       \
                        dist/grafana-agentctl-darwin-amd64      \
                        dist/grafana-agentctl-darwin-arm64      \
                        dist/grafana-agentctl-windows-amd64.exe \
                        dist/grafana-agentctl-freebsd-amd64

dist/grafana-agentctl-linux-amd64: GO_TAGS += netgo promtail_journal_enabled
dist/grafana-agentctl-linux-amd64: GOOS    := linux
dist/grafana-agentctl-linux-amd64: GOARCH  := amd64
dist/grafana-agentctl-linux-amd64:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-linux-arm64: GO_TAGS += netgo promtail_journal_enabled
dist/grafana-agentctl-linux-arm64: GOOS    := linux
dist/grafana-agentctl-linux-arm64: GOARCH  := arm64
dist/grafana-agentctl-linux-arm64:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-linux-ppc64le: GO_TAGS += netgo promtail_journal_enabled
dist/grafana-agentctl-linux-ppc64le: GOOS    := linux
dist/grafana-agentctl-linux-ppc64le: GOARCH  := ppc64le
dist/grafana-agentctl-linux-ppc64le:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-linux-s390x: GO_TAGS += netgo promtail_journal_enabled
dist/grafana-agentctl-linux-s390x: GOOS    := linux
dist/grafana-agentctl-linux-s390x: GOARCH  := s390x
dist/grafana-agentctl-linux-s390x:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-darwin-amd64: GO_TAGS += netgo
dist/grafana-agentctl-darwin-amd64: GOOS    := darwin
dist/grafana-agentctl-darwin-amd64: GOARCH  := amd64
dist/grafana-agentctl-darwin-amd64:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-darwin-arm64: GO_TAGS += netgo
dist/grafana-agentctl-darwin-arm64: GOOS    := darwin
dist/grafana-agentctl-darwin-arm64: GOARCH  := arm64
dist/grafana-agentctl-darwin-arm64:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-windows-amd64.exe: GOOS   := windows
dist/grafana-agentctl-windows-amd64.exe: GOARCH := amd64
dist/grafana-agentctl-windows-amd64.exe:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

dist/grafana-agentctl-freebsd-amd64: GO_TAGS += netgo
dist/grafana-agentctl-freebsd-amd64: GOOS    := freebsd
dist/grafana-agentctl-freebsd-amd64: GOARCH  := amd64
dist/grafana-agentctl-freebsd-amd64:
	$(PACKAGING_VARS) AGENTCTL_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agentctl

#
# agent-flow release binaries
#
# agent-flow release binaries are intermediate build assets used for producing
# Flow-specific system packages. As such, they are built in a dist.temp
# directory instead of the normal dist directory.
#
# Only targets needed for system packages are used here.
#

dist-agent-flow-binaries: dist.temp/grafana-agent-flow-linux-amd64       \
                          dist.temp/grafana-agent-flow-linux-arm64       \
                          dist.temp/grafana-agent-flow-linux-ppc64le     \
                          dist.temp/grafana-agent-flow-linux-s390x       \
                          dist.temp/grafana-agent-flow-windows-amd64.exe

dist.temp/grafana-agent-flow-linux-amd64: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist.temp/grafana-agent-flow-linux-amd64: GOOS    := linux
dist.temp/grafana-agent-flow-linux-amd64: GOARCH  := amd64
dist.temp/grafana-agent-flow-linux-amd64: generate-ui
	$(PACKAGING_VARS) FLOW_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent-flow

dist.temp/grafana-agent-flow-linux-arm64: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist.temp/grafana-agent-flow-linux-arm64: GOOS    := linux
dist.temp/grafana-agent-flow-linux-arm64: GOARCH  := arm64
dist.temp/grafana-agent-flow-linux-arm64: generate-ui
	$(PACKAGING_VARS) FLOW_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent-flow

dist.temp/grafana-agent-flow-linux-ppc64le: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist.temp/grafana-agent-flow-linux-ppc64le: GOOS    := linux
dist.temp/grafana-agent-flow-linux-ppc64le: GOARCH  := ppc64le
dist.temp/grafana-agent-flow-linux-ppc64le: generate-ui
	$(PACKAGING_VARS) FLOW_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent-flow

dist.temp/grafana-agent-flow-linux-s390x: GO_TAGS += netgo builtinassets promtail_journal_enabled
dist.temp/grafana-agent-flow-linux-s390x: GOOS    := linux
dist.temp/grafana-agent-flow-linux-s390x: GOARCH  := s390x
dist.temp/grafana-agent-flow-linux-s390x: generate-ui
	$(PACKAGING_VARS) FLOW_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent-flow

dist.temp/grafana-agent-flow-windows-amd64.exe: GO_TAGS += builtinassets
dist.temp/grafana-agent-flow-windows-amd64.exe: GOOS    := windows
dist.temp/grafana-agent-flow-windows-amd64.exe: GOARCH  := amd64
dist.temp/grafana-agent-flow-windows-amd64.exe: generate-ui
	$(PACKAGING_VARS) FLOW_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent-flow

#
# agent-service release binaries.
#
# agent-service release binaries are intermediate build assets used for
# producing Flow-specific system packages. As such, they are built in a
# dist.temp directory instead of the normal dist directory.
#
# Only targets needed for system packages are used here.
#

dist-agent-service-binaries: dist.temp/grafana-agent-service-windows-amd64.exe

dist.temp/grafana-agent-service-windows-amd64.exe: GO_TAGS += builtinassets
dist.temp/grafana-agent-service-windows-amd64.exe: GOOS    := windows
dist.temp/grafana-agent-service-windows-amd64.exe: GOARCH  := amd64
dist.temp/grafana-agent-service-windows-amd64.exe: generate-ui
	$(PACKAGING_VARS) SERVICE_BINARY=$@ "$(MAKE)" -f $(PARENT_MAKEFILE) agent-service


#
# DEB and RPM grafana-agent packages.
#

AGENT_ENVIRONMENT_FILE_rpm := /etc/sysconfig/grafana-agent
AGENT_ENVIRONMENT_FILE_deb := /etc/default/grafana-agent

# generate_agent_fpm(deb|rpm, package arch, agent arch, output file)
define generate_agent_fpm =
	fpm -s dir -v $(AGENT_PACKAGE_VERSION) -a $(2) \
		-n grafana-agent --iteration $(AGENT_PACKAGE_RELEASE) -f \
		--log error \
		--license "Apache 2.0" \
		--vendor "Grafana Labs" \
		--url "https://github.com/grafana/agent" \
		--rpm-digest sha256 \
		-t $(1) \
		--after-install packaging/grafana-agent/$(1)/control/postinst \
		--before-remove packaging/grafana-agent/$(1)/control/prerm \
		--config-files /etc/grafana-agent.yaml \
		--config-files $(AGENT_ENVIRONMENT_FILE_$(1)) \
		--rpm-rpmbuild-define "_build_id_links none" \
		--package $(4) \
			dist/grafana-agent-linux-$(3)=/usr/bin/grafana-agent \
			dist/grafana-agentctl-linux-$(3)=/usr/bin/grafana-agentctl \
			packaging/grafana-agent/grafana-agent.yaml=/etc/grafana-agent.yaml \
			packaging/grafana-agent/environment-file=$(AGENT_ENVIRONMENT_FILE_$(1)) \
			packaging/grafana-agent/$(1)/grafana-agent.service=/usr/lib/systemd/system/grafana-agent.service
endef

AGENT_PACKAGE_VERSION := $(patsubst v%,%,$(VERSION))
AGENT_PACKAGE_RELEASE := 1
AGENT_PACKAGE_PREFIX  := dist/grafana-agent-$(AGENT_PACKAGE_VERSION)-$(AGENT_PACKAGE_RELEASE)

.PHONY: dist-agent-packages
dist-agent-packages: dist-agent-packages-amd64   \
                     dist-agent-packages-arm64   \
                     dist-agent-packages-ppc64le \
                     dist-agent-packages-s390x

.PHONY: dist-agent-packages-amd64
dist-agent-packages-amd64: dist/grafana-agent-linux-amd64 dist/grafana-agentctl-linux-amd64
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_agent_fpm,deb,amd64,amd64,$(AGENT_PACKAGE_PREFIX).amd64.deb)
	$(call generate_agent_fpm,rpm,x86_64,amd64,$(AGENT_PACKAGE_PREFIX).amd64.rpm)
endif

.PHONY: dist-agent-packages-arm64
dist-agent-packages-arm64: dist/grafana-agent-linux-arm64 dist/grafana-agentctl-linux-arm64
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_agent_fpm,deb,arm64,arm64,$(AGENT_PACKAGE_PREFIX).arm64.deb)
	$(call generate_agent_fpm,rpm,aarch64,arm64,$(AGENT_PACKAGE_PREFIX).arm64.rpm)
endif

.PHONY: dist-agent-packages-ppc64le
dist-agent-packages-ppc64le: dist/grafana-agent-linux-ppc64le dist/grafana-agentctl-linux-ppc64le
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_agent_fpm,deb,ppc64el,ppc64le,$(AGENT_PACKAGE_PREFIX).ppc64el.deb)
	$(call generate_agent_fpm,rpm,ppc64le,ppc64le,$(AGENT_PACKAGE_PREFIX).ppc64le.rpm)
endif

.PHONY: dist-agent-packages-s390x
dist-agent-packages-s390x: dist/grafana-agent-linux-s390x dist/grafana-agentctl-linux-s390x
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_agent_fpm,deb,s390x,s390x,$(AGENT_PACKAGE_PREFIX).s390x.deb)
	$(call generate_agent_fpm,rpm,s390x,s390x,$(AGENT_PACKAGE_PREFIX).s390x.rpm)
endif

#
# DEB and RPM grafana-agent-flow packages.
#

FLOW_ENVIRONMENT_FILE_rpm := /etc/sysconfig/grafana-agent-flow
FLOW_ENVIRONMENT_FILE_deb := /etc/default/grafana-agent-flow

# generate_flow_fpm(deb|rpm, package arch, agent arch, output file)
define generate_flow_fpm =
	fpm -s dir -v $(FLOW_PACKAGE_VERSION) -a $(2) \
		-n grafana-agent-flow --iteration $(FLOW_PACKAGE_RELEASE) -f \
		--log error \
		--license "Apache 2.0" \
		--vendor "Grafana Labs" \
		--url "https://github.com/grafana/agent" \
		--rpm-digest sha256 \
		-t $(1) \
		--after-install packaging/grafana-agent-flow/$(1)/control/postinst \
		--before-remove packaging/grafana-agent-flow/$(1)/control/prerm \
		--config-files /etc/grafana-agent-flow.river \
		--config-files $(FLOW_ENVIRONMENT_FILE_$(1)) \
		--rpm-rpmbuild-define "_build_id_links none" \
		--package $(4) \
			dist.temp/grafana-agent-flow-linux-$(3)=/usr/bin/grafana-agent-flow \
			packaging/grafana-agent-flow/grafana-agent-flow.river=/etc/grafana-agent-flow.river \
			packaging/grafana-agent-flow/environment-file=$(FLOW_ENVIRONMENT_FILE_$(1)) \
			packaging/grafana-agent-flow/$(1)/grafana-agent-flow.service=/usr/lib/systemd/system/grafana-agent-flow.service
endef

FLOW_PACKAGE_VERSION := $(patsubst v%,%,$(VERSION))
FLOW_PACKAGE_RELEASE := 1
FLOW_PACKAGE_PREFIX  := dist/grafana-agent-flow-$(AGENT_PACKAGE_VERSION)-$(AGENT_PACKAGE_RELEASE)

.PHONY: dist-agent-flow-packages
dist-agent-flow-packages: dist-agent-flow-packages-amd64   \
                          dist-agent-flow-packages-arm64   \
                          dist-agent-flow-packages-ppc64le \
                          dist-agent-flow-packages-s390x

.PHONY: dist-agent-flow-packages-amd64
dist-agent-flow-packages-amd64: dist.temp/grafana-agent-flow-linux-amd64
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_flow_fpm,deb,amd64,amd64,$(FLOW_PACKAGE_PREFIX).amd64.deb)
	$(call generate_flow_fpm,rpm,x86_64,amd64,$(FLOW_PACKAGE_PREFIX).amd64.rpm)
endif

.PHONY: dist-agent-flow-packages-arm64
dist-agent-flow-packages-arm64: dist.temp/grafana-agent-flow-linux-arm64
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_flow_fpm,deb,arm64,arm64,$(FLOW_PACKAGE_PREFIX).arm64.deb)
	$(call generate_flow_fpm,rpm,aarch64,arm64,$(FLOW_PACKAGE_PREFIX).arm64.rpm)
endif

.PHONY: dist-agent-flow-packages-ppc64le
dist-agent-flow-packages-ppc64le: dist.temp/grafana-agent-flow-linux-ppc64le
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_flow_fpm,deb,ppc64el,ppc64le,$(FLOW_PACKAGE_PREFIX).ppc64el.deb)
	$(call generate_flow_fpm,rpm,ppc64le,ppc64le,$(FLOW_PACKAGE_PREFIX).ppc64le.rpm)
endif

.PHONY: dist-agent-flow-packages-s390x
dist-agent-flow-packages-s390x: dist.temp/grafana-agent-flow-linux-s390x
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	$(call generate_flow_fpm,deb,s390x,s390x,$(FLOW_PACKAGE_PREFIX).s390x.deb)
	$(call generate_flow_fpm,rpm,s390x,s390x,$(FLOW_PACKAGE_PREFIX).s390x.rpm)
endif

#
# Windows installer
#

# TODO(rfratto): update the install_script.nsis so we don't need to copy assets
# over into the packaging/windows folder.
.PHONY: dist-agent-installer
dist-agent-installer: dist/grafana-agent-windows-amd64.exe
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	cp ./dist/grafana-agent-windows-amd64.exe ./packaging/grafana-agent/windows
	cp LICENSE ./packaging/grafana-agent/windows
	# quotes around mkdir are manadory. ref: https://github.com/grafana/agent/pull/5664#discussion_r1378796371
	"mkdir" -p dist
	makensis -V4 -DVERSION=$(VERSION) -DOUT="../../../dist/grafana-agent-installer.exe" ./packaging/grafana-agent/windows/install_script.nsis
endif

.PHONY: dist-agent-flow-installer
dist-agent-flow-installer: dist.temp/grafana-agent-flow-windows-amd64.exe dist.temp/grafana-agent-service-windows-amd64.exe
ifeq ($(USE_CONTAINER),1)
	$(RERUN_IN_CONTAINER)
else
	# quotes around mkdir are manadory. ref: https://github.com/grafana/agent/pull/5664#discussion_r1378796371
	"mkdir" -p dist
	makensis -V4 -DVERSION=$(VERSION) -DOUT="../../../dist/grafana-agent-flow-installer.exe" ./packaging/grafana-agent-flow/windows/install_script.nsis
endif
