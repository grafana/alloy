# Benchmark notes

These are synthetic benchmarks meant to represent common workloads. These are not meant to be exhaustive or fine-grained.
These will give a coarse idea of how the agent behaves in a situations.

## Prometheus Metrics

### Running the benchmarks

Running `PROM_USERNAME="" PROM_PASSWORD="" ./benchmark.sh` will start the benchmark and run for 8 hours. The duration and type of tests
can be adjusted by editing the `metris.sh` file. This will start two Agents and the benchmark runner. Relevant CPU and memory metrics
will be sent to the endpoint described in `normal.river`.

TODO: Add mixin for graph I am using

### Adjusting the benchmark

Each benchmark can be adjusted within `test.river`. These settings allow fine tuning to a specific scenario. Each `prometheus.test.metric` component
exposes a service discovery URL that is used to collect the targets.

### Benchmark categories

#### prometheus.test.metrics "single"

This roughly represents a single node exporter and is the simpliest use case. Every `10m` 5% of the metrics are replaced driven by `churn_percent`.

#### prometheus.test.metrics "many"

This roughly represents scraping many node_exporter instances in say a Kubernetes environment.

#### prometheus.test.metrics "large"

This represents scraping 2 very large instances with 1,000,000 series.

#### prometheus.test.metrics "churn"

This represents a worst case scenario, 2 large instances with an extremely high churn rate.

### Adjusting the tests

`prometheus.relabel` is often a CPU bottleneck so adding additional rules allows you to test the impact of that.

### Rules

There are existing rules to only send to the prometheus remote write the specific metrics that matter. These are tagged with the `runtype` and the benchmark. For instance `normal-large`.

The benchmark starts an endpoint to consume the metrics from `prometheus.test.metrics`, in half the tests it will return HTTP Status 200 and in the other half will return 500.

TODO add optional pyroscope profiles


## Loki Logs