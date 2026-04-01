# Beyla update implementation status

State: complete

## Plan executed

Bumped Beyla v3.6.0 → v3.7.0 and OBI v1.12.2-0.20260318145328-e31c5acda289 → v1.16.2.

## Changes made

1. `dependency-replacements.yaml` - OBI version: v1.12.2-pseudo → v1.16.2
2. `go.mod` - beyla v3.7.0, OBI replace v1.16.2
3. `collector/go.mod` - beyla v3.7.0, OBI replace v1.16.2
4. `collector/builder-config.yaml` - OBI replace v1.16.2
5. `docs/sources/_index.md.t` - BEYLA_VERSION: v3.7.0
6. `internal/component/beyla/ebpf/args.go` - added Stats struct and Stats field to Arguments
7. `internal/component/beyla/ebpf/beyla_linux.go`:
   - Added "stats" to validFeatures map
   - Added Stats.Convert() method
   - Wired cfg.Stats = a.Stats.Convert() in Arguments.Convert()
   - Fixed Routes.Convert() to deep-copy default RoutesConfig (pointer aliasing bug in beyla.DefaultConfig())
8. `internal/component/beyla/ebpf/beyla_linux_test.go` - added Stats assertion in TestArguments_ConvertDefaultConfig
9. `docs/sources/reference/components/beyla/beyla.ebpf.md` - added stats feature, stats block to table, stats section

## Notable fix

Beyla v3.7.0's DefaultConfig() mutates a global pointer (obi.DefaultConfig.Routes) when setting
the new default `UnmatchLowCardinality`. All Convert() methods that call DefaultConfig() share
the same *transform.RoutesConfig pointer, causing a race condition where a later DefaultConfig()
call resets the routes config modified by a prior call.

Fix: Routes.Convert() now deep-copies the default routes config
(`defaultRoutes := *beyla.DefaultConfig().Routes; routes := &defaultRoutes`)
before modifying it, so subsequent DefaultConfig() calls don't reset it.

## Verification

Tests pass:
```
go test ./internal/component/beyla/ebpf/...
ok  github.com/grafana/alloy/internal/component/beyla/ebpf  0.045s
```
