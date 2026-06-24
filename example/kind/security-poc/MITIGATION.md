# Mitigations

Defense against the attack in [SECURITY_POC.md](SECURITY_POC.md): an attacker who
controls the config Alloy fetches and runs (via `import.*` or `remotecfg`).

## Proposed design: external security policy + signed config

A **security policy** in a separate file, passed by CLI flag
(`--security-policy=<path>`), read once at startup and immutable thereafter.
`import.*` and `remotecfg` cannot define or relax it. (If it's a ConfigMap, make it
immutable, or bake it into the image/PodSpec, so the data-plane RBAC can't edit it.)

We pick the external file because it's the easiest to iterate on in this POC.
An in-Alloy-config `security_policy {}` block (root-only, "read once"
service) is also possible and more native, but it must additionally guarantee
`remotecfg` can't set it, so it's harder to prove correct.

**Backward compatibility (no breaking changes):**

- **No `--security-policy` flag** → status-quo Alloy, nothing restricted.
- **Flag present but empty policy** → deny all (components and endpoints).
- **Flag present with rules** → default-deny *within the policy*; only what's listed
  is allowed.

The policy defines:

- **Component and config blocks allowlist / denylist** — which components and config blocks may run.
- **External endpoint allowlist** — where components may connect.
- **Connections between components** — optional allowlist of connections between components.
- **Signature requirement** — require a verified signature on all fetched config.
- **Public keys / trust anchors** used to verify those signatures.

### Signed config

Each config source can optionally provide a **JWS** (detached) alongside its content — a sibling
path/URL or a header, TBD per source. Alloy verifies it before evaluating, and
fails closed:

- asymmetric alg only (EdDSA/ES256/RS256), never `none` or HMAC;
- key resolved via the policy's pinned keys;
- claims checked: `exp` (freshness), monotonic `ver` (anti-rollback), `aud`
  (intended collector/fleet);
- content hash matches the fetched bytes.

### Enforcement choke points

- Component registry → component gate.
- Shared HTTP/gRPC dialer → endpoint gate (must be central, or attackers can pivot).
- Config loader → signature gate.

### Validation command

A dry-run subcommand to check a config against a policy *before* deploying, to help iterate on the policy:

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

Future extension: automatically generate the policy from the config that is maximally tight.

### Limits

Signing reduces trust to the signing key — a key/pipeline compromise still produces
valid configs, so the component and endpoint gates remain essential.

## Grafana Cloud Fleet Management

Fleet Management connections are already over TLS.

The Alloy's security policies could become a feature of Fleet Management:
Users can define an **org-wide policy** in
the Fleet Management UI, and every pipeline is checked against it **before it can be saved** (the validation command above, run server-side).
Alloys can be **seeded with the same (or a subset of the) policy file** so enforcement also happens at the edge if users desire the extra security.

## Portability

This is an engine-agnostic config-admission layer. It works in the Alloy engine and
can be ported to the OTel engine / OpAMP world (supervisor verifies the signed
remote config; an extension enforces the component and endpoint gates), ideally
sharing one policy + JWS schema.
