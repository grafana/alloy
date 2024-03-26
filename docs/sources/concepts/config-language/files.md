---
canonical: https://grafana.com/docs/alloy/latest/concepts/config-language/files/
description: Learn about River files
title: Files
weight: 100
---

# Files

River files are plain text files with the `.alloy` file extension.
You can refer to each {{< param "PRODUCT_NAME" >}} file as a "configuration file" or an "{{< param "PRODUCT_NAME" >}} configuration."

{{< param "PRODUCT_NAME" >}} configuration files must be UTF-8 encoded and can contain Unicode characters.
{{< param "PRODUCT_NAME" >}} configuration files can use Unix-style line endings (LF) and Windows-style line endings (CRLF), but formatters may replace all line endings with Unix-style ones.
