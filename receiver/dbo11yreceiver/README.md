# db-o11y receiver (PoC)

The `dbo11y` receiver runs Database Observability for PostgreSQL inside the collector, without the Alloy runtime. It's the collector counterpart of `database_observability.postgres` and shares its per-database code (`internal/component/database_observability/postgres/dbinstance`), so `server_id` is computed in exactly one place.

Status: development. This is a proof of concept.

## Configuration

```yaml
receivers:
  dbo11y:
    databases:
      - data_source_name: postgres://user@db:5432/postgres?sslmode=disable
        # name: optional, prefixes collector names in health messages
        # cloud_provider: { aws: { arn: ... } }
    # Defaults match database_observability.postgres.
    enable_collectors: [table_stats]
    disable_collectors: []
    exclude_databases: [...]
    exclude_users: [...]
    exclude_current_user: true
    query_samples: { collect_interval: 10s }
    query_details: { collect_interval: 1m, statements_limit: 100 }
    schema_details: { collect_interval: 1m }
    explain_plans: { collect_interval: 1m, per_collect_ratio: 1.0 }
    health_check: { collect_interval: 1h }
    logs: { enable_error_logs_processing: false }
    prometheus_exporter:
      disabled: false
      enabled_collectors: [stat_statements]
    scrape_interval: 15s
    reconnect_interval: 30s

service:
  pipelines:
    metrics: { receivers: [dbo11y], exporters: [otlphttp] }
    logs: { receivers: [dbo11y], exporters: [otlphttp] }
```

Put the same receiver ID on a metrics and a logs pipeline. Both pipelines then share one connection and one `server_id` per database.

## Output

Each database gets its own resource with `service.name="integrations/db-o11y"` and `service.instance.id=<postgresql://host:port/dbname>`. Prometheus OTLP ingest turns those back into `job` and `instance`.

- Metrics: the database's Prometheus registry (db-o11y metrics plus the embedded postgres_exporter) is gathered every `scrape_interval`. `server_id` is a resource attribute, so Prometheus OTLP ingest puts it in `target_info` unless it's promoted. The cloud provider labels are datapoint attributes, the way a scrape adds target labels today. There's no `up` or `scrape_*` series, since nothing is scraped.
- Logs: one log record per db-o11y entry. The line is the body. Every former Loki label except `server_id` (`op`, `job`, `instance`, `engine`) is a log record attribute. `server_id` is a resource attribute, so Loki needs a `resource_attributes` entry for it. Loki doesn't rebuild `job`/`instance` from `service.*`, so a tenant `otlp_config` has to promote the attributes the app needs.

The receiver's own health goes to the collector's status reporting (recoverable error while a database is unreachable or has stopped collectors) and its own counters (`dbo11y_receiver_connect_errors`, `dbo11y_receiver_gather_errors`, `dbo11y_receiver_consume_errors`) to the collector's telemetry.

## Limitations

- **Embedded exporter off doesn't correlate.** With `prometheus_exporter.disabled: true` no `pg_*` metrics are produced. A separate postgres_exporter receiver can supply them, but its series won't carry this receiver's `server_id`: the collector has no way to hand relabeled targets between components the way the Alloy component does with `targets`. The app won't correlate those `pg_*` series with db-o11y data.
- **No inbound error logs.** The Alloy component exports `logs_receiver` and parses Postgres error logs pushed into it. The receiver doesn't. Two ways to add it:
  - A second input on this receiver, for example a `logs_receiver` extension or an OTLP endpoint that only this receiver reads. It keeps one owner per database, since the parser needs the database's connection and `server_id`, but it makes a receiver also accept data, which is unusual in the collector.
  - A separate processor or connector that parses error logs coming from `filelog` or similar. It's the OTel-shaped option, but it needs the `server_id` of the database the log came from, so it would either look it up from this receiver (cross-component state the collector doesn't offer) or compute it again (a second `server_id` owner, which isn't allowed).
  This PoC leaves it open.
- **No per-database `rule` blocks.** The relabel rules of `database_instance` blocks aren't configurable yet.
- No clustering.
