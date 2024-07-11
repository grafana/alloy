.ONESHELL:
.DELETE_ON_ERROR:
export SHELL     := bash
export SHELLOPTS := pipefail:errexit
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rule

include docs.mk

.PHONY: docs
docs: check-cloudwatch-integration

check-cloudwatch-integration:
	$(PODMAN) run -v $(shell git rev-parse --show-toplevel):/repo -v $(shell pwd):/docs -w /repo golang:1.22.5-bullseye go run internal/static/integrations/cloudwatch_exporter/docs/doc.go check /docs/sources/reference/components/prometheus/prometheus.exporter.cloudwatch.md

generate-cloudwatch-integration:
	$(PODMAN) run -v $(shell git rev-parse --show-toplevel):/repo -v $(shell pwd):/docs -w /repo golang:1.22.5-bullseye go run internal/static/integrations/cloudwatch_exporter/docs/doc.go generate
