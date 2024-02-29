---
aliases:
- ../../../../configuration/integrations/integrations-next/snmp-config/
- /docs/grafana-cloud/monitor-infrastructure/agent/static/configuration/integrations/integrations-next/snmp-config/
- /docs/grafana-cloud/send-data/agent/static/configuration/integrations/integrations-next/snmp-config/
canonical: https://grafana.com/docs/agent/latest/static/configuration/integrations/integrations-next/snmp-config/
description: Learn about snmp config next
title: snmp config next
---

# snmp config next

The `snmp` block configures the `snmp` integration,
which is an embedded version of
[`snmp_exporter`](https://github.com/prometheus/snmp_exporter). This allows collection of SNMP metrics from the network devices with ease.


## Quick configuration example

To get started, define SNMP targets in Grafana agent's integration block:

```yaml
metrics:
  wal_directory: /tmp/wal
integrations:
  snmp:
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

If you need to scrape SNMP devices in more dynamic environment, and cannot define devices in `snmp_targets` because targets would change over time, you can use service discovery approach. For instance, with [DNS discovery](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#dns_sd_config):

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
    autoscrape:
      enable: false # set autoscrape to off
    walk_params:
      private:
        retries: 2
```


Full reference of options:

```yaml
  # Provide an explicit value to uniquely identify this instance of the
  # integration. If not provided, a reasonable default will be inferred based
  # on the integration.
  #
  # The value here must be unique across all instances of the same integration.
  [instance: <string>]

  # Override autoscrape defaults for this integration.
  autoscrape:
    # Enables autoscrape of integrations.
    [enable: <boolean> | default = <integrations.metrics.autoscrape.enable>]

    # Specifies the metrics instance name to send metrics to.
    [metrics_instance: <string> | default = <integrations.metrics.autoscrape.metrics_instance>]

    # Autoscrape interval and timeout.
    [scrape_interval: <duration> | default = <integrations.metrics.autoscrape.scrape_interval>]
    [scrape_timeout: <duration> | default = <integrations.metrics.autoscrape.scrape_timeout>]

  # An optional extra set of labels to add to metrics from the integration target. These
  # labels are only exposed via the integration service discovery HTTP API and
  # added when autoscrape is used. They will not be found directly on the metrics
  # page for an integration.
  extra_labels:
    [ <labelname>: <labelvalue> ... ]

  #
  # Exporter-specific configuration options
  #

  # SNMP configuration file with custom modules.
  # See https://github.com/prometheus/snmp_exporter#generating-configuration for more details how to generate custom snmp.yml file.
  # If not defined, embedded snmp_exporter default set of modules is used.
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
