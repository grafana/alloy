# Beyla update implementation status

State: complete

## Changes made

### Version bumps
- `go.mod`: `github.com/grafana/beyla/v3` v3.7.0 → v3.9.0
- `go.mod`: OBI replace directive v1.16.2 → v1.18.0
- `dependency-replacements.yaml`: OBI v1.16.2 → v1.18.0
- `collector/builder-config.yaml`: OBI v1.16.2 → v1.18.0
- `collector/go.mod`: OBI replace directive v1.16.2 → v1.18.0
- `docs/sources/_index.md.t`: BEYLA_VERSION v3.7.0 → v3.9.0

### New fields added
- `args.go`: `Metrics.ExemplarFilter string` (OBI v1.18.0 `PrometheusConfig.ExemplarFilter`)
- `args.go`: `EBPFMapsConfig` struct + `EBPF.MapsConfig EBPFMapsConfig` (OBI v1.18.0 `EBPFTracer.MapsConfig`)
- `args.go`: `Injector.ImageVolumePath string` (Beyla v3.9.0 `SDKInject.ImageVolumePath`)

### Logic updates in beyla_linux.go
- `validInstrumentations`: added `"genai"` and `"memcached"`
- `validFeatures`: added `"*"` and `"all"`
- `hasAppFeature()`: handles `"*"` and `"all"` wildcards
- `Metrics.Convert()`: propagates `ExemplarFilter`
- `EBPF.Convert()`: propagates `MapsConfig.GlobalScaleFactor`; also fixed `PayloadExtraction.HTTP.GenAI.OpenAI.Enabled` (OpenAI moved under GenAI in OBI v1.18.0)
- `Injector.Convert()`: propagates `ImageVolumePath`
- `Injector.Validate()`: enforces mutual exclusivity of `ImageVolumePath` with `HostMountPath` and `SDKPkgVersion`

### Tests
- Updated `TestConvert_Attributes` for new `ReconnectInitialInterval` default in Beyla v3.9.0
- Updated `TestConvert_EBPF` for `PayloadExtraction.HTTP.GenAI.OpenAI.Enabled` path
- Updated `TestArguments_UnmarshalSyntax` for same
- Added: `TestMetrics_Validate_ExemplarFilter`, `TestMetrics_Convert_ExemplarFilter`
- Added: `TestMetrics_Validate_FeatureWildcard`
- Added: `TestEBPF_Convert_MapsConfig`
- Added: `TestInjector_Validate_ImageVolumePath`, `TestInjector_Convert_ImageVolumePath`
- Added `"genai"`, `"memcached"` to `TestMetrics_Validate` and `TestTraces_Validate`

### Docs
- `beyla.ebpf.md`: added `image_volume_path`, `exemplar_filter`, `maps_config`, `genai`, `memcached`, `"*"`/`"all"` features

## Verification
Tests passed: `go test ./internal/component/beyla/ebpf/...` OK
