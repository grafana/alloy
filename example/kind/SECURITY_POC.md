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

- **Value:** `SECRET_1=flag_1`
- **Where:** env var on the Alloy container (`alloy.extraEnv` in `values.yaml`).
- **Lesson:** secrets passed as env vars leak easily — process listings,
  `/proc/<pid>/environ`, crash dumps, child processes, and anything that can
  read the pod spec.

### Flag 2 — secret in a ConfigMap, mounted as a file

- **Value:** `SECRET_2=flag_2`
- **Where:** ConfigMap `security-poc-flags`, also mounted into the Alloy
  container at `/etc/security-poc/SECRET_2`.
- **Lesson:** ConfigMaps are not secrets — they are plaintext to anyone with
  read access to the namespace. Mounting one as a file exposes it to anything
  that can read the container filesystem (or traverse it via a path bug).

### Flag 3 — secret in a pod annotation

- **Value:** `flag_3` (annotation `security-poc/flag-3` on pod `vuln-http-server`)
- **Where:** metadata on the `vuln-http-server` pod (a tiny `hashicorp/http-echo`).
- **Lesson:** requires Kubernetes API read access. The same read also reveals
  the container image and version (`hashicorp/http-echo:0.2.3`), which an
  attacker uses to look up known CVEs — especially dangerous for
  internet-facing servers.

### Flag 4 — secret in an internal HTTP response

- **Value:** `flag_4`
- **Where:** `GET http://internal-api.monitoring.svc:8080/internal-endpoint`
  (a small Python stdlib server, pod `internal-api`).
- **Lesson:** internal-only endpoints are reachable from inside the cluster.
  If Alloy can be made to fetch arbitrary URLs (SSRF) or is configured to
  scrape this target, the response body — the flag — is exposed.

### Flag 5 — unauthenticated shutdown / DoS endpoint

- **Value:** no text flag; the weakness itself is the point.
- **Where:** `GET http://internal-api.monitoring.svc:8080/quitquitquit`
  returns `shutting down critical server`.
- **Lesson:** an unauthenticated control endpoint that can take down a
  critical service is a denial-of-service vector. (In this POC it only returns
  the message and keeps running; real implementations that actually shut down
  are the risk being modelled.)

## Verifying a flag

Always use the kind kubeconfig:

```sh
# Flag 1 — env var
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring exec deploy/alloy -c alloy -- printenv SECRET_1

# Flag 2 — mounted file
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring exec deploy/alloy -c alloy -- cat /etc/security-poc/SECRET_2

# Flag 3 — pod annotation
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring get pod vuln-http-server -o jsonpath='{.metadata.annotations.security-poc/flag-3}'

# Flag 4 + 5 — internal HTTP server (via port-forward)
kubectl --kubeconfig build/kubeconfig.yaml -n monitoring port-forward pod/internal-api 8080:8080 &
curl -s http://localhost:8080/internal-endpoint   # flag_4
curl -s http://localhost:8080/quitquitquit         # shutting down critical server
```
