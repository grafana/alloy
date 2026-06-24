# Security POC

This POC explores what happens when an attacker can control the configuration
that Alloy runs.

Alloy is configured to fetch part of its config from an in-cluster HTTP server
using `import.http`, then run whatever it gets back.

In practice, there are other import sources, so this could be anything from a Git repository, HTTP endpoint, a local file or Fleet Management server (self-hosted or in Grafana Cloud via API).

## Demonstrated

### Reading environment variables

**Demonstrated:** `SECRET_1=secret_value_flag_1`

Attacker is able to read environment variables passed to the Alloy process.

### Read access to file system

**Demonstrated:** `SECRET_2=secret_value_flag_2` in a Kubernetes Secret `security-poc-flags`, mounted into the Alloy
  container at `/etc/security-poc/SECRET_2`.

Attacker is able to read arbitrary files in the Alloy container.

### Server-side Request Forgery (SSRF) to internal services

**Demonstrated:** `GET http://internal-api.monitoring.svc:8080/quitquitquit` and `GET http://internal-api.monitoring.svc:8080/internal-endpoint` return `shutting down critical server` and `secret_value_flag_4` respectively.

Attacker is able to make requests to internal services. This can help discover further vulnerabilities and pivot.

### Reconnaissance via Kubernetes API

**Demonstrated:** `https://kubernetes.default.svc/api/v1/*` can be used to return manifests of all resources in the cluster, including secrets.

Attacker is able to enumerate all resources in the cluster, including secrets. This can help discover further vulnerabilities and pivot.

Default RBAC rules in the Alloy helm chart grant `get, list, watch` on `secrets` cluster-wide.

## Further potential attack vectors

### Cloud metadata service (IMDS)

On AWS/GCP/Azure, `remote.http` can query `169.254.169.254` with no extra permissions — it returns temporary IAM/service-account credentials. With IMDSv1 it's a single GET; IMDSv2 requires a PUT first to get a session token, which `remote.http` handles via `method = "PUT"`. Stolen credentials can then reach S3, IAM, or any cloud API from outside the cluster.

### Beyla attack surface

If attacker controls Alloy config AND Alloy runs with eBPF capabilities: they can wiretap all HTTPS traffic (post-TLS), capture full SQL queries with literal values, read HTTP bodies up to 64KB
