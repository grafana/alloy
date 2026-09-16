---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.cadvisor/
aliases:
  - ../prometheus.exporter.cadvisor/ # /docs/alloy/latest/reference/components/prometheus.exporter.cadvisor/
description: Learn about prometheus.exporter.cadvisor
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-09-15
title: prometheus.exporter.cadvisor
---

# `prometheus.exporter.cadvisor`

The `prometheus.exporter.cadvisor` component collects container metrics with [cAdvisor](https://github.com/google/cadvisor).

{{< docs/shared lookup="reference/components/exporter-clustering-warning.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< admonition type="note" >}}
The `prometheus.exporter.cadvisor` component only works on Linux systems.
On other operating systems, {{< param "PRODUCT_NAME" >}} writes a warning to the logs and the component doesn't collect container metrics.
{{< /admonition >}}

The component requires specific permissions and configuration depending on your deployment environment.

{{< tabs >}}
{{< tab-content name="Linux binary" >}}

When you run {{< param "PRODUCT_NAME" >}} as a Linux binary, systemd service, or as the `alloy` user, grant the `alloy` user permissions to access the container runtime socket and related directories to collect metrics from containers on the host.

The component works with Docker, `containerd`, CRI-O, and systemd container runtimes.

For Docker, grant permissions using one of these approaches:

- **Add to docker group (recommended)**: Add the `alloy` user to the `docker` group:

  ```bash
  sudo usermod -aG docker alloy
  ```

  Then restart the {{< param "PRODUCT_NAME" >}} service:

  ```bash
  sudo systemctl restart alloy
  ```

  {{< admonition type="note" >}}
  The `docker` group grants privileges equivalent to the `root` user.
  For more information about the security implications, refer to [Docker security](https://docs.docker.com/engine/security/#docker-daemon-attack-surface).
  {{< /admonition >}}

- **Using ACLs**: Grant the `alloy` user read and execute permissions to `/var/lib/docker/`:

  ```bash
  sudo setfacl -R -m d:u:alloy:rx /var/lib/docker/
  sudo setfacl -R -m u:alloy:rx /var/lib/docker/
  ```

  {{< admonition type="note" >}}
  You must rerun these commands when adding containers, as they don't automatically apply to newly created directories.
  {{< /admonition >}}

- **Running as root**: Modify the systemd service to run {{< param "PRODUCT_NAME" >}} as root:

  ```bash
  sudo systemctl edit alloy.service
  ```

  Add the following override:

  ```ini
  [Service]
  User=root
  ```

  {{< admonition type="caution" >}}
  Running {{< param "PRODUCT_NAME" >}} as root has security implications.
  Only use this approach if necessary for your environment.
  {{< /admonition >}}

For more information about running {{< param "PRODUCT_NAME" >}} without root privileges, refer to [Access and permissions for {{< param "FULL_PRODUCT_NAME" >}} on Linux][access-linux].

[access-linux]: ../../../../access_permissions/linux/

{{< /tab-content >}}
{{< tab-content name="Kubernetes" >}}

When you run {{< param "PRODUCT_NAME" >}} in Kubernetes to collect container metrics, deploy {{< param "PRODUCT_NAME" >}} as a DaemonSet to collect metrics from each node.

The DaemonSet requires:

- **Host network access**: To access the container runtime
- **Volume mounts**: Access to the container runtime socket and system directories
- **Security context**: Privileged access or specific capabilities

For detailed guidance, refer to the [Kubernetes DaemonSet example](#kubernetes-daemonset-example) section.

{{< /tab-content >}}
{{< tab-content name="Docker container" >}}

When you run {{< param "PRODUCT_NAME" >}} itself as a Docker container to monitor other containers on the host, the {{< param "PRODUCT_NAME" >}} container requires:

- **Privileged mode**: Access to host resources
- **Volume mounts**: Access to the container runtime socket and system directories

For a complete Docker container deployment example, refer to the [Docker deployment example](#docker-deployment-example) section.

{{< /tab-content >}}
{{< /tabs >}}

## Usage

```alloy
prometheus.exporter.cadvisor "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.cadvisor`:

| Name                           | Type           | Description                                                                                                         | Default                             | Required |
| ------------------------------ | -------------- | ------------------------------------------------------------------------------------------------------------------- | ----------------------------------- | -------- |
| `allowlisted_container_labels` | `list(string)` | Allowlist of container labels to convert to Prometheus labels.                                                      | `[]`                                | no       |
| `containerd_host`              | `string`       | The `containerd` endpoint.                                                                                          | `"/run/containerd/containerd.sock"` | no       |
| `containerd_namespace`         | `string`       | The `containerd` namespace.                                                                                         | `"k8s.io"`                          | no       |
| `disable_root_cgroup_stats`    | `bool`         | Disable collecting root `cgroup` stats.                                                                             | `false`                             | no       |
| `disabled_metrics`             | `list(string)` | List of metrics to disable. If set, this list overrides the default disabled metrics.                               | _see below_                         | no       |
| `docker_host`                  | `string`       | Docker endpoint.                                                                                                    | `"unix:///var/run/docker.sock"`     | no       |
| `docker_only`                  | `bool`         | Only report docker containers in addition to root stats.                                                            | `false`                             | no       |
| `docker_tls_ca`                | `string`       | Path to a trusted CA for TLS connection to docker.                                                                  | `"ca.pem"`                          | no       |
| `docker_tls_cert`              | `string`       | Path to client certificate for TLS connection to docker.                                                            | `"cert.pem"`                        | no       |
| `docker_tls_key`               | `string`       | Path to private key for TLS connection to docker.                                                                   | `"key.pem"`                         | no       |
| `enabled_metrics`              | `list(string)` | List of metrics to enable. If set, this list overrides `disabled_metrics`.                                          | `[]`                                | no       |
| `env_metadata_allowlist`       | `list(string)` | Allowlist of environment variable keys, matched by prefix, to collect for containers.                               | `[]`                                | no       |
| `perf_events_config`           | `string`       | Path to a JSON file containing the configuration of performance events to measure.                                  | `""`                                | no       |
| `raw_cgroup_prefix_allowlist`  | `list(string)` | List of `cgroup` path prefixes to collect, even when you set `docker_only`.                                         | `[]`                                | no       |
| `resctrl_interval`             | `duration`     | Interval to update `resctrl mon` groups.                                                                            | `"0"`                               | no       |
| `storage_duration`             | `duration`     | Length of time to keep data stored in memory.                                                                       | `"2m"`                              | no       |
| `store_container_labels`       | `bool`         | Whether to convert container labels and environment variables into labels on Prometheus metrics for each container. | `true`                              | no       |
| `use_docker_tls`               | `bool`         | Use TLS to connect to docker.                                                                                       | `false`                             | no       |

For `allowlisted_container_labels` to take effect, set `store_container_labels` to `false`.

If a container uses the `overlayfs` storage driver, set the `containerd_host` attribute correctly to retrieve its metrics.

`env_metadata_allowlist` works only with the `containerd` and Docker runtimes.

If you don't set `perf_events_config`, {{< param "PRODUCT_NAME" >}} doesn't measure `perf` events.

A `resctrl_interval` of `0` disables updating mon groups.

The values for `enabled_metrics` and `disabled_metrics` don't correspond to Prometheus metrics, but to kinds of metrics that cAdvisor exposes or omits.
The values that you can use are:

{{< column-list >}}

- `"advtcp"`
- `"app"`
- `"cpu"`
- `"cpuLoad"`
- `"cpu_topology"`
- `"cpuset"`
- `"disk"`
- `"diskIO"`
- `"hugetlb"`
- `"memory"`
- `"memory_numa"`
- `"network"`
- `"oom_event"`
- `"percpu"`
- `"perf_event"`
- `"pressure"`
- `"process"`
- `"referenced_memory"`
- `"resctrl"`
- `"sched"`
- `"tcp"`
- `"udp"`

{{< /column-list >}}

By default, cAdvisor disables the following metric kinds:

{{< column-list >}}

- `"advtcp"`
- `"cpu_topology"`
- `"cpuset"`
- `"hugetlb"`
- `"memory_numa"`
- `"process"`
- `"referenced_memory"`
- `"resctrl"`
- `"sched"`
- `"tcp"`
- `"udp"`

{{< /column-list >}}

## Blocks

The `prometheus.exporter.cadvisor` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.cadvisor` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.cadvisor` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.cadvisor` doesn't expose any component-specific debug metrics.

## Examples

The following examples show you how to configure `prometheus.exporter.cadvisor` and how to deploy {{< param "PRODUCT_NAME" >}} in Docker and Kubernetes.

### Component configuration

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.cadvisor`:

```alloy
prometheus.exporter.cadvisor "example" {
  docker_host = "unix:///var/run/docker.sock"

  storage_duration = "5m"
}

// Configure a prometheus.scrape component to collect cadvisor metrics.
prometheus.scrape "scraper" {
  targets    = prometheus.exporter.cadvisor.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

### Docker deployment example

To collect container metrics with {{< param "PRODUCT_NAME" >}} in Docker, you must mount the necessary host directories and run the container in privileged mode.

{{< admonition type="note" >}}
The `prometheus.exporter.cadvisor` component only works when the {{< param "PRODUCT_NAME" >}} container runs on a Linux Docker host.
Docker Desktop for Mac and Docker Desktop for Windows run containers in a Linux VM, which prevents direct host monitoring.
{{< /admonition >}}

The following Docker Compose example shows the required configuration:

```yaml
services:
  alloy:
    image: grafana/alloy:latest
    privileged: true
    ports:
      - "12345:12345"
    volumes:
      - ./config.alloy:/etc/alloy/config.alloy
      - /var/run/docker.sock:/var/run/docker.sock
      - /:/rootfs:ro
      - /var/run:/var/run:rw
      - /sys:/sys:ro
      - /var/lib/docker/:/var/lib/docker:ro
      - /dev/disk/:/dev/disk:ro
    command: run --server.http.listen-addr=0.0.0.0:12345 --storage.path=/var/lib/alloy/data /etc/alloy/config.alloy
```

The required volume mounts are:

- `/var/run/docker.sock`: Docker socket for container discovery and API access
- `/`: Read-only access to the host root filesystem for system metrics
- `/var/run`: Read-write access to host runtime data for container state
- `/sys`: Read-only access to host system information for `cgroup` and device metrics
- `/var/lib/docker/`: Read-only access to the Docker storage directory for container metadata and layer information
- `/dev/disk/`: Read-only access to disk device information for disk I/O metrics

{{< admonition type="caution" >}}
Running in privileged mode grants the container access to all host devices.
Only use privileged mode when necessary and ensure proper network isolation.
{{< /admonition >}}

For a complete working example with Grafana and Prometheus, refer to the [alloy-scenarios repository][alloy-scenarios].

[alloy-scenarios]: https://github.com/grafana/alloy-scenarios/tree/main/docker-monitoring

### Kubernetes DaemonSet example

When you run {{< param "PRODUCT_NAME" >}} in Kubernetes, deploy it as a DaemonSet to collect container metrics from each node.

The following example shows the required configuration for a DaemonSet:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: alloy
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: alloy
  template:
    metadata:
      labels:
        app: alloy
    spec:
      serviceAccountName: alloy
      hostNetwork: true
      hostPID: true
      containers:
        - name: alloy
          image: grafana/alloy:latest
          args:
            - run
            - /etc/alloy/config.alloy
            - --server.http.listen-addr=0.0.0.0:12345
            - --storage.path=/var/lib/alloy/data
          ports:
            - containerPort: 12345
              name: http-metrics
          securityContext:
            privileged: true
          volumeMounts:
            - name: config
              mountPath: /etc/alloy
            - name: rootfs
              mountPath: /rootfs
              readOnly: true
            - name: var-run
              mountPath: /var/run
            - name: sys
              mountPath: /sys
              readOnly: true
            - name: docker
              mountPath: /var/lib/docker
              readOnly: true
            - name: dev-disk
              mountPath: /dev/disk
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: alloy-config
        - name: rootfs
          hostPath:
            path: /
        - name: var-run
          hostPath:
            path: /var/run
        - name: sys
          hostPath:
            path: /sys
        - name: docker
          hostPath:
            path: /var/lib/docker
        - name: dev-disk
          hostPath:
            path: /dev/disk
```

Key configuration requirements:

- **hostNetwork: true**: Allows access to the host network stack
- **hostPID: true**: Enables process-level metrics collection
- **privileged: true**: Grants access to host resources
- **Volume mounts**: Provide access to container runtime and system directories

{{< admonition type="note" >}}
For container runtimes other than Docker, such as `containerd` or CRI-O, adjust the volume mounts and `docker_host` or `containerd_host` arguments accordingly.
{{< /admonition >}}

{{< admonition type="caution" >}}
Running containers with privileged access in Kubernetes has security implications.
Consider using Pod Security Standards and RBAC to limit exposure, and only deploy to dedicated monitoring namespaces.
{{< /admonition >}}

For more information about deploying {{< param "PRODUCT_NAME" >}} on Kubernetes, refer to [Deploy {{< param "FULL_PRODUCT_NAME" >}}][deploy].

[deploy]: ../../../../set-up/deploy/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.cadvisor` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
