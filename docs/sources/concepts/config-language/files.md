---
aliases:
- ../configuration-language/files/ # /docs/agent/latest/flow/concepts/configuration-language/files/
- /docs/grafana-cloud/agent/flow/concepts/config-language/files/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/files/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/files/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/files/
# Previous page aliases for backwards compatibility:
- ../../configuration-language/files/ # /docs/agent/latest/flow/configuration-language/files/
- /docs/grafana-cloud/agent/flow/config-language/files/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/files/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/files/
- /docs/grafana-cloud/send-data/agent/flow/config-language/files/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/files/
description: Learn about River files
title: Files
weight: 100
---

# Files

River files are plain text files with the `.river` file extension.
You can refer to each River file as a "configuration file" or a "River configuration."

River files must be UTF-8 encoded and can contain Unicode characters.
River files can use Unix-style line endings (LF) and Windows-style line endings (CRLF), but formatters may replace all line endings with Unix-style ones.
