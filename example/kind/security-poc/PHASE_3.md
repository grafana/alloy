# Phase 3: JWS Signature Gate

Requires all fetched (dynamic) config to carry a verified JWS signature before evaluation.

**Policy fields added this phase:** `require_signature`, `trust_anchors`.

## What it protects

Signed configs ensure that even if an attacker can intercept or modify the config source (MITM on `import.http`, compromised Git repo, compromised Fleet Management server), they cannot produce a config that Alloy will run without the signing key.

Note: signing reduces trust to the key — a signing key compromise still produces valid configs. The component and endpoint gates remain essential.

## Policy fields

```go
type SecurityPolicy struct {
    // ... previous phases ...
    RequireSignature bool     `yaml:"require_signature"`
    TrustAnchors     []string `yaml:"trust_anchors"` // PEM-encoded public keys
}
```

## JWS format

- Detached compact JWS (signature alongside the config, not inline)
- Algorithms: EdDSA, ES256, RS256 only — never `none` or HMAC
- Required claims: `exp` (expiry), `ver` (monotonic version for best-effort anti-rollback), `aud` (intended collector/fleet ID), content hash of the config bytes

## Delivery convention (TBD per import source)

| Source | Where the JWS travels |
|--------|----------------------|
| `import.http` | Sibling URL (`<url>.jws`) or `X-Alloy-Signature` response header |
| `import.git` | Sibling file in repo (`<config-file>.jws`) |
| `remotecfg` | Extra field in the API response |
| Local root config | Sibling file (`<config>.jws`) or skip (operator controls the file) |

## Hook points (fail-closed)

If `require_signature = true` and no valid signature is found, Alloy refuses to load/apply the config:

- `internal/alloycli/cmd_run.go` `loadSourceFiles()` — root config
- `internal/runtime/internal/controller/node_config_import.go` `ImportConfigNode.onContentUpdate()` — imported modules
- `internal/service/remotecfg/config_manager.go` `parseAndLoad()` — remotecfg

## New file: `internal/securitypolicy/verify.go`

```go
func Verify(configBytes []byte, jwsToken string, policy *SecurityPolicy) error
```

Parses the JWS, resolves the key from `TrustAnchors`, verifies the signature and claims.

## Anti-rollback caveat

The monotonic `ver` check needs the last-seen version to survive restarts. Storing it in memory loses it on restart; storing in Alloy's data directory loses it if the pod/volume is recreated (normal in Kubernetes). `ver` is best-effort — `exp` is the real replay defence.
