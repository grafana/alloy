.ONESHELL:
.DELETE_ON_ERROR:
export SHELL     := bash
export SHELLOPTS := pipefail:errexit
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rule

CHECK_CLOUDWATCH_INTEGRATION ?= 1

include docs.mk

ifeq ($(CHECK_CLOUDWATCH_INTEGRATION),1)
.PHONY: docs
docs: check-cloudwatch-integration
endif

check-cloudwatch-integration:
	$(PODMAN) run -v "$(shell git rev-parse --show-toplevel):/repo:z" -v "$(shell pwd):/docs:z" -w /repo golang:1.25.6-bullseye go run internal/static/integrations/cloudwatch_exporter/docs/doc.go check /docs/sources/reference/components/prometheus/prometheus.exporter.cloudwatch.md

generate-cloudwatch-integration:
	$(PODMAN) run -v "$(shell git rev-parse --show-toplevel):/repo:z" -v "$(shell pwd):/docs:z" -w /repo golang:1.25.6-bullseye go run internal/static/integrations/cloudwatch_exporter/docs/doc.go generate
