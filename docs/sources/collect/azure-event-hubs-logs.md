---
canonical: https://grafana.com/docs/alloy/latest/collect/azure-event-hubs-logs/
description: Learn how to deploy Grafana Alloy in an Azure Kubernetes Service cluster to collect logs from Azure Event Hubs and send them to Grafana Cloud
menuTitle: Collect Azure Event Hubs logs
title: Deploy Grafana Alloy in an Azure Kubernetes Service and collect Azure Event Hubs logs
weight: 300
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}} in an Azure Kubernetes Service and collect Azure Event Hubs logs

Deploy {{< param "FULL_PRODUCT_NAME" >}} in an Azure Kubernetes Service (AKS) cluster to collect logs from Azure Event Hubs and send them to Grafana Cloud.

{{< param "PRODUCT_NAME" >}} authenticates with Azure Event Hubs using Azure Workload Identity, which eliminates the need for secrets or connection strings.
Logs flow from your Azure resources to Azure Event Hubs, where {{< param "PRODUCT_NAME" >}} consumes them and forwards them to Grafana Cloud Loki.

## Before you begin

Ensure you have the following:

- Azure administrator access with `Microsoft.Authorization/roleAssignments/write` permissions, such as [Role Based Access Control Administrator](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#role-based-access-control-administrator) or [User Access Administrator](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#user-access-administrator)
- [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) installed and authenticated
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured to access your AKS cluster
- [Helm](https://helm.sh/docs/intro/install/) installed

## Prepare your Azure environment

Azure Event Hubs exposes different endpoints depending on the protocol:

| Protocol | Port         | When to use                                                         |
| -------- | ------------ | ------------------------------------------------------------------- |
| Kafka    | `9093` (TLS) | Applications send events using Kafka clients or Kafka Connect.      |
| AMQP     | `5671` (TLS) | Applications send events using Azure SDKs or AMQP client libraries. |

Both protocols use the hostname `<EVENTHUB_NAMESPACE>.servicebus.windows.net`.

The `loki.source.azure_event_hubs` component uses the Kafka protocol, so the examples in this procedure use port `9093`.

1. Use the Azure portal to [create or reuse a resource group](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal).

1. Create or reuse an [Azure Kubernetes Service](https://learn.microsoft.com/en-us/azure/aks/learn/quick-kubernetes-deploy-cli) (AKS) cluster.

1. Enable [Microsoft Entra Workload ID](https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview) and [OpenID Connect](https://learn.microsoft.com/en-us/azure/aks/use-oidc-issuer) in your AKS cluster.

   ```shell
   az aks update \
       --resource-group <RESOURCE_GROUP> \
       --name <AKS_CLUSTER_NAME> \
       --enable-oidc-issuer \
       --enable-workload-identity
   ```

   Replace the following:

   - _`<RESOURCE_GROUP>`_: Your Azure resource group
   - _`<AKS_CLUSTER_NAME>`_: The name of your AKS cluster

1. Retrieve the OIDC issuer URL for your cluster.
   You need this value when creating the federated credential.

   ```shell
   az aks show \
       --resource-group <RESOURCE_GROUP> \
       --name <AKS_CLUSTER_NAME> \
       --query "oidcIssuerProfile.issuerUrl" \
       --output tsv
   ```

   Replace the following:

   - _`<RESOURCE_GROUP>`_: Your Azure resource group
   - _`<AKS_CLUSTER_NAME>`_: The name of your AKS cluster

1. Create a [user-assigned managed identity](https://learn.microsoft.com/en-us/entra/identity/managed-identities-azure-resources/how-manage-user-assigned-managed-identities?pivots=identity-mi-methods-azp#create-a-user-assigned-managed-identity).

   ```shell
   az identity create \
       --resource-group <RESOURCE_GROUP> \
       --name <MANAGED_IDENTITY_NAME>
   ```

   Replace the following:

   - _`<RESOURCE_GROUP>`_: Your Azure resource group
   - _`<MANAGED_IDENTITY_NAME>`_: A name for your managed identity

1. Retrieve the client ID for your managed identity.
   You need this value for the ServiceAccount annotation and {{< param "PRODUCT_NAME" >}} configuration.

   ```shell
   az identity show \
       --resource-group <RESOURCE_GROUP> \
       --name <MANAGED_IDENTITY_NAME> \
       --query clientId \
       --output tsv
   ```

   Replace the following:

   - _`<RESOURCE_GROUP>`_: Your Azure resource group
   - _`<MANAGED_IDENTITY_NAME>`_: The name of your managed identity

1. Create a Kubernetes namespace for {{< param "PRODUCT_NAME" >}}.

   ```shell
   kubectl create namespace alloy
   ```

1. Create a Kubernetes ServiceAccount with the workload identity annotation.

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: alloy
     namespace: alloy
     annotations:
       azure.workload.identity/client-id: "<CLIENT_ID>"
   EOF
   ```

   Replace the following:

   - _`<CLIENT_ID>`_: The client ID from the previous step

   {{< collapse title="How Azure Workload Identity authentication works" >}}

   Azure Workload Identity connects three components:

   Kubernetes ServiceAccount
   : The ServiceAccount annotation `azure.workload.identity/client-id` specifies which managed identity the Pod can impersonate.

   Federated credential
   : The federated credential on the managed identity trusts tokens from your AKS cluster's OIDC issuer for a specific ServiceAccount (`system:serviceaccount:<namespace>:<name>`).

   Runtime token exchange
   : When the Pod runs, AKS issues a token for the ServiceAccount.
   Azure validates this token against the federated credential and returns an access token for the managed identity.
   {{< param "PRODUCT_NAME" >}} uses this token to authenticate to Event Hubs without a connection string.

   If authentication fails, verify:

   - The OIDC issuer URL matches your cluster
   - The `--subject` value matches `system:serviceaccount:<namespace>:<serviceaccount>`
   - The managed identity `clientId` matches the ServiceAccount annotation
   - The `--audiences` value is `api://AzureADTokenExchange`

   {{< /collapse >}}

1. Create a federated identity credential to link your managed identity with the Kubernetes ServiceAccount.

   ```shell
   az identity federated-credential create \
       --name alloy-federated-credential \
       --identity-name <MANAGED_IDENTITY_NAME> \
       --resource-group <RESOURCE_GROUP> \
       --issuer <OIDC_ISSUER_URL> \
       --subject system:serviceaccount:alloy:alloy \
       --audiences api://AzureADTokenExchange
   ```

   Replace the following:

   - _`<RESOURCE_GROUP>`_: Your Azure resource group
   - _`<MANAGED_IDENTITY_NAME>`_: The name of your managed identity
   - _`<OIDC_ISSUER_URL>`_: The OIDC issuer URL

<!-- vale Grafana.Headings = NO -->
## Configure Azure Event Hubs
<!-- vale Grafana.Headings = YES -->

1. Follow the steps to [Set up Azure Event Hubs](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/monitor-cloud-provider/azure/config-azure-logs/#set-up-azure-event-hubs).

1. Assign the `Azure Event Hubs Data Receiver` role to your managed identity.

   Get the managed identity principal ID:

   ```shell
   PRINCIPAL_ID=$(az identity show \
       --resource-group <RESOURCE_GROUP> \
       --name <MANAGED_IDENTITY_NAME> \
       --query principalId \
       --output tsv)
   ```

   Assign the role at namespace scope for least-privilege access:

   ```shell
   az role assignment create \
       --assignee $PRINCIPAL_ID \
       --role "Azure Event Hubs Data Receiver" \
       --scope /subscriptions/<SUBSCRIPTION_ID>/resourceGroups/<RESOURCE_GROUP>/providers/Microsoft.EventHub/namespaces/<EVENTHUB_NAMESPACE>
   ```

   Replace the following:

   - _`<MANAGED_IDENTITY_NAME>`_: The name of your managed identity
   - _`<RESOURCE_GROUP>`_: Your Azure resource group
   - _`<SUBSCRIPTION_ID>`_: Your Azure subscription ID
   - _`<EVENTHUB_NAMESPACE>`_: The name of your Event Hub namespace

   {{< collapse title="Role assignment scope options" >}}

   Assign the role at the smallest scope that meets your requirements:

   Namespace scope (recommended)
   : Grants access only to the specific Event Hub namespace. Use this for least-privilege access.

   Resource group or subscription scope
   : Grants access to all Event Hubs in the resource group or subscription. Use this only if {{< param "PRODUCT_NAME" >}} must read from multiple namespaces.

   Example resource group scope:

   ```shell
   az role assignment create \
       --assignee $PRINCIPAL_ID \
       --role "Azure Event Hubs Data Receiver" \
       --scope /subscriptions/<SUBSCRIPTION_ID>/resourceGroups/<RESOURCE_GROUP>
   ```

   To verify the assignment:

   ```shell
   az role assignment list --assignee $PRINCIPAL_ID --scope <SCOPE>
   ```

   {{< /collapse >}}

## Install {{% param "PRODUCT_NAME" %}}

1. Add the Grafana Helm repository.

   ```shell
   helm repo add grafana https://grafana.github.io/helm-charts
   helm repo update
   ```

1. Retrieve the tenant ID for your Azure subscription.

   ```shell
   az account show --query tenantId --output tsv
   ```

1. Create a `values.yaml` file with the following configuration.

   {{< admonition type="warning" >}}
   Don't store sensitive credentials directly in `values.yaml` or commit them to version control.
   For production environments, use a Kubernetes Secret with `secretKeyRef`, or an external secret manager such as HashiCorp Vault, Azure Key Vault, or the External Secrets Operator.
   {{< /admonition >}}

   The configuration uses port `9093` for the Azure Event Hubs Kafka-compatible endpoint.
   The `loki.source.azure_event_hubs` component in {{< param "PRODUCT_NAME" >}} requires the Kafka-compatible endpoint and doesn't support AMQP for this integration.

   The `authentication` block uses OAuth 2.0 with Azure Workload Identity through the federated credential you created earlier.
   Kafka-compatible endpoints use SASL/OAUTHBEARER with Microsoft Entra ID tokens, so you don't need an Event Hub connection string.

   ```yaml
   serviceAccount:
     create: false
     name: alloy

   controller:
     type: statefulset
     replicas: 1
     podLabels:
       azure.workload.identity/use: "true"

   alloy:
     extraEnv:
       - name: "AZURE_CLIENT_ID"
         value: "<CLIENT_ID>"
       - name: "AZURE_TENANT_ID"
         value: "<TENANT_ID>"
     configMap:
       content: |
         loki.source.azure_event_hubs "azure" {
           fully_qualified_namespace = "<EVENTHUB_NAMESPACE>.servicebus.windows.net:9093"
           event_hubs                = ["<EVENTHUB_NAME>"]

           authentication {
             mechanism = "oauth"
           }

           use_incoming_timestamp = true
           labels = {
             job = "integrations/azure_event_hubs",
           }
           forward_to = [loki.write.grafana_cloud.receiver]
         }

         loki.write "grafana_cloud" {
           endpoint {
             url = "<GRAFANA_CLOUD_LOKI_URL>"
             basic_auth {
               username = "<GRAFANA_CLOUD_LOKI_USERNAME>"
               password = "<GRAFANA_CLOUD_API_KEY>"
             }
           }
         }
   ```

   Replace the following:

   - _`<CLIENT_ID>`_: Your managed identity client ID
   - _`<TENANT_ID>`_: Your Azure tenant ID
   - _`<EVENTHUB_NAMESPACE>`_: Your Event Hub namespace name
   - _`<EVENTHUB_NAME>`_: Your Event Hub name
   - _`<GRAFANA_CLOUD_LOKI_URL>`_: Your Grafana Cloud Loki endpoint, such as `https://logs-prod-us-central1.grafana.net/loki/api/v1/push`
   - _`<GRAFANA_CLOUD_LOKI_USERNAME>`_: Your Grafana Cloud Loki username
   - _`<GRAFANA_CLOUD_API_KEY>`_: Your Grafana Cloud API key

1. Install {{< param "PRODUCT_NAME" >}} using Helm.

   ```shell
   helm install alloy grafana/alloy \
       --namespace alloy \
       -f values.yaml
   ```

## Verify the installation

1. Check that the {{< param "PRODUCT_NAME" >}} Pod is running.

   ```shell
   kubectl get pods -n alloy
   ```

   You should see output similar to:

   ```text
   NAME      READY   STATUS    RESTARTS   AGE
   alloy-0   1/1     Running   0          1m
   ```

1. Check the {{< param "PRODUCT_NAME" >}} logs for any errors.

   ```shell
   kubectl logs -n alloy -l app.kubernetes.io/name=alloy
   ```

   {{< collapse title="Quick validation tips" >}}

   - Verify authentication and connection:

     ```shell
     kubectl logs -n alloy -l app.kubernetes.io/name=alloy | grep -E -i "authenticated|connected|sasl"
     ```

   - Push a test event to the Event Hub and confirm a matching log appears in Grafana Explore within approximately one minute.

   - If errors occur, verify the role assignment:

     ```shell
     az role assignment list --assignee <PRINCIPAL_ID> --scope <SCOPE>
     ```

   {{< /collapse >}}

1. In Grafana Cloud, navigate to **Explore** and select your Loki data source to view the incoming logs.

## Optional: Configure {{% param "PRODUCT_NAME" %}} to extract labels from Azure Event Hubs

By default, the {{< param "PRODUCT_NAME" >}} configuration doesn't extract labels from the Event Hubs log lines.

You can configure {{< param "PRODUCT_NAME" >}} to use `loki.process` to extract labels such as `resourceId`, `category`, and `resourceGroup` from Azure resource logs.

The `loki.process` component uses three stages to transform the logs:

1. **`stage.json`**: Parses the log line as JSON and extracts the `resourceId` and `category` fields.
1. **`stage.regex`**: Parses the `resourceId` to extract resource details like `subscriptionId`, `resourceGroup`, and `resourceName`.
1. **`stage.labels`**: Creates Loki labels from the extracted values for easier querying.

Update the `alloy.configMap.content` section in your `values.yaml` file with the following configuration.

When set to `true`, the `use_incoming_timestamp` setting uses the event's timestamp, such as Event Hubs `EnqueuedTimeUtc` or a `timestamp` field in the payload.
The default is `false`, which uses {{< param "PRODUCT_NAME" >}} ingestion time.
Keep the default if your events lack reliable timestamps.

```yaml
alloy:
  configMap:
    content: |
      loki.source.azure_event_hubs "azure" {
        fully_qualified_namespace = "<EVENTHUB_NAMESPACE>.servicebus.windows.net:9093"
        event_hubs                = ["<EVENTHUB_NAME>"]

        authentication {
          mechanism = "oauth"
        }

        use_incoming_timestamp = true
        labels = {
          job = "integrations/azure_event_hubs",
        }
        forward_to = [loki.process.azure_logs.receiver]
      }

      loki.process "azure_logs" {
        stage.json {
          expressions = {
            resourceId = "resourceId",
            category   = "category",
          }
        }

        stage.regex {
          expression = "(?i)/subscriptions/(?P<subscriptionId>[^/]+)/resourcegroups/(?P<resourceGroup>[^/]+)/providers/(?P<providerNamespace>[^/]+)/(?P<resourceType>[^/]+)/(?P<resourceName>[^/]+)"
          source     = "resourceId"
        }

        stage.labels {
          values = {
            category       = "",
            resourceId     = "",
            resourceGroup  = "",
            service_name   = "resourceName",
          }
        }

        forward_to = [loki.write.grafana_cloud.receiver]
      }

      loki.write "grafana_cloud" {
        endpoint {
          url = "<GRAFANA_CLOUD_LOKI_URL>"
          basic_auth {
            username = "<GRAFANA_CLOUD_LOKI_USERNAME>"
            password = "<GRAFANA_CLOUD_API_KEY>"
          }
        }
      }
```

Replace the following:

- _`<EVENTHUB_NAMESPACE>`_: Your Event Hub namespace name
- _`<EVENTHUB_NAME>`_: Your Event Hub name
- _`<GRAFANA_CLOUD_LOKI_URL>`_: Your Grafana Cloud Loki endpoint, such as `https://logs-prod-us-central1.grafana.net/loki/api/v1/push`
- _`<GRAFANA_CLOUD_LOKI_USERNAME>`_: Your Grafana Cloud Loki username
- _`<GRAFANA_CLOUD_API_KEY>`_: Your Grafana Cloud API key

After updating the configuration, upgrade the Helm release:

```shell
helm upgrade alloy grafana/alloy \
    --namespace alloy \
    -f values.yaml
```

## Next steps

- [Query logs in Grafana Explore](https://grafana.com/docs/grafana-cloud/visualizations/explore/) to analyze your Azure Event Hubs data
- [Create dashboards](https://grafana.com/docs/grafana-cloud/visualizations/dashboards/) to visualize your Azure logs
- [Set up alerts](https://grafana.com/docs/grafana-cloud/alerting/) to get notified about important events in your Azure resources

## Related documentation

For more information about the {{< param "PRODUCT_NAME" >}} components, refer to:

- [`loki.source.azure_event_hubs`](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.azure_event_hubs/): Receives logs from Azure Event Hubs
- [`loki.process`](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.process/): Processes and transforms log entries using configurable stages
- [`loki.write`](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.write/): Sends logs to Loki or Grafana Cloud Logs

For more information about Azure monitoring in Grafana Cloud, refer to:

- [Monitor Microsoft Azure](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/monitor-cloud-provider/azure/): Overview of Azure monitoring options
- [Troubleshoot Azure integrations](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/monitor-cloud-provider/azure/troubleshoot/): Common issues and solutions
- [Collect Azure metrics](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/monitor-cloud-provider/azure/config-azure-metrics/): Configure Azure metrics collection