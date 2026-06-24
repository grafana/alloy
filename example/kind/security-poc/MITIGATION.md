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
- **Stdlib function allowlist / denylist** — which stdlib functions may be called. Some are
  capabilities in their own right (e.g. `sys.env` reads environment variables, file readers read
  the filesystem) and are not components, so the component gate alone misses them.
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

- Component registry → component gate (also gates stdlib functions at evaluation time).
- Config loader → signature gate.
- **Endpoint gate — no single choke point exists today.** There is no shared HTTP/gRPC dialer
  to hook. Outbound connections are built independently across many places: Alloy-native HTTP
  clients (e.g. `loki.write`, `mimir.write`, `prometheus.scrape`, `remote.http`), OTel exporters
  (`otelcol.*`, their own clients), and client-go paths (`discovery.kubernetes`,
  `loki.source.kubernetes`). So the endpoint allowlist must be enforced **per component that
  egresses**, for every such component. This is repetitive and easy to get wrong: one missed
  client is a full bypass. We accept this and lean on thorough review — including LLM-assisted
  review — to find gaps, and on tests that assert each egress component honours the policy.
  A future refactor toward a central dialer would make this enforceable in one place.

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

**Anti-rollback is best-effort.** The monotonic `ver` check needs the last-seen version to
survive restarts. We can keep it in memory (lost on restart) or in Alloy's data directory (lost
if the pod/volume is recreated, which is normal in Kubernetes). So `ver` is not a strong control
on its own — **`exp` (freshness) is the real replay defence**, and `ver` is a best-effort extra.

**Exfil through an allowed endpoint is out of scope.** If config sends data to an endpoint the
policy permits, attacker-controlled config can stuff secrets into that traffic.

## Complementary mitigations (outside this design)

These are not part of the config-admission layer but reduce blast radius and are worth doing
alongside it:

- **Tighten the Helm chart RBAC.** The POC works partly because default RBAC grants
  `get, list, watch` on `secrets` cluster-wide. Scope the ServiceAccount down to what the
  deployment actually needs (drop `secrets` where unused, prefer namespaced Roles over
  cluster-wide). This directly limits the Kubernetes reconnaissance and secret-theft vectors,
  independent of whether config is signed.

- **Kubernetes NetworkPolicy (egress default-deny).** The Alloy helm chart ships a
  disabled NetworkPolicy (`networkPolicy.enabled: false`). Enabling it with default-deny
  egress and explicit allow rules blocks SSRF to in-cluster services and the cloud IMDS
  endpoint (`169.254.169.254/32`) without any Alloy code changes. Limitation: standard
  NetworkPolicy is L3/L4 only — no hostname matching for external endpoints (CDN IPs
  churn). Works in kind today (kindnet enforces it).

- **DNS-aware egress policy (Cilium `toFQDNs`).** For hostname-level allowlisting of
  external endpoints (`metrics.grafana.net`, etc.), Cilium's `CiliumNetworkPolicy` with
  `toFQDNs` watches DNS responses and enforces policies by resolved IP. Requires replacing
  kindnet with Cilium as the CNI. The helm chart has no Cilium template today; extra
  manifests would be needed.

- **Cloud IMDS hardening.** On AWS: enforce IMDSv2 with `HttpPutResponseHopLimit=1` at
  the EC2 instance level — packets from Pods (hop > 1) cannot reach the metadata service
  regardless of Alloy config. Zero Alloy changes; pure cloud provider config. GCP and Azure
  have analogous controls (metadata concealment, managed identity scoping).

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
