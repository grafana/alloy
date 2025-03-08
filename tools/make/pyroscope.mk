OTEL_PROFILER_ARCH ?=
OTEL_PROFILER_GIT_REF ?= $(shell grep -E "github.com/grafana/opentelemetry-ebpf-profiler" go.mod | awk '{print $$NF}' | sed 's/.*-//g')
OTEL_PROFILER_DIR ?= opentelemetry-ebpf-profiler

.PHONY: pyroscope-dependencies
pyroscope-dependencies: otel-profiler-symblib

.PHONY: otel-profiler-symblib
otel-profiler-symblib:
	rm -rf $(OTEL_PROFILER_DIR) target
	git clone https://github.com/grafana/opentelemetry-ebpf-profiler.git $(OTEL_PROFILER_DIR)  && \
		cd $(OTEL_PROFILER_DIR) && \
		git checkout $(OTEL_PROFILER_GIT_REF) && \
		rm -rf .cargo/config.toml # use viceroy
ifeq ($(OTEL_PROFILER_ARCH),amd64)
	TARGET_ARCH=amd64 make -C $(OTEL_PROFILER_DIR) rust-components
	mv $(OTEL_PROFILER_DIR)/target ./target
else ifeq ($(OTEL_PROFILER_ARCH),arm64)
	TARGET_ARCH=arm64 make -C $(OTEL_PROFILER_DIR) rust-components
	mv $(OTEL_PROFILER_DIR)/target ./target
else ifeq ($(OTEL_PROFILER_ARCH),both) # not really an arch, just allow build both supported arches for dist/* packaging
	TARGET_ARCH=amd64 make -C $(OTEL_PROFILER_DIR) rust-components
	TARGET_ARCH=arm64 make -C $(OTEL_PROFILER_DIR) rust-components
	mv $(OTEL_PROFILER_DIR)/target target
else
	@echo skipping unsupported arch $(OTEL_PROFILER_ARCH)
	mkdir -p target
endif
	rm -rf $(OTEL_PROFILER_DIR)

.PHONY: dist/pyroscope-dependencies
dist/pyroscope-dependencies:
	$(MAKE) pyroscope-dependencies \
    	OTEL_PROFILER_ARCH=both
