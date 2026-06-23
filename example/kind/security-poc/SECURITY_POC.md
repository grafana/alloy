# Security POC

A set of intentionally planted "flags" in this local kind cluster, used to build a security POC.

Everything here is deployed by a single task:

```sh
task deploy:security-poc
```

This creates the `monitoring` namespace and deploys:

- **Alloy** (1 replica) from the chart, using `config/security-poc/values.yaml`.
- The extra k8s manifests in `config/security-poc/manifests/`.

> All flag values are dummy strings. Nothing here is a real secret. The cluster is created on purpose for the POC.

## Attack vector

This POC explores what happens when an attacker can control the configuration
that Alloy runs.

Alloy is configured to fetch part of its config from an in-cluster HTTP server
using `import.http`, then run whatever it gets back:

- The Alloy main config (`config/security-poc/values.yaml`) imports a module
  from `http://config-server.monitoring.svc/module.alloy` with a 1 second
  poll, and instantiates the `user_pipeline` custom component it declares.
- That URL is served by the `config-server` pod (nginx), which just serves the
  `user-pipeline-module` ConfigMap. Today the module is a harmless no-op.
- Because the poll frequency is 1 second, any change to that ConfigMap is
  picked up and executed by Alloy almost immediately — no restart needed.

For this POC we explore the scenario where the attacker gained control over the configuration that Alloy fetches from the endpoint above. In practice, there are other import sources, so this could be anything from a Git repository, HTTP endpoint, a local file or Fleet Management server (self-hosted or in Grafana Cloud via API).

## Flags

### Flag 1 — secret in an environment variable

- **Value:** `SECRET_1=secret_value_flag_1`
- **Where:** env var on the Alloy container (`alloy.extraEnv` in `values.yaml`).

### Flag 2 — secret in a Kubernetes Secret, mounted as a file

- **Value:** `SECRET_2=secret_value_flag_2`
- **Where:** Kubernetes Secret `security-poc-flags`, mounted into the Alloy
  container at `/etc/security-poc/SECRET_2`.

### Flag 3 — secret in a pod annotation

- **Value:** `secret_value_flag_3` (annotation `security-poc/flag-3` on pod `vuln-http-server`)
- **Where:** metadata on the `vuln-http-server` pod (a tiny `hashicorp/http-echo`).

### Flag 4 — unauthenticated DoS endpoint on an internal service

- **Value:** no text flag; the capability itself is the point.
- **Where:** `GET http://internal-api.monitoring.svc:8080/quitquitquit`
  returns `shutting down critical server`.
- **Bonus:** `GET /internal-endpoint` on the same pod returns `secret_value_flag_4`
  — demonstrates SSRF to internal services alongside the DoS vector.

### Flag 5 — k8s API: full resources (secrets, pods, etc.) enumeration

- **Value:** all resources in the cluster, including secrets.
- **Where:** `https://kubernetes.default.svc/api/v1/secrets` — the k8s API server,
  called using Alloy's own mounted ServiceAccount token.
- **Why it works by default:** the Alloy helm chart grants `get, list, watch` on
  `secrets` cluster-wide. A compromised pipeline
  config can use the same credential to enumerate every resource.

## Verifying flags are deployed correctly

Always use the kind kubeconfig:

```sh
# Flag 1 — env var
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring exec deploy/alloy -c alloy -- printenv SECRET_1

# Flag 2 — mounted file
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring exec deploy/alloy -c alloy -- cat /etc/security-poc/SECRET_2

# Flag 3 — pod annotation
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring get pod vuln-http-server -o jsonpath='{.metadata.annotations.security-poc/flag-3}'

# Flag 4 — DoS + SSRF (via port-forward)
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring port-forward pod/internal-api 8080:8080 &
curl -s http://localhost:8080/quitquitquit         # shutting down critical server
curl -s http://localhost:8080/internal-endpoint    # secret_value_flag_4 (SSRF bonus)

# Flag 5 — list all secrets via k8s API (ServiceAccount token is mounted in pod)
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring exec deploy/alloy -c alloy -- \
  sh -c 'curl -sk -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  https://kubernetes.default.svc/api/v1/secrets | python3 -c "import sys,json; [print(s[\"metadata\"][\"name\"]) for s in json.load(sys.stdin)[\"items\"]]"'
```

## Demo notes

- **`kubectl.kubernetes.io/last-applied-configuration` annotation** — `kubectl apply`
  stores the full resource manifest as a JSON annotation on every object it touches.
  The flag3 demo's `labelmap` rule picks this up automatically, so every log line
  arriving at the receiver carries the entire pod spec (env vars, image, ports,
  volumes, args) as a label value. No extra steps needed — it's already there.
