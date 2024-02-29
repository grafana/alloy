---
aliases:
- ../../../configuration/integrations/snmp-config/
- /docs/grafana-cloud/monitor-infrastructure/agent/static/configuration/integrations/snmp-config/
- /docs/grafana-cloud/send-data/agent/static/configuration/integrations/snmp-config/
canonical: https://grafana.com/docs/agent/latest/static/configuration/integrations/snmp-config/
description: Learn about snmp config
title: snmp config
---

# snmp config

The `snmp` block configures the `snmp` integration,
which is an embedded version of
[`snmp_exporter`](https://github.com/prometheus/snmp_exporter). This allows collection of SNMP metrics from the network devices with ease.

{{< admonition type="note" >}}
`snmp config` uses the latest configuration introduced in version 0.23 of the Prometheus `snmp_exporter`.
{{< /admonition >}}

## Quick configuration example

To get started, define SNMP targets in Grafana agent's integration block:

```yaml
metrics:
  wal_directory: /tmp/wal
integrations:
  snmp:
    enabled: true
    snmp_targets:
      - name: network_switch_1
        address: 192.168.1.2
        module: if_mib
        walk_params: public
        auth: public
      - name: network_router_2
        address: 192.168.1.3
        module: mikrotik
        walk_params: private
        auth: private
    walk_params:
      private:
        retries: 2
      public:
        retries: 1
```

## Prometheus service discovery use case

If you need to scrape SNMP devices in more dynamic environment, and can't define devices in `snmp_targets` because targets would change over time, you can use service discovery approach. For instance, with [DNS discovery](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#dns_sd_config):

```yaml

metrics:
  wal_directory: /tmp/wal
  configs:
    - name: snmp_targets
      scrape_configs:
        - job_name: 'snmp'
          dns_sd_configs:
            - names:
              - switches.srv.example.org
              - routers.srv.example.org
          params:
            module: [if_mib]
            walk_params: [private]
            auth: [private]
          metrics_path: /integrations/snmp/metrics
          relabel_configs:
            - source_labels: [__address__]
              target_label: __param_target
            - source_labels: [__param_target]
              target_label: instance
            - replacement: 127.0.0.1:12345 # address must match grafana agent -server.http.address flag
              target_label: __address__
integrations:
  snmp:
    enabled: true
    scrape_integration: false # set autoscrape to off
    walk_params:
      private:
        retries: 2
```


Full reference of options:

```yaml
  # Enables the snmp integration, allowing the Agent to automatically
  # collect metrics for the specified github objects.
  [enabled: <boolean> | default = false]

  # Sets an explicit value for the instance label when the integration is
  # self-scraped. Overrides inferred values.
  #
  # The default value for this integration is inferred from the hostname portion
  # of api_url.
  [instance: <string>]

  # Automatically collect metrics from this integration. If disabled,
  # the snmp integration will be run but not scraped and thus not
  # remote-written. Metrics for the integration will be exposed at
  # /integrations/snmp/metrics and can be scraped by an external
  # process.
  [scrape_integration: <boolean> | default = <integrations_config.scrape_integrations>]

  # How often should the metrics be collected? Defaults to
  # prometheus.global.scrape_interval.
  [scrape_interval: <duration> | default = <global_config.scrape_interval>]

  # The timeout before considering the scrape a failure. Defaults to
  # prometheus.global.scrape_timeout.
  [scrape_timeout: <duration> | default = <global_config.scrape_timeout>]

  # Allows for relabeling labels on the target.
  relabel_configs:
    [- <relabel_config> ... ]

  # Relabel metrics coming from the integration, allowing to drop series
  # from the integration that you don't care about.
  metric_relabel_configs:
    [ - <relabel_config> ... ]

  # How frequent to truncate the WAL for this integration.
  [wal_truncate_frequency: <duration> | default = "60m"]

  #
  # Exporter-specific configuration options
  #

  # SNMP configuration file with custom modules.
  # See https://github.com/prometheus/snmp_exporter#generating-configuration for more details how to generate custom snmp.yml file.
  [config_file: <string> | default = ""]

  # Embedded SNMP configuration. You can specify your modules here instead of an external config file.
  # See https://github.com/prometheus/snmp_exporter/tree/main#generating-configuration for more details how to specify your SNMP modules.
  # If this and config_file are not defined, embedded snmp_exporter default set of modules is used.
  snmp_config:
    [- <modules> ... ]
    [- <auths> ... ]

  # List of SNMP targets to poll
  snmp_targets:
    [- <snmp_target> ... ]

  # Map of SNMP connection profiles that can be used to override default SNMP settings.
  walk_params:
    [ <string>: <walk_param> ... ]


```
## snmp_target config

```yaml
  # Name of a snmp_target
  [name: <string>]

  # The address of SNMP device
  [address: <string>]

  # SNMP module to use for polling
  [module: <string> | default = ""]

  # SNMP authentication profile to use
  [auth: <string> | default = ""]

  # walk_param config to use for this snmp_target
  [walk_params: <string> | default = ""]
```

## walk_param config

```yaml
  # How many objects to request with GET/GETBULK, defaults to 25.
  # May need to be reduced for buggy devices.
  [max_repetitions: <int> | default = 25]

  # How many times to retry a failed request, defaults to 3.
  [retries: <int> | default = 3]

  # Timeout for each SNMP request, defaults to 5s.
  [timeout: <duration> | default = 5s]
```


## About SNMP modules

SNMP module is the set of SNMP counters to be scraped together from the specific network device.

SNMP modules available can be found in the embedded snmp.yml file [here](https://github.com/grafana/agent/blob/main/internal/static/integrations/snmp_exporter/common/snmp.yml). If not specified, `if_mib` module is used.

If you need to use custom SNMP modules, you can [generate](https://github.com/prometheus/snmp_exporter#generating-configuration) your own snmp.yml file and specify it using `config_file` parameter.
