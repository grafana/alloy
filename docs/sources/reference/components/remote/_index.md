---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote/
description: Learn about the remote components in Grafana Alloy
review_date: 2026-09-11
labels:
  products:
    - oss
title: remote
weight: 100
---

# `remote`

The `remote` components retrieve content from sources outside of your {{< param "PRODUCT_NAME" >}} configuration, such as files in object storage, secrets in HashiCorp Vault, and Kubernetes ConfigMaps and Secrets, and expose that content to other components.

Use a `remote` component when a value your configuration depends on, for example, a credential or a list of scrape targets, is managed somewhere else and can change independently of your configuration.

{{< section >}}
