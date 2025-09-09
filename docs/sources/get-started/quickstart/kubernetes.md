---
canonical: https://grafana.com/docs/alloy/latest/get-started/quickstart/kubernetes/
description: Get Kubernetes cluster metrics into Grafana quickly with Grafana Alloy
menuTitle: Quickstart Kubernetes monitoring
title: Quickstart Kubernetes monitoring with Grafana Alloy
weight: 300
---

# Quickstart Kubernetes monitoring with {{% param "FULL_PRODUCT_NAME" %}}

Get your Kubernetes cluster metrics flowing to Grafana quickly.
This guide shows you how to deploy {{< param "PRODUCT_NAME" >}} on Kubernetes, configure it to collect essential cluster metrics (nodes, pods, services, containers), and visualize them in Grafana.

## Before you begin

Before you begin, ensure you have the following:

- A Kubernetes cluster with administrative access
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/) configured to access your cluster
- [Helm](https://helm.sh/docs/intro/install/) installed on your local machine
- A Grafana instance with Prometheus data source configured

  If you don't have a Grafana instance yet, you can:

  - [Set up Grafana Cloud](https://grafana.com/docs/grafana-cloud/account-management/create-account/) for a managed solution, or
  - [Install Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/installation/) on your own infrastructure

  To configure a Prometheus data source in Grafana, refer to [Add a Prometheus data source](https://grafana.com/docs/grafana/latest/datasources/prometheus/configure-prometheus-data-source/).

## Step 1: Deploy {{% param "PRODUCT_NAME" %}}

1. Add the Grafana Helm chart repository:

   ```shell
   helm repo add grafana https://grafana.github.io/helm-charts
   ```

1. Update the Helm chart repository:

   ```shell
   helm repo update
   ```

1. Create a namespace for {{< param "PRODUCT_NAME" >}}:

   ```shell
   kubectl create namespace alloy
   ```

1. Create a `values.yaml` file with your {{< param "PRODUCT_NAME" >}} configuration:

   ```yaml
   alloy:
     configMap:
       content: |
         // Basic Kubernetes cluster monitoring configuration

         // Discover and collect metrics from Kubernetes nodes
         discovery.kubernetes "nodes" {
           role = "node"
         }

         // Discover and collect metrics from Kubernetes pods
         discovery.kubernetes "pods" {
           role = "pod"
         }

         // Discover and collect metrics from Kubernetes services
         discovery.kubernetes "services" {
           role = "service"
         }

         // Collect node-level metrics (kubelet, cAdvisor)
         prometheus.scrape "nodes" {
           targets = discovery.kubernetes.nodes.targets
           forward_to = [prometheus.remote_write.grafana_cloud.receiver]
           scrape_interval = "30s"
         }

         // Collect pod metrics from discovered pods
         prometheus.scrape "pods" {
           targets = discovery.kubernetes.pods.targets
           forward_to = [prometheus.remote_write.grafana_cloud.receiver]
           scrape_interval = "30s"
         }

         // Collect service metrics from discovered services
         prometheus.scrape "services" {
           targets = discovery.kubernetes.services.targets
           forward_to = [prometheus.remote_write.grafana_cloud.receiver]
           scrape_interval = "30s"
         }

         // This block sends your metrics to Grafana Cloud
         // Replace the placeholders with your actual Grafana Cloud values
         prometheus.remote_write "grafana_cloud" {
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

   - _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
   - _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
   - _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

   {{< admonition type="tip" >}}
   To find your `remote_write` connection details if you are using a Grafana Cloud connection:

   1. Log in to [Grafana Cloud](https://grafana.com/).
   1. Navigate to **Connections** > **Add new connection** > **Hosted Prometheus metrics**.
   1. Copy the following details:
      - **URL** (Remote Write Endpoint)
      - **Username**
      - **Password/API Key**

   If you are using a self-managed Grafana connection, the _`<PROMETHEUS_REMOTE_WRITE_URL>`_ should be `"http://<YOUR-PROMETHEUS-SERVER-URL>:9090/api/v1/write"`.
   The _`<USERNAME>`_ and _`<PASSWORD>`_ are what you set up when you installed Grafana and Prometheus.
   {{< /admonition >}}

1. Deploy {{< param "PRODUCT_NAME" >}}:

   ```shell
   helm install alloy grafana/alloy \
     --namespace alloy \
     --values values.yaml
   ```

## Step 2: Verify the deployment

Verify that {{< param "PRODUCT_NAME" >}} is running successfully:

```shell
kubectl get pods -n alloy
```

You should see the {{< param "PRODUCT_NAME" >}} Pod in `Running` status.

{{< admonition type="note" >}}
If the deployment fails, check that your cluster has sufficient resources and that you have the necessary permissions to create resources in the `alloy` namespace.
{{< /admonition >}}

## Step 3: Configure monitoring (optional)

If you need to update the configuration after deployment:

1. Edit your `values.yaml` file to update the configuration.

1. Update the {{< param "PRODUCT_NAME" >}} deployment:

   ```shell
   helm upgrade alloy grafana/alloy \
     --namespace alloy \
     --values values.yaml
   ```

### Troubleshoot the deployment

If the deployment fails or metrics aren't flowing, check these common issues:

```shell
kubectl describe pod -n alloy -l app.kubernetes.io/name=alloy
kubectl logs -n alloy deployment/alloy | grep -i error
kubectl auth can-i get pods --as=system:serviceaccount:alloy:alloy
```

Common issues:

- **RBAC permissions**: Ensure the service account has permissions to discover Kubernetes resources
- **Network policies**: Verify that {{< param "PRODUCT_NAME" >}} can reach your Prometheus endpoint
- **Resource limits**: Check if the Pod has sufficient CPU and memory resources
- **Configuration errors**: Validate the configuration syntax in the Helm values

## Step 4: Visualize your metrics in Grafana

Within a few minutes of deploying {{< param "PRODUCT_NAME" >}}, your Kubernetes metrics should appear in Grafana.

### Visualize in Grafana Cloud

1. Log in to your [Grafana Cloud](https://grafana.com/) instance.
1. Navigate to **Connections** > **Infrastructure** > **Kubernetes**.
1. Click **Install Integration** if not already installed.
1. Go to **Dashboards** and look for Kubernetes-related dashboards such as:
   - **Kubernetes / Compute Resources / Cluster**
   - **Kubernetes / Compute Resources / Namespace (Pods)**
   - **Kubernetes / Compute Resources / Node (Pods)**

Alternatively, import a community dashboard:

1. Go to **Dashboards** > **New** > **Import**.
1. Enter dashboard ID: `8588` (Kubernetes Cluster Monitoring).
1. Click **Load**.
1. Select your Prometheus data source and click **Import**.

### Visualize in self-managed Grafana

1. Open your Grafana instance.
1. Go to **Dashboards** > **New** > **Import**.
1. Enter dashboard ID `8588` or download the JSON from the [Grafana dashboard library](https://grafana.com/grafana/dashboards/8588-kubernetes-cluster-monitoring-via-prometheus/).
1. Click **Load**.
1. Select your Prometheus data source and click **Import**.

### What you should see

The dashboard displays comprehensive Kubernetes cluster metrics:

- **Cluster Overview**: Node count, Pod count, CPU and memory usage
- **Node Metrics**: Individual node CPU, memory, disk, and network utilization
- **Pod Metrics**: Pod resource usage, restart counts, and status
- **Container Metrics**: Container CPU, memory usage, and limits
- **Network Metrics**: Network I/O and traffic patterns across the cluster

{{< admonition type="note" >}}
Metrics should appear in Grafana within a few minutes of deploying {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Troubleshoot

If metrics don't appear in Grafana after several minutes, check these common issues:

### Verify {{< param "PRODUCT_NAME" >}} is running

```shell
kubectl get pods -n alloy
kubectl logs -n alloy deployment/alloy --tail=50
```

Look for error messages about configuration parsing, network connectivity, or authentication.

### Check configuration syntax

Validate your configuration by examining the logs:

```shell
kubectl logs -n alloy deployment/alloy | grep -i "error\|failed\|invalid"
```

### Test network connectivity

Verify that {{< param "PRODUCT_NAME" >}} can reach your Prometheus endpoint:

```shell
kubectl exec -n alloy deployment/alloy -- wget --spider -q "<PROMETHEUS_REMOTE_WRITE_URL>"
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.

### Verify RBAC permissions

Check that {{< param "PRODUCT_NAME" >}} has the necessary permissions to discover Kubernetes resources:

```shell
kubectl auth can-i get nodes --as=system:serviceaccount:alloy:alloy
kubectl auth can-i get pods --as=system:serviceaccount:alloy:alloy
kubectl auth can-i get services --as=system:serviceaccount:alloy:alloy
```

### Check the {{< param "PRODUCT_NAME" >}} UI

Access the {{< param "PRODUCT_NAME" >}} debug UI to inspect component health:

```shell
kubectl port-forward -n alloy deployment/alloy 12345:12345
```

Then open `http://localhost:12345` and check:

1. **Graph** tab for component connections
2. Component health indicators for any errors

### Common solutions

- **Pod won't start**: Check resource limits and node capacity: `kubectl describe pod -n alloy`
- **RBAC errors**: The Helm chart should create appropriate permissions automatically
- **Network timeout**: Verify firewall settings and network policies
- **Authentication failed**: Regenerate your API token in Grafana Cloud
- **No metrics in Grafana**: Wait a few minutes for the first scrape cycle to complete

### Kubernetes-specific troubleshooting

- **Service discovery not working**: Verify that the {{< param "PRODUCT_NAME" >}} service account has cluster-wide read permissions
- **Resource discovery timeouts**: Consider adding namespace filtering to reduce the scope of discovery
- **High cardinality metrics**: Use label filtering to reduce metric volume from noisy applications
- **Performance issues**: Adjust scrape intervals and consider deploying as a DaemonSet for large clusters

## Next steps

- [Set up alerting rules](https://grafana.com/docs/grafana/latest/alerting/) to get notified when cluster metrics exceed thresholds
- [Configure log collection](https://grafana.com/docs/alloy/latest/reference/components/loki/) from Kubernetes pods and containers
- [Add distributed tracing](https://grafana.com/docs/alloy/latest/reference/components/otelcol/) to monitor application performance
- [Monitor specific applications](https://grafana.com/docs/alloy/latest/reference/components/prometheus/) running in your cluster
- [Explore advanced Kubernetes configurations](https://grafana.com/docs/alloy/latest/configure/kubernetes/) for production deployments

### Learn more

- [{{< param "FULL_PRODUCT_NAME" >}} documentation](https://grafana.com/docs/alloy/latest/)
- [Kubernetes monitoring best practices](https://grafana.com/docs/grafana/latest/fundamentals/intro-prometheus/)
- [Grafana dashboard best practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
- [Observability with Grafana](https://grafana.com/docs/grafana/latest/fundamentals/)
