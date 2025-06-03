---
canonical: https://grafana.com/docs/alloy/latest/concepts/configuration-syntax/files/
aliases:
  - ../../concepts/configuration-syntax/files/ # /docs/alloy/latest/concepts/configuration-syntax/files/
description: Learn about Alloy configuration files
title: Configuration files
weight: 100
---

# Configuration files

{{< param "PRODUCT_NAME" >}} configuration files are plain text files with a `.alloy` file extension.
Refer to each {{< param "PRODUCT_NAME" >}} file as a "configuration file" or an "{{< param "PRODUCT_NAME" >}} configuration."

{{< param "PRODUCT_NAME" >}} configuration files must be UTF-8 encoded and support Unicode characters.
They can use Unix-style line endings (LF) or Windows-style line endings (CRLF).
Formatters may replace all line endings with Unix-style ones.
