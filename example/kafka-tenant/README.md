# Tenant-affine Kafka demo

Local end-to-end demo of `kafka.tenant_producer` and `kafka.tenant_consumer`.

```
loadgen ──► nginx ──► producer-1 ─┐                 ┌─► consumer-1 ─┐
                  └─► producer-2 ─┴─► Redpanda ─────┤               ├─► sink (fake Mimir/Loki/Pyroscope/OTLP)
                                  (1 topic,         └─► consumer-2 ─┘
                                   1 partition/tenant)
```

- `config/tenants.yaml` maps four tenants to the four partitions of `alloy-telemetry`.
- The `topic` service creates the topic. The components never create topics.
- Every writer in `config/consumer.alloy` adds `X-Alloy-Consumer`, so the sink can show which consumer delivered each tenant's data.
- `scripts/loadgen.py` sends Prometheus remote write, Loki push, Pyroscope `/ingest`, and OTLP/HTTP metrics, logs, and traces for every tenant.
  It then checks that the sink received every signal of a tenant from exactly one consumer, with the right `X-Scope-OrgID`.

## Run

Build a Linux Alloy binary for your Docker architecture:

```sh
cd collector
GOOS=linux GOARCH=$(docker version -f '{{.Server.Arch}}') CGO_ENABLED=0 go build -o ../build/alloy-linux .
cd ..
```

Start the stack and run the load generator:

```sh
cd example/kafka-tenant
docker compose up -d
docker compose run --rm loadgen
```

Expected output ends with one line per tenant, for example:

```
OK   tenant-a: all signals delivered by consumer-2
OK   tenant-b: all signals delivered by consumer-1
OK   tenant-c: all signals delivered by consumer-2
OK   tenant-d: all signals delivered by consumer-1
```

## Failover

Stop one consumer. After its session times out (`session_timeout = "10s"`), its partitions move whole to the other consumer:

```sh
docker compose stop consumer-1
sleep 15
docker compose run --rm loadgen   # every tenant is now delivered by consumer-2
docker compose start consumer-1
```

## Inspect

- Sink counts by tenant, consumer, and path: `curl localhost:9999/`
- Consumer metrics: `curl -s localhost:12348/metrics | grep kafka_tenant_consumer`
- Producer metrics: `curl -s localhost:12346/metrics | grep kafka_tenant_producer`

## Clean up

```sh
docker compose down -v
```
