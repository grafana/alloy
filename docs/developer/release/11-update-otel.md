# Update Open Telemetry Contrib

Grafana Alloy is listed as a distribution of the OpenTelemetry Collector. If there are any new OTel components that Grafana Alloy needs to be associated with, then open a PR in [OpenTelemetry Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) and add Alloy to the list of distributions. [Example](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/653ab064bb797ed2b4ae599936a7b9cfdad18a29/receiver/kafkareceiver/README.md?plain=1#L7)

## Steps

1. Determine if there are any new OTEL components by looking at the changelog.

2. Create a PR in OpenTelemetry Contrib.

3. Find those OTEL components in contrib and add Grafana Alloy as a distribution.

4. Tag Juraci ([jpkrohling](https://github.com/jpkrohling)) on the PR.
