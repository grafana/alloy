# Prometheus Dependency Update — v0.311.2 (2026-04-13)

## Branch

`kgeckhart/prometheus-v0.311.2-upgrade` (branched from main)

## Versions

| Dependency | Before | After |
|---|---|---|
| `github.com/prometheus/prometheus` | `v0.309.2-0.20260113170727-c7bc56cf6c8f` + fork | `v0.311.2` |
| `github.com/grafana/loki/v3` | `v3.6.5` | `v3.7.1` |
| `github.com/grafana/prometheus` fork | `v1.8.2-0.20260313093229-87200e297b57` | **removed** |
| `github.com/grafana/memberlist` pin | `v0.3.1-0.20220714140823-09ffed8adbbe` | `v0.3.1-0.20251126142931-6f9f62ab6f86` |
| `github.com/charmbracelet/x/cellbuf` | `v0.0.13-0.20250311204145-2c3ea96c31dd` | `v0.0.15` |
| `github.com/open-telemetry/opentelemetry-collector-contrib` (full suite) | `v0.147.0` | `v0.151.0` (target; blocked — see Task 6) |

## Rationale for Loki bump

`tsdb/errors` was deleted from Prometheus in upstream PR [#17768](https://github.com/prometheus/prometheus/pull/17768) (merged Feb 4, 2026, part of every release ≥ v3.10.0). Loki v3.6.x still imports it. Loki v3.7.x does not. There is no version of Prometheus that contains both the `sent_batch_duration` fix (PR [#18214](https://github.com/prometheus/prometheus/pull/18214), merged Mar 3) and `tsdb/errors` — they are mutually exclusive. Loki v3.7.1 is a released version consistent with the pattern used in previous upgrades.

## Completed

- [x] Task 1 — Fork audit: PR #18214 is in v3.11.2; fork has exactly one commit (the upstream merge); fork dropped.
- [x] Task 2 — `dependency-replacements.yaml` updated, `go.mod` bumped, `make generate-module-dependencies` passes, `go mod tidy` clean.

## Remaining: Task 3 — Compilation errors

### Notes on planning doc vs reality

Several errors listed in the original plan were **incorrect** (based on stale gopls cache resolving to the old fork):
- `prometheus.NewFanout` does NOT need a new logger arg — signature unchanged (4 args).
- `scrape.NewManager` does NOT take `appendableV2` — new signature is still 5 args: `(opts, logger, jsonFn, appendable, registerer)`. Old call was already correct once types resolved.
- `scrapeConfig.ExtraScrapeMetrics` was NOT removed — field still exists.
- `w.series.GetOrSet` signature did NOT change.

### External dependency fixes — DONE

| Package | Fix |
|---|---|
| `github.com/grafana/memberlist` | ✅ Pin updated to `v0.3.1-0.20251126142931-6f9f62ab6f86` in `dependency-replacements.yaml` and `go.mod` |
| `github.com/charmbracelet/x/cellbuf` | ✅ Upgraded to stable `v0.0.15` (was pre-release using newer `ansi.Style` API than the pinned `ansi` version) |
| `github.com/open-telemetry/opentelemetry-collector-contrib` suite | ⏸ Blocked — see Task 6. Target is `v0.151.0` (not yet released as of 2026-04-16). v0.149.0 is incompatible (`pkg/translator/loki` still calls `promql_parser.ParseMetric` which was removed in Prometheus v0.311.2). v0.150.0 is incompatible (`prometheusreceiver`, `prometheusremotewriteexporter`, `prometheusexporter` require a post-tag Prometheus pre-release `v0.311.2-0.20260409145810-72293ff1d2e0` with a breaking `discovery.CreateAndRegisterSDMetrics` API change not in the released tag). Waiting for v0.151.0 to see if it targets a proper Prometheus release. |

### External dependency fixes — DONE (upstream PRs staged)

| Package | Fix |
|---|---|
| `github.com/prometheus-community/prom-label-proxy` v0.12.1 | ✅ Personal fork `kgeckhart/prom-label-proxy` at `v0.12.2-0.20260414165134-176fc4881167`. Fixes `parser.ParseExpr` → `parser.NewParser(parser.Options{}).ParseExpr` and `parser.ParseMetricSelector` → `parser.NewParser(parser.Options{}).ParseMetricSelector`. Also bumps prometheus v0.310.0 → v0.311.2. **Upstream:** [prometheus-community/prom-label-proxy#349](https://github.com/prometheus-community/prom-label-proxy/pull/349) (`philipgough:parser-opts`, opened 2026-04-08) covers the same fix (bumps to v0.311.1; uses cleaner Options pattern). Giving author time to update before acting — check back after 2026-04-18. Note: `--enable-promql-experimental-functions` / `--enable-promql-duration-expression-parsing` CLI flags are temporarily no-ops in our fork; #349 properly wires them via Options. |
| `github.com/prometheus-operator/prometheus-operator` v0.86.1 | ✅ Personal fork `kgeckhart/prometheus-operator` at `v0.86.2-0.20260414175915-c764d55e812e`. Fixes `parser.ParseExpr` in `namespacelabeler/labeler.go` and `rulefmt.Parse` in `pkg/operator/rules.go`. Bumps prometheus v0.310.0 → v0.311.2. **Upstream PR not yet opened** — hard dependency on prom-label-proxy merging and cutting a release first (prometheus-operator imports `prom-label-proxy/injectproxy` directly). No existing upstream PR as of 2026-04-16. |

### Alloy code fixes — DONE

| File | Fix |
|---|---|
| `internal/component/prometheus/scrape/scrape.go:366` | ✅ `scrape.NewManager` — added `nil` appendableV2 arg (new 6-arg signature confirmed by `go build`) |
| `internal/component/prometheus/relabel/relabel.go:270` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/relabel/relabel.go:254` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/api/routes.go:89` | ✅ `promql_parser.ParseMetric` → `promql_parser.NewParser(promql_parser.Options{}).ParseMetric()` |
| `internal/component/loki/source/api/routes.go:103` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/aws_firehose/internal/handler.go:181` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/azure_event_hubs/internal/parser/parser.go:203` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/docker/tailer.go:352` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/gcplog/internal/gcplogtarget/formatter.go:84` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/gelf/internal/target/gelftarget.go:127` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/heroku/routes.go:83` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/internal/kafkatarget/formatter.go:19` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/podlogs/reconciler.go:334` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/loki/source/syslog/internal/syslogtarget/syslogtarget.go:186,263,308` | ✅ `relabel.Process` → `relabel.ProcessBuilder` (3 instances) |
| `internal/static/agentctl/waltools/samples.go:37` | ✅ `parser.ParseMetricSelector` → `parser.NewParser(parser.Options{}).ParseMetricSelector()` |
| `internal/static/metrics/wal/wal.go:656` | ✅ `wlog.Checkpoint` — added `false` for new `enableSTStorage bool` arg |
| `internal/static/metrics/wal/wal.go` | ✅ Added `AppenderV2` panic stub to satisfy `storage.Storage` interface (used by `remote_write.go`) |
| `internal/mimir/client/types.go:49,50` | ✅ Import changed from `gopkg.in/yaml.v3` to `go.yaml.in/yaml/v3` |
| `internal/mimir/client/types.go:54` | ✅ `rulefmt.RuleNode.Validate` — added `parser.NewParser(parser.Options{})` arg |
| `internal/static/traces/promsdprocessor/prom_sd_processor.go:177` | ✅ `relabel.Process` → `relabel.ProcessBuilder` |
| `internal/component/mimir/rules/kubernetes/events.go:254` | ✅ `parser.ParseExpr` → `parser.NewParser(parser.Options{}).ParseExpr()` |
| `internal/component/loki/rules/kubernetes/events.go:227` | ✅ `rulefmt.Parse` — added `parser.NewParser(parser.Options{})` arg |
| `integration-tests/docker/utils.go` | ✅ Import changed from `docker/docker` to `moby/moby`; `ForListeningPort(string(natPort))` |
| `integration-tests/docker/utils.go` | ✅ `hc.PortBindings` rewritten to use `network.PortMap` / `network.MustParsePort` / `netip.MustParseAddr`; removed `nat` import |
| `internal/component/pyroscope/util/test/container/java.go` | ✅ Container import changed to `moby/moby`; `MappedPort` arg changed from `nat.Port("8080/tcp")` to `"8080/tcp"` |

### Alloy code fixes — REMAINING

| File | Error | Fix |
|---|---|---|
| `internal/component/prometheus/operator/common/crdmanager.go:194` | ✅ `scrape.NewManager` — added `nil` appendableV2 arg (same pattern as `scrape.go`; was masked by prom-label-proxy build failure) |
| `internal/cmd/alloy-service` | Windows-only package (`main_windows.go`); "function main is undeclared" on macOS is expected, not a real error | N/A — not a real error |

### Migration pattern for `relabel.Process` → `relabel.ProcessBuilder`

```go
// Before
relabelled, keep = relabel.Process(lbls.Copy(), cfgs...)

// After
lb := labels.NewBuilder(lbls)
keep = relabel.ProcessBuilder(lb, cfgs...)
relabelled = lb.Labels()
```

Note: if a `*labels.Builder` already exists at the call site, use it directly — no need to call `lb.Labels()` first to get the input.

### Migration pattern for `parser.ParseExpr` / `parser.ParseMetricSelector` / `parser.ParseMetric`

```go
// Before
expr, err := parser.ParseExpr(query)
selector, err := parser.ParseMetricSelector(selectorStr)
ls, err := promql_parser.ParseMetric(s)

// After
expr, err := parser.NewParser(parser.Options{}).ParseExpr(query)
selector, err := parser.NewParser(parser.Options{}).ParseMetricSelector(selectorStr)
ls, err := promql_parser.NewParser(promql_parser.Options{}).ParseMetric(s)
```

### prom-label-proxy fork plan

Two files need patching in `github.com/prometheus-community/prom-label-proxy`:
- `injectproxy/enforce.go:57`: `parser.ParseExpr(q)` → `parser.NewParser(parser.Options{}).ParseExpr(q)`
- `injectproxy/routes.go:671`: `parser.ParseMetricSelector(m)` → `parser.NewParser(parser.Options{}).ParseMetricSelector(m)`

Steps:
1. Create fork at `github.com/grafana/prom-label-proxy`
2. Apply the 2-line fix on a branch off `v0.12.1`
3. Add replace directive in `dependency-replacements.yaml` and `go.mod`

### testcontainers / moby API change context

The otel-contrib bump to v0.150.0 triggered a transitive upgrade of `testcontainers-go` from `v0.40.0` to `v0.42.0`. testcontainers-go v0.42.0 switched from `github.com/docker/docker` to `github.com/moby/moby/api`. The moby `network.PortMap` type is completely different from `nat.PortMap`:
- `nat.PortMap = map[nat.Port][]nat.PortBinding` where `nat.Port` is a `string` type
- `network.PortMap = map[network.Port][]network.PortBinding` where `network.Port` is a struct and `network.PortBinding.HostIP` is `netip.Addr`

## Task 4 — Test failures — DONE

### Test fixes applied

| File | Fix |
|---|---|
| `internal/component/loki/relabel/relabel.go:255` | ✅ `relabel.ProcessBuilder` `keep` return value was ignored — added early return of `nil` when `keep=false` (Drop action was not dropping entries) |
| `internal/component/common/kubernetes/prometheus_rule_group_diff_test.go:15` | ✅ `rulefmt.Parse` — added `parser.NewParser(parser.Options{})` arg (test file missed during Task 3) |
| `internal/component/prometheus/fanout_test.go` | ✅ Added `AppenderV2` panic stub to `noopStore` to satisfy `storage.Storage` interface |
| `internal/component/prometheus/pipeline_test.go` | ✅ Added `AppenderV2` panic stub to `testStorage` to satisfy `storage.Storage` interface |
| `internal/static/agentctl/waltools/walstats_test.go:96` | ✅ `wlog.Checkpoint` — added `false` for new `enableSTStorage bool` arg (same as non-test fix in `wal.go`) |
| `internal/component/remote/vault/vault_test.go:198` | ✅ `nat.Port("80/tcp")` → `"80/tcp"` string; removed `nat` import (testcontainers moby API change) |
| `internal/converter/internal/prometheusconvert/validate.go` | ✅ `validateStorageConfig`: Prometheus v0.311+ always injects default `TSDBConfig` and `ExemplarsConfig` after unmarshaling; updated to compare against known defaults via `reflect.DeepEqual` instead of nil check |
| `internal/converter/internal/staticconvert/testdata/prom_remote_write.alloy` | ✅ Updated hashes `601bd8→7f4a4f` and `422454→0e2644` (remote write config structure change shifted hash) |

### Pre-existing failures (not regressions, AWS env issue)

These tests fail on this machine due to missing AWS SSO config (`sso_account_id`, `sso_role_name`) and are unrelated to the upgrade:
- `internal/component/otelcol/exporter/awss3` — `TestSumoICMarshaler`, `TestSumoICMarshalerUpdate`
- `internal/component/prometheus/exporter/tests` — `TestInstanceKey/cloudwatch`
- `internal/static/integrations/cloudwatch_exporter` — `TestDecoupledCloudwatchExporterIntegrationProperSetup`, `TestCloudwatchExporterIntegrationProperSetup`

## Task 6 — OTel core alignment — BLOCKED (waiting for v0.151.0)

### Why blocked

otel-contrib must be on the same version tag as `go.opentelemetry.io/collector/*` core. The compatible version is blocked by two incompatibilities:

| Version | Problem |
|---|---|
| v0.149.0 | `pkg/translator/loki` calls `promql_parser.ParseMetric` — removed in Prometheus v0.311.2 |
| v0.150.0 | `prometheusreceiver`, `prometheusremotewriteexporter`, `prometheusexporter` require `github.com/prometheus/prometheus v0.311.2-0.20260409145810-72293ff1d2e0` — a post-tag pre-release with a breaking change to `discovery.CreateAndRegisterSDMetrics` (return type changed from `map[string]DiscovererMetrics` to `*SDMetrics`) and `discovery.NewManager` (parameter type changed to match). This is too many forks/pre-releases to carry alongside the existing prom-label-proxy and prometheus-operator forks. |

Waiting for **v0.151.0** to see if it targets a proper Prometheus release and resolves both issues. When v0.151.0 ships, check `pkg/translator/loki` and the three prometheus-dependent packages for their prometheus dependency version.

### What was completed before blocking

1. ✅ `filestatsreceiver` fork removed — upstream PR #45680 merged 2026-01-27.
2. ✅ `make generate-module-dependencies` — cleared filestatsreceiver replace from go.mod files.

### What to do when v0.151.0 is available

1. Check `prometheusreceiver`, `prometheusremotewriteexporter`, `prometheusexporter`, and `pkg/translator/loki` go.mod files for their `prometheus/prometheus` dependency — must be a released tag, not a pre-release.
2. If clean: update `dependency-replacements.yaml` otel version to `v0.151.0`, run `make generate-module-dependencies`.
3. Run `make generate-otel-collector-distro` (updates `collector/go.mod`, `collector/builder-config.yaml`).
4. Run `go mod tidy` — verify clean.
5. Fix any compilation errors in `./internal/component/otelcol/...`.
6. Fix any test failures.

### Forks checked before starting

Per the process doc, checked `dependency-replacements.yaml` for otel core or contrib replace directives — only `filestatsreceiver` was relevant (dropped, see above).

## Remaining: Task 7 — Component config audit (otelcol, v0.147.0 → v0.151.0) — BLOCKED (same as Task 6)

For each Alloy otelcol component, check whether upstream otel-contrib added new config fields between v0.147.0 and v0.151.0 that Alloy should expose. **Note:** The config audit was performed against v0.150.0 structs. Several fields implemented (kafka `RecordPartitioner`/`RecordHeaders`, awss3 `TagObjectAfterIngestion`, googlecloudpubsub `FlowControl`, tail_sampling `TailStorageID`) may be v0.150.0-only. These implementations are on the branch but the branch currently does not compile. Needs re-verification once we know the target version. Components:

**Connectors:** count, host_info, servicegraph, spanlogs, spanmetrics
**Exporters:** awss3, datadog, debug, faro, file, googlecloud, googlecloudpubsub, kafka, loadbalancing, loki, otlp, otlphttp, prometheus, splunkhec, syslog
**Processors:** attributes, batch, cumulativetodelta, deltatocumulative, discovery, filter, groupbyattrs, interval, k8sattributes, memorylimiter, metricstarttime, probabilistic_sampler, resourcedetection, span, tail_sampling, transform
**Receivers:** awscloudwatch, awsecscontainermetrics, awss3, cloudflare, datadog, faro, file_stats, filelog, fluentforward, googlecloudpubsub, influxdb, jaeger, kafka, loki, otlp, prometheus, solace, splunkhec, syslog, tcplog, vcenter, zipkin
**Auth/Extensions:** basic, bearer, google, headers, oauth2, sigv4, jaeger_remote_sampling
**Storage:** file

### Approach
1. ✅ Fetched otel-contrib release changelogs for v0.148.0, v0.149.0
2. ✅ Identified which components had config-relevant changes
3. ✅ Compared upstream config structs with Alloy wrappers

### Findings — components that need new config exposed

| Component | New field(s) | Status |
|---|---|---|
| `otelcol.auth.headers` | `value_file *string` in header block (file-based credential with auto-refresh; mutually exclusive with `value`, `from_context`, `from_attribute`) | ❌ Not exposed; validation also needs updating |
| `otelcol.receiver.cloudflare` | `max_request_body_size int64` (default 20MB, 0=unlimited) | ❌ Not exposed |
| `otelcol.receiver.filelog` | `include_file_permissions bool` (adds `log.file.permissions` attribute, not Windows); `max_log_size_behavior string` (`split`/`truncate`) | ❌ Neither exposed |
| `otelcol.auth.sigv4` | `external_id string` in `assume_role` block (cross-account auth) | ❌ Not exposed |
| `otelcol.processor.tail_sampling` | `sampling_strategy string` (`trace-complete` default, `span-ingest`); `tail_storage` reference to extension | ❌ Neither exposed |
| `otelcol.receiver.awss3` | `tag_object_after_ingestion bool` | ❌ Not exposed |
| `otelcol.receiver.googlecloudpubsub` | `flow_control` block: `trigger_ack_batch_duration`, `stream_ack_deadline`, `max_outstanding_messages`, `max_outstanding_bytes` | ❌ Not exposed |
| `otelcol.connector.spanmetrics` | `enable_metrics_sampling_method bool` (adds `sampling.method` attribute: `extrapolated` or `counted`) | ❌ Not exposed |
| `otelcol.exporter.kafka` | `record_headers` (static headers map on outgoing records); `record_partitioner` block (sticky_key config) | ❌ Neither exposed |
| `otelcol.processor.resourcedetection` | IBM Cloud detectors: `ibmcloud` (classic + vpc sub-detectors) | ❌ Not exposed |

### Findings — no action needed

| Component | Reason |
|---|---|
| `otelcol.receiver.prometheus` | `report_extra_scrape_metrics` was never exposed in Alloy — nothing to remove |
| `otelcol.exporter.kafka` deprecated `topic`/`encoding` | Alloy's `Convert()` maps these to per-signal `SignalConfig.Topic/Encoding` (still valid upstream fields); no breakage |
| `otelcol.receiver.kafka` | `kafka.topic/partition/offset` metadata attributes are automatic (hardcoded in receiver code, no config needed); SASL OAUTHBEARER is a new valid string value for the existing `mechanism` field |
| `otelcol.exporter.prometheus` `keep_alives_enabled` fix | Behavioral fix, no config change |
| `otelcol.receiver.awss3` zstd support | Handled automatically by decompression layer, no explicit config needed |

## Remaining: Task 5 — Changelog + docs (PR description)

### What is user-facing

Most of this upgrade is transparent to users. The one item that needs a breaking-change note in the PR description:

**`loki.process` — parsed labels no longer override structured metadata (Loki v3.7.0, [#19991](https://github.com/grafana/loki/pull/19991))**

`pkg/logql/log/parser.go` was changed to protect structured metadata from being silently overwritten by parser stages. All parser stages (`stage.json`, `stage.logfmt`, `stage.regexp`, `stage.pattern`, `stage.unpack`) previously let a parsed label win over an existing structured metadata key with the same name. Now the structured metadata value is kept under the bare key and the parsed value is placed under `<key>_extracted`.

There is no opt-out — it is a hardcoded logic change in the library with no feature flag. We cannot stay on Loki v3.6.x because it imports `tsdb/errors` which was removed from Prometheus in v3.10+, making the Loki bump forced and the two mutually exclusive.

**Who is affected:** Users of `loki.process` who:
1. Have structured metadata already on log entries before a parser stage runs (e.g. from an OTel source, or from an earlier `stage.structured_metadata` in the same pipeline), AND
2. Have a parser stage that extracts a field with the same key name as that structured metadata.

No error or warning is emitted — the label name silently changes. Users should audit their `loki.process` pipelines if they use structured metadata alongside parser stages.

**What is NOT user-facing:**
- Loki PR #19996 (relabel config validation panic fix) — Promtail-only, Alloy has its own implementations.
- `prometheus_remote_storage_sent_batch_duration_seconds` semantics fix — users receive the fix automatically; no action needed.
- Prometheus v3.10/v3.11 PromQL new functions (`fill()`, `histogram_quantiles()`, etc.) — execute server-side against Mimir/Prometheus, not in Alloy itself.
- WAL/TSDB bug fixes — transparent.
