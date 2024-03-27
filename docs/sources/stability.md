---
canonical: https://grafana.com/docs/alloy/latest/stability/
description: Alloy features fall into one of three stability categories, experimental, public preview, or generally available
title: Stability
weight: 600
_build:
  list: false
noindex: true
---

# Stability

Stability of functionality usually refers to the stability of a _use case,_ such as collecting and forwarding OpenTelemetry metrics.

Features within the {{< param "FULL_PRODUCT_NAME" >}} project will fall into one of three stability categories:

* **Experimental**: A new use case is being explored.
* **Public preview**: Functionality covering a use case is being matured.
* **Generally available**: Functionality covering a use case is believed to be stable.

The default stability is stable.
Features are explicitly marked as experimental or public preview if they aren't stable.

## Experimental

The **experimental** stability category is used to denote that maintainers are
exploring a new use case, and would like feedback.

* Experimental features are subject to frequent breaking changes.
* Experimental features can be removed with no equivalent replacement.
* Experimental features may require enabling feature flags to use.

Unless removed, experimental features eventually graduate to public preview.

## Public preview

The **public preview** stability category is used to denote a feature which is being matured.

* Beta features are subject to occasional breaking changes.
* Beta features can be replaced by equivalent functionality that covers the same use case.
* Beta features can be used without enabling feature flags.

Unless replaced with equivalent functionality, public preview features eventually graduate to generally available.

## Generally available

The **generally available** stability category is used to denote a feature as stable.

* Breaking changes to stable features are rare, and will be well-documented.
* If new functionality is introduced to replace existing stable functionality, deprecation and removal timeline will be well-documented.
* Stable features can be used without enabling feature flags.
