---
aliases:
- /docs/agent/shared/flow/reference/components/http-client-proxy-config-description-args/
- /docs/grafana-cloud/agent/shared/flow/reference/components/http-client-proxy-config-description-args/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/http-client-proxy-config-description-args/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/http-client-proxy-config-description-args/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/http-client-proxy-config-description-args/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/http-client-proxy-config-description-args/
description: Shared content, http client config description
headless: true
---

`no_proxy` can contain IPs, CIDR notations, and domain names. IP and domain
names can contain port numbers. `proxy_url` must be configured if `no_proxy`
is configured.

`proxy_from_environment` uses the environment variables HTTP_PROXY, HTTPS_PROXY
and NO_PROXY (or the lowercase versions thereof). Requests use the proxy from
the environment variable matching their scheme, unless excluded by NO_PROXY.
`proxy_url` and `no_proxy` must not be configured if `proxy_from_environment`
is configured.

`proxy_connect_header` should only be configured if `proxy_url` or `proxy_from_environment` are configured.