# Slim Alloy Build — Design

This document describes the design of the **slim Alloy distribution**: a trimmed
build of Grafana Alloy that keeps only the components needed for a
metrics-scrape + remote-write + node-exporter + Loki-file-logs deployment, and
strips out the dependency-heavy subsystems that are not used. The result is a
**~59 MB** binary, down from the **~516 MB** default debug build.

> Companion: [Chinese version](./slim-build-design.zh-CN.md)

## 1. Motivation

The default Alloy binary built from `collector/` is very large (~516 MB debug,
~321 MB stripped) because it ships every OTel Collector component, every native
Alloy component (~180), and the dependency-heavy subsystems they pull in
(AWS/GCP/Azure SDKs, Kubernetes client-go, the OTel Collector framework, etc.).

For a lightweight single-node deployment that only does **metrics scraping +
remote_write + node_exporter + Loki file logs**, the vast majority of that is
dead weight. This design shrinks the binary by ~89% while keeping exactly the
components that deployment needs.

## 2. Results

| Stage | Size | What changed |
|------:|-----:|--------------|
| Baseline (debug, full) | 516 MB | Default `make alloy` (no strip) |
| Phase 1 | 321 MB | Trim component registries + `-s -w` strip |
| Phase 2 | 165 MB | `slim` tag gates converters + static integrations |
| **Phase 3** | **59 MB** | `slim` tag removes k8s client-go + cloud SDKs |

Total reduction: **516 MB → 59 MB (-89%)**.

Dependency impact (full → slim, by compiled package count):

| Dependency | full | slim |
|---|---:|---:|
| k8s.io/client-go | 148 | 2 |
| hashicorp/go-discover (+ cloud SDKs) | all | 0 |
| DataDog | 193 | 0 |
| apache/arrow-go | 31 | 0 |
| opentelemetry-collector-contrib | 317 | ~47 |

## 3. Kept components (the whitelist)

The slim distro registers exactly these 10 native Alloy components:

| Component | Import path |
|---|---|
| `prometheus.scrape` | `internal/component/prometheus/scrape` |
| `prometheus.remote_write` | `internal/component/prometheus/remotewrite` |
| `prometheus.relabel` | `internal/component/prometheus/relabel` |
| `prometheus.exporter.unix` | `internal/component/prometheus/exporter/unix` |
| `prometheus.exporter.self` | `internal/component/prometheus/exporter/self` |
| `discovery.relabel` | `internal/component/discovery/relabel` |
| `loki.source.file` | `internal/component/loki/source/file` |
| `loki.process` | `internal/component/loki/process` |
| `loki.write` | `internal/component/loki/write` |
| `local.file_match` | `internal/component/local/file_match` |

`remotecfg` (Fleet Management remote configuration) is part of the Alloy engine
and remains available — it is not a component in the registry.

## 4. Architecture: two independent trimming axes

The size reduction comes from two orthogonal mechanisms.

### Axis A — Component registry trim (permanent, on this branch)

Two registries are edited in place to contain only what is needed:

- **Native components** — `internal/component/all/all.go` is reduced from ~180
  blank imports to the 10 above. The Alloy engine has no hard requirement on
  which components are registered; unregistered components simply produce an
  "unknown component" error if referenced in config, with no effect on startup.
- **OTel Collector components** — `collector/builder-config.yaml` is stripped to
  the `alloyengine` extension, the confmap providers, and one `nop`
  receiver/exporter placeholder. The OCB-generated files (`main.go`,
  `components.go`, `go.mod`, `go.sum`) are regenerated via
  `make generate-otel-collector-distro`.

This axis is a permanent edit on the `slim-collector-distro` branch. The full
component set is preserved on `main`.

### Axis B — `slim` build tag (toggleable)

A Go build tag `slim` gates the dependency-heavy subsystems that are pulled in
at the framework level (not via the component registry), following the repo's
existing split-file pattern (`embedalloyui`, `boringcrypto`). Each gated unit
has a `//go:build !slim` file (full implementation) and a `//go:build slim`
file (stub / empty). Building with `GO_TAGS=slim` selects the stubs.

The gated subsystems (none used by the target deployment):

| Subsystem | Files | Why it's heavy |
|---|---|---|
| OTel feature-gate registration | `internal/util/otel_feature_gate*.go` | Blank-imports `k8sattributesprocessor` → OTel k8s processor + openshift client-go |
| Cluster peer discovery | `internal/service/cluster/discovery/go_discovery*.go`, `peer_discovery.go` | `hashicorp/go-discover` → k8s client-go **and all cloud-provider SDKs** |
| Config converters (`alloy convert`) | `internal/converter/convert_{heavy,slim}.go`, `converter.go` | `otelcolconvert`/`staticconvert`/`prometheusconvert`/`promtailconvert` → OTel components, static integrations, prometheus k8s SD |
| Prometheus SD install | `flowcmd/flowcmd.go`, `flowcmd/integrations_full.go` | `prometheus/discovery/install` registers all SD providers (k8s, ec2, azure, gce…) |
| Static-mode integrations | `flowcmd/integrations_full.go` | `static/integrations/install` → vmware/azure/gcp exporters |

**Why both axes are needed.** Trimming the component registry alone (Axis A)
left the binary at ~165 MB, because k8s client-go and the cloud SDKs are pulled
by **framework-level** code (clustering, converters, SD install) — not by the
component registry. Empirically, removing any single anchor did nothing;
client-go has multiple independent import paths. Only gating **all** of them
together (Axis B, phase 3) dropped client-go from 148 to 2 packages and the
binary to 59 MB.

## 5. Build instructions

| Build | Command | Size |
|---|---|---|
| **Slim** (this branch) | `GO_TAGS=slim SKIP_UI_BUILD=1 SKIP_CODE_GENERATION=1 RELEASE_BUILD=1 make alloy` | ~59 MB |
| **Full** (from `main`) | `git checkout main && RELEASE_BUILD=1 make alloy` | ~321 MB |

Notes:
- `RELEASE_BUILD=1` adds `-ldflags "-s -w"` (strip symbol table + DWARF).
- `SKIP_UI_BUILD=1` skips the `npm` UI build; the default build does not embed
  the UI (no `embedalloyui` tag), so this is safe.
- `SKIP_CODE_GENERATION=1` skips OCB regeneration (already committed).
- The Makefile auto-prepends `gore2regex`, so the effective tags are
  `gore2regex slim`.

## 6. Slim build trade-offs

The slim build intentionally disables features the target deployment does not
use. Each degrades gracefully (clear error), never crashes:

- **No cluster peer auto-discovery** — `--cluster.discover-peers` returns a
  "not supported in slim build" error. Static `--cluster.join-addresses` still
  works.
- **No `alloy convert`** — converting otelcol/static/prometheus/promtail configs
  returns a critical diagnostic pointing to a full build.
- **No OTel Collector components / `alloy otel`** — the OTel component set is
  not compiled in.

The core deployment path — `alloy run` of native Prometheus/Loki components,
including remotecfg-delivered config and self-monitoring — is fully supported.

## 7. Adding a component to the slim build

Add its blank import to `internal/component/all/all.go` and rebuild. If the new
component pulls a heavy dependency that should stay out of slim, gate that
dependency with the `slim` tag using the split-file pattern in section 4.

## 8. Verification

The slim binary is verified by running a config that instantiates all 10
components (`alloy run`) and asserting the controller finishes graph evaluation
with no component build errors. Dependency removal is checked with
`go list -tags "gore2regex slim" -deps` (client-go ≤ 2, go-discover 0). The
default (no-tag) build of every gated package is also confirmed to compile, so
the full distribution is never broken by the slim plumbing.

## 9. File map

```
internal/component/all/all.go                         # Axis A: native registry (10 components)
collector/builder-config.yaml                         # Axis A: OTel registry (minimal)
internal/util/otel_feature_gate{,_full,_slim}.go      # Axis B: OTel feature gates
internal/service/cluster/discovery/
    peer_discovery.go                                 # Axis B: tag-agnostic dispatch
    go_discovery.go        (//go:build !slim)         # Axis B: real go-discover
    go_discovery_slim.go   (//go:build slim)          # Axis B: stub
internal/converter/
    converter.go                                      # Axis B: tag-agnostic dispatch
    convert_heavy.go       (//go:build !slim)         # Axis B: real converters
    convert_slim.go        (//go:build slim)          # Axis B: stubs
flowcmd/flowcmd.go                                    # Axis B: SD install removed
flowcmd/integrations_full.go (//go:build !slim)       # Axis B: SD + static integrations
```
