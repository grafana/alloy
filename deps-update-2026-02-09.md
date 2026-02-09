# Prometheus Dependency Update — 2026-02-09

Upgraded `github.com/prometheus/prometheus` from v0.308.0 (v3.8.0) to v0.309.1 (v3.9.1) across all modules. The other Prometheus client libraries were already at their latest compatible versions.

## API changes in Prometheus v0.309.1

Prometheus renamed "Created Timestamp" to "Start Timestamp" throughout the `storage.Appender` interface:

- `AppendCTZeroSample` → `AppendSTZeroSample`
- `AppendHistogramCTZeroSample` → `AppendHistogramSTZeroSample`
- `CreatedTimestampAppender` → `StartTimestampAppender`
- `Commit()`/`Rollback()` moved into a new embedded `AppenderTransaction` interface (no code changes needed for this)

All Alloy implementations of `storage.Appender` were updated to match.

## Fork of otel-contrib prometheusreceiver

The upstream `receiver/prometheusreceiver` at v0.142.0 implements `storage.Appender` with the old method names. The fix landed upstream in otel-contrib v0.144.0, but that requires OTel Collector v1.50.0 which we aren't on yet.

To bridge the gap, I created the `release/142-prometheus-0.309` branch on `grafana/opentelemetry-collector-contrib` with the minimal rename applied on top of v0.142.0. A replace directive points to that branch. This replace should be removed during the next OTel Collector upgrade (v0.144.0+).

## walqueue update

`github.com/grafana/walqueue` was updated to a newer commit (92af63e, merged upstream 2026-01-22) that implements the renamed `storage.Appender` methods.

## featuregate replace removed

The `go.opentelemetry.io/collector/featuregate` replace directive (and the `otelfeaturegatefix` blank imports) were removed. The upstream issue (prometheus/prometheus#13842) was resolved in 2024 and the fork was no longer needed.

## Test fix

One converter test expectation was updated — hash-based endpoint names in `prom_remote_write.alloy` changed due to the Prometheus config struct changes. This is cosmetic and not user-facing.
