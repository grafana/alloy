# Benchmark tool

Benchmark tool is created by `go build .` to generate the executable. Benchmark expects two environment vars PROM_USERNAME and PROM_PASSWORD to be set to write to grafana dashboard.

It can be called via `./benchmark metrics --name=churn_good_network --duration=4h --type=churn --benchmarks=normal,queue` that will start 4 processes. Note benchmark and alloy-test are always created and any number of benchmarks can be run that each spawn an alloy instance.

* The benchmark tool itself that opens up a black hole metrics receiver on port 8888
  * Exposes 2 metrics 
    * `benchmark_series_received` How many metrics received from each test. These should roughly be the same for each benchmark. They vary a bit on timing and at the end should be the same.
    * `benchmark_errors` How many errors are generated from each test. Generally this should be 0.
* Alloy in test mode, churn represents how many series are replaced with new ones during a metrics refresh. This is very similar to Avalanche. The config is test.river
  * Exposes a set of prometheus.test.metrics instances for a variety of tests
    * single
      * 1 instance with 2000 metrics and churns metrics every 10m changing 5% of them
    * many
      * 1000 instances with 1000 metrics and churns metrics every 10m changing 5% of them
    * large
      * 2 instances of 1,000,000 metrics and churns metrics every 10m changing 5% of them
    * churn
      * 2 instances of 200,000 metrics and churns metrics every 2m changing 50% of them
* normal
  * This starts an alloy instance on an open port and pulls metrics from alloy test
* queue
  * This starts an alloy instance on an open port and pulls metrics from alloy test

Benchmarks are defined in benchmark.json and can be added / adjusted as needed. Configs are generally configured to write memory,cpu and important stats to a central dashboard for review. They generally have two writes, one to black hole for all the metrics and one to the dashboard.


## Flags

* name - The name of the benchmark to run, this will be added to the exported metrics
* duration - The duration to run the test for. Accepts go time strings
* type - The type of metrics to use; single,man,churn,large or if you have added any to test.river they can be referenced.
* benchmarks - List of benchmarks to run. Run `benchmark list` to list all possible benchmarks. Comma delimited
* network-down - If set to true, the network will be down for the duration of the test.