# Size-aware scrape allocation demo

## 1. Skewed workload in kind

Use the existing `example/kind` setup: one node, three clustered Alloy instances,
three 800-series targets and 24 ten-series targets. Every Alloy performs discovery.
Send metrics to the private Grafana Cloud stack using gitignored credentials.

Run `task cluster:up` and `task deploy:vanilla` from `example/kind`.
Edit checked-in YAML to change the workload; see the
[demo README](example/kind/config/target-allocation/README.md) for preliminary observations.

## 2. Vanilla Target Allocator

Run `task deploy:ta-hashing`: build TA from `~/workspace/opentelemetry-operator`,
load it into kind, and deploy one standalone TA using consistent hashing.
TA discovers targets; Alloy consumes assignments through `discovery.http`, with
scrape clustering disabled. No Operator is installed. Common target manifests are
shared between modes. `task deploy:vanilla` switches back.

## 3. Size-aware allocation

Start with pure simulations of the current TA algorithm; compare balance and
assignment stability before implementing adjustments. Requirements and candidate
algorithms live in [ALGO.md](ALGO.md). `task deploy:ta-packing` is a TODO.

HA is optional later work, not a demo requirement. If added, use Kubernetes Lease
leader election, likely with three TA instances. Routing and failover state
preservation remain undecided. Keep one TA for now.

## 4. Demo

Show the skew, switch allocation, and compare load and target movement.
Stretch goal: demonstrate savings in a dev cluster.
Stop local workloads with `task cluster:down`.
