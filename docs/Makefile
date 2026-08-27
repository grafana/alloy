.ONESHELL:
.DELETE_ON_ERROR:
export SHELL     := bash
export SHELLOPTS := pipefail:errexit
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rule

SYNC_CLOUDWATCH_DOCS ?= 1

include docs.mk

ifeq ($(SYNC_CLOUDWATCH_DOCS),1)
.PHONY: docs
docs: sync-cloudwatch-docs
endif

.PHONY: sync-cloudwatch-docs
sync-cloudwatch-docs:
	task --dir "$(shell git rev-parse --show-toplevel)" docs:cloudwatch-sync
