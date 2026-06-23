# Mitigations

Defense against the attack in [SECURITY_POC.md](SECURITY_POC.md): an attacker who
controls the config Alloy fetches and runs (via `import.*` or `remotecfg`).

## Proposed design: external security policy + signed config

A **security policy** in a separate file, passed by CLI flag
(`--security-policy=<path>`), read once at startup and immutable thereafter.
`import.*` and `remotecfg` cannot define or relax it. (If it's a ConfigMap, make it
immutable, or bake it into the image/PodSpec, so the data-plane RBAC can't edit it.)

We pick the external file because it's the easiest to iterate on and ports to the
OTel engine. An in-Alloy-config `security_policy {}` block (root-only, "read once"
service) is also possible and more native, but it must additionally guarantee
`remotecfg` can't set it, so it's harder to prove correct.

**Backward compatibility (no breaking changes):**

- **No `--security-policy` flag** → status-quo Alloy, nothing restricted.
- **Flag present but empty policy** → deny all (components and endpoints).
- **Flag present with rules** → default-deny *within the policy*; only what's listed
  is allowed.

The policy defines:

- **Component allowlist / denylist** — which components may run.
- **External endpoint allowlist** — where components may connect.
- **Connections between components** — optional, `warn` only (legit vs. abuse is
  domain-specific, so don't hard-block).
- **Signature requirement** — require a verified signature on all fetched config.
- **Public keys / trust anchors** used to verify those signatures.

### Signed config

Each config source provides a **JWS** (detached) alongside its content — a sibling
path/URL or a header, TBD per source. Alloy verifies it before evaluating, and
fails closed:

- asymmetric alg only (EdDSA/ES256/RS256), never `none` or HMAC;
- key resolved via the policy's pinned keys;
- claims checked: `exp` (freshness), monotonic `ver` (anti-rollback), `aud`
  (intended collector/fleet);
- content hash matches the fetched bytes.

For `import.git`, verify native signed commits/tags instead.

### Enforcement choke points

- Component registry → component gate.
- Shared HTTP/gRPC dialer → endpoint gate (must be central, or attackers pivot).
- Config loader → signature gate.

### Validation command

A dry-run subcommand to check a config against a policy *before* deploying:

```sh
alloy security-policy check --security-policy=policy.yaml config.alloy
```

It reports:

- **Violations** — components/endpoints/connections the config uses that the policy
  forbids (this config would be rejected).
- **Missing permissions** — what to add to the policy to make this config pass.
- **Tightening suggestions** — allowed entries the config never uses, so you can
  remove them and shrink the policy.

This makes the policy easy to author iteratively and to debug rejections.

### Limits

Signing reduces trust to the signing key — a key/pipeline compromise still produces
valid configs, so the component and endpoint gates remain essential. The k8s API is
a legitimate endpoint, so flag 5 needs least-privilege RBAC, not an egress rule.

## Grafana Cloud Fleet Management

The same policy becomes a Fleet Management feature: define an **org-wide policy** in
the Fleet Management UI, and every pipeline is checked against it **before it can be
saved** (the validation command above, run server-side). Alloys must be **seeded
with the same policy file** so enforcement also happens at the edge, not just at
save time. Keeping both in sync is extra work — that's the cost of hardening.

Fleet Management can also get extra features, e.g. **certificates** for signing
config (it's a common source of configs), so collectors verify Fleet-delivered
config against a Fleet CA instead of managing pinned keys by hand.

## Portability

This is an engine-agnostic config-admission layer. It works in the Alloy engine and
can be ported to the OTel engine / OpAMP world (supervisor verifies the signed
remote config; an extension enforces the component and endpoint gates), ideally
sharing one policy + JWS schema.
