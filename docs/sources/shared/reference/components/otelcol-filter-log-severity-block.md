---
aliases:
- /docs/agent/shared/flow/reference/components/otelcol-filter-log-severity-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/otelcol-filter-log-severity-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/otelcol-filter-log-severity-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/otelcol-filter-log-severity-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/otelcol-filter-log-severity-block/
description: Shared content, otelcol filter log severity block
headless: true
---

This block defines how to match based on a log record's SeverityNumber field.

The following arguments are supported:

Name              | Type     | Description                                   | Default | Required
------------------|----------|-----------------------------------------------|---------|---------
`match_undefined` | `bool`   | Whether logs with "undefined" severity match. |         | yes
`min`             | `string` | The lowest severity that may be matched.      |         | yes

If `match_undefined` is true, entries with undefined severity will match.

The following table lists the severities supported by OTel.
The value for `min` should be one of the values in the "Log Severity" column.

Log Severity | Severity number
------------ | ---------------
TRACE        | 1
TRACE2       | 2
TRACE3       | 3
TRACE4       | 4
DEBUG        | 5
DEBUG2       | 6
DEBUG3       | 7
DEBUG4       | 8
INFO         | 9
INFO2        | 10
INFO3        | 11
INFO4        | 12
WARN         | 13
WARN2        | 14
WARN3        | 15
WARN4        | 16
ERROR        | 17
ERROR2       | 18
ERROR3       | 19
ERROR4       | 20
FATAL        | 21
FATAL2       | 22
FATAL3       | 23
FATAL4       | 24

For example, if the `min` attribute in the `log_severity` block is "INFO", then INFO, WARN, ERROR, and FATAL logs will match.
