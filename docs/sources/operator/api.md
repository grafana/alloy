---
aliases:
- /docs/agent/latest/operator/crd/
- /docs/grafana-cloud/agent/operator/api/
- /docs/grafana-cloud/monitor-infrastructure/agent/operator/api/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/operator/api/
- /docs/grafana-cloud/send-data/agent/operator/api/
canonical: https://grafana.com/docs/agent/latest/operator/api/
title: Custom Resource Definition Reference
description: Learn about the Grafana Agent API
weight: 500
---
# Custom Resource Definition Reference
## Resource Types:
* [Deployment](#monitoring.grafana.com/v1alpha1.Deployment) 
* [GrafanaAgent](#monitoring.grafana.com/v1alpha1.GrafanaAgent) 
* [IntegrationsDeployment](#monitoring.grafana.com/v1alpha1.IntegrationsDeployment) 
* [LogsDeployment](#monitoring.grafana.com/v1alpha1.LogsDeployment) 
* [MetricsDeployment](#monitoring.grafana.com/v1alpha1.MetricsDeployment) 
### Deployment <a name="monitoring.grafana.com/v1alpha1.Deployment"></a>
Deployment is a set of discovered resources relative to a GrafanaAgent. The tree of resources contained in a Deployment form the resource hierarchy used for reconciling a GrafanaAgent. 
#### Fields
|Field|Description|
|-|-|
|apiVersion|string<br/>`monitoring.grafana.com/v1alpha1`|
|kind|string<br/>`Deployment`|
|`Agent`<br/>_[GrafanaAgent](#monitoring.grafana.com/v1alpha1.GrafanaAgent)_|  Root resource in the deployment.  |
|`Metrics`<br/>_[[]MetricsDeployment](#monitoring.grafana.com/v1alpha1.MetricsDeployment)_|  Metrics resources discovered by Agent.  |
|`Logs`<br/>_[[]LogsDeployment](#monitoring.grafana.com/v1alpha1.LogsDeployment)_|  Logs resources discovered by Agent.  |
|`Integrations`<br/>_[[]IntegrationsDeployment](#monitoring.grafana.com/v1alpha1.IntegrationsDeployment)_|  Integrations resources discovered by Agent.  |
|`Secrets`<br/>_[github.com/grafana/agent/internal/static/operator/assets.SecretStore](https://pkg.go.dev/github.com/grafana/agent/internal/static/operator/assets#SecretStore)_|  The full list of Secrets referenced by resources in the Deployment.  |
### GrafanaAgent <a name="monitoring.grafana.com/v1alpha1.GrafanaAgent"></a>
(Appears on:[Deployment](#monitoring.grafana.com/v1alpha1.Deployment))
GrafanaAgent defines a Grafana Agent deployment. 
#### Fields
|Field|Description|
|-|-|
|apiVersion|string<br/>`monitoring.grafana.com/v1alpha1`|
|kind|string<br/>`GrafanaAgent`|
|`metadata`<br/>_[Kubernetes meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_|     Refer to the Kubernetes API documentation for the fields of the `metadata` field. |
|`spec`<br/>_[GrafanaAgentSpec](#monitoring.grafana.com/v1alpha1.GrafanaAgentSpec)_|  Spec holds the specification of the desired behavior for the Grafana Agent cluster.  |
|`logLevel`<br/>_string_|  LogLevel controls the log level of the generated pods. Defaults to &#34;info&#34; if not set.  |
|`logFormat`<br/>_string_|  LogFormat controls the logging format of the generated pods. Defaults to &#34;logfmt&#34; if not set.  |
|`apiServer`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.APIServerConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.APIServerConfig)_|  APIServerConfig lets you specify a host and auth methods to access the Kubernetes API server. If left empty, the Agent assumes that it is running inside of the cluster and will discover API servers automatically and use the pod&#39;s CA certificate and bearer token file at /var/run/secrets/kubernetes.io/serviceaccount.  |
|`podMetadata`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.EmbeddedObjectMetadata](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.EmbeddedObjectMetadata)_|  PodMetadata configures Labels and Annotations which are propagated to created Grafana Agent pods.  |
|`version`<br/>_string_|  Version of Grafana Agent to be deployed.  |
|`paused`<br/>_bool_|  Paused prevents actions except for deletion to be performed on the underlying managed objects.  |
|`image`<br/>_string_|  Image, when specified, overrides the image used to run Agent. Specify the image along with a tag. You still need to set the version to ensure Grafana Agent Operator knows which version of Grafana Agent is being configured.  |
|`configReloaderVersion`<br/>_string_|  Version of Config Reloader to be deployed.  |
|`configReloaderImage`<br/>_string_|  Image, when specified, overrides the image used to run Config Reloader. Specify the image along with a tag. You still need to set the version to ensure Grafana Agent Operator knows which version of Grafana Agent is being configured.  |
|`imagePullSecrets`<br/>_[[]Kubernetes core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core)_|  ImagePullSecrets holds an optional list of references to Secrets within the same namespace used for pulling the Grafana Agent image from registries. More info: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod  |
|`storage`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.StorageSpec](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.StorageSpec)_|  Storage spec to specify how storage will be used.  |
|`volumes`<br/>_[[]Kubernetes core/v1.Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core)_|  Volumes allows configuration of additional volumes on the output StatefulSet definition. The volumes specified are appended to other volumes that are generated as a result of StorageSpec objects.  |
|`volumeMounts`<br/>_[[]Kubernetes core/v1.VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core)_|  VolumeMounts lets you configure additional VolumeMounts on the output StatefulSet definition. Specified VolumeMounts are appended to other VolumeMounts generated as a result of StorageSpec objects in the Grafana Agent container.  |
|`resources`<br/>_[Kubernetes core/v1.ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core)_|  Resources holds requests and limits for individual pods.  |
|`nodeSelector`<br/>_map[string]string_|  NodeSelector defines which nodes pods should be scheduling on.  |
|`serviceAccountName`<br/>_string_|  ServiceAccountName is the name of the ServiceAccount to use for running Grafana Agent pods.  |
|`secrets`<br/>_[]string_|  Secrets is a list of secrets in the same namespace as the GrafanaAgent object which will be mounted into each running Grafana Agent pod. The secrets are mounted into /var/lib/grafana-agent/extra-secrets/&lt;secret-name&gt;.  |
|`configMaps`<br/>_[]string_|  ConfigMaps is a list of config maps in the same namespace as the GrafanaAgent object which will be mounted into each running Grafana Agent pod. The ConfigMaps are mounted into /var/lib/grafana-agent/extra-configmaps/&lt;configmap-name&gt;.  |
|`affinity`<br/>_[Kubernetes core/v1.Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#affinity-v1-core)_|  Affinity, if specified, controls pod scheduling constraints.  |
|`tolerations`<br/>_[[]Kubernetes core/v1.Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#toleration-v1-core)_|  Tolerations, if specified, controls the pod&#39;s tolerations.  |
|`topologySpreadConstraints`<br/>_[[]Kubernetes core/v1.TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#topologyspreadconstraint-v1-core)_|  TopologySpreadConstraints, if specified, controls the pod&#39;s topology spread constraints.  |
|`securityContext`<br/>_[Kubernetes core/v1.PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#podsecuritycontext-v1-core)_|  SecurityContext holds pod-level security attributes and common container settings. When unspecified, defaults to the default PodSecurityContext.  |
|`containers`<br/>_[[]Kubernetes core/v1.Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core)_|  Containers lets you inject additional containers or modify operator-generated containers. This can be used to add an authentication proxy to a Grafana Agent pod or to change the behavior of an operator-generated container. Containers described here modify an operator-generated container if they share the same name and if modifications are done via a strategic merge patch. The current container names are: `grafana-agent` and `config-reloader`. Overriding containers is entirely outside the scope of what the Grafana Agent team supports and by doing so, you accept that this behavior may break at any time without notice.  |
|`initContainers`<br/>_[[]Kubernetes core/v1.Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core)_|  InitContainers let you add initContainers to the pod definition. These can be used to, for example, fetch secrets for injection into the Grafana Agent configuration from external sources. Errors during the execution of an initContainer cause the pod to restart. More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Using initContainers for any use case other than secret fetching is entirely outside the scope of what the Grafana Agent maintainers support and by doing so, you accept that this behavior may break at any time without notice.  |
|`priorityClassName`<br/>_string_|  PriorityClassName is the priority class assigned to pods.  |
|`runtimeClassName`<br/>_string_|  RuntimeClassName is the runtime class assigned to pods.  |
|`portName`<br/>_string_|  Port name used for the pods and governing service. This defaults to agent-metrics.  |
|`metrics`<br/>_[MetricsSubsystemSpec](#monitoring.grafana.com/v1alpha1.MetricsSubsystemSpec)_|  Metrics controls the metrics subsystem of the Agent and settings unique to metrics-specific pods that are deployed.  |
|`logs`<br/>_[LogsSubsystemSpec](#monitoring.grafana.com/v1alpha1.LogsSubsystemSpec)_|  Logs controls the logging subsystem of the Agent and settings unique to logging-specific pods that are deployed.  |
|`integrations`<br/>_[IntegrationsSubsystemSpec](#monitoring.grafana.com/v1alpha1.IntegrationsSubsystemSpec)_|  Integrations controls the integration subsystem of the Agent and settings unique to deployed integration-specific pods.  |
|`enableConfigReadAPI`<br/>_bool_|  enableConfigReadAPI enables the read API for viewing the currently running config port 8080 on the agent. &#43;kubebuilder:default=false  |
|`disableReporting`<br/>_bool_|  disableReporting disables reporting of enabled feature flags to Grafana. &#43;kubebuilder:default=false  |
|`disableSupportBundle`<br/>_bool_|  disableSupportBundle disables the generation of support bundles. &#43;kubebuilder:default=false  |
### IntegrationsDeployment <a name="monitoring.grafana.com/v1alpha1.IntegrationsDeployment"></a>
(Appears on:[Deployment](#monitoring.grafana.com/v1alpha1.Deployment))
IntegrationsDeployment is a set of discovered resources relative to an IntegrationsDeployment. 
#### Fields
|Field|Description|
|-|-|
|apiVersion|string<br/>`monitoring.grafana.com/v1alpha1`|
|kind|string<br/>`IntegrationsDeployment`|
|`Instance`<br/>_[Integration](#monitoring.grafana.com/v1alpha1.Integration)_|    |
### LogsDeployment <a name="monitoring.grafana.com/v1alpha1.LogsDeployment"></a>
(Appears on:[Deployment](#monitoring.grafana.com/v1alpha1.Deployment))
LogsDeployment is a set of discovered resources relative to a LogsInstance. 
#### Fields
|Field|Description|
|-|-|
|apiVersion|string<br/>`monitoring.grafana.com/v1alpha1`|
|kind|string<br/>`LogsDeployment`|
|`Instance`<br/>_[LogsInstance](#monitoring.grafana.com/v1alpha1.LogsInstance)_|    |
|`PodLogs`<br/>_[[]PodLogs](#monitoring.grafana.com/v1alpha1.PodLogs)_|    |
### MetricsDeployment <a name="monitoring.grafana.com/v1alpha1.MetricsDeployment"></a>
(Appears on:[Deployment](#monitoring.grafana.com/v1alpha1.Deployment))
MetricsDeployment is a set of discovered resources relative to a MetricsInstance. 
#### Fields
|Field|Description|
|-|-|
|apiVersion|string<br/>`monitoring.grafana.com/v1alpha1`|
|kind|string<br/>`MetricsDeployment`|
|`Instance`<br/>_[MetricsInstance](#monitoring.grafana.com/v1alpha1.MetricsInstance)_|    |
|`ServiceMonitors`<br/>_[[]github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.ServiceMonitor](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.ServiceMonitor)_|    |
|`PodMonitors`<br/>_[[]github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.PodMonitor](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.PodMonitor)_|    |
|`Probes`<br/>_[[]github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.Probe](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.Probe)_|    |
### CRIStageSpec <a name="monitoring.grafana.com/v1alpha1.CRIStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
CRIStageSpec is a parsing stage that reads log lines using the standard CRI logging format. It needs no defined fields. 
### DockerStageSpec <a name="monitoring.grafana.com/v1alpha1.DockerStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
DockerStageSpec is a parsing stage that reads log lines using the standard Docker logging format. It needs no defined fields. 
### DropStageSpec <a name="monitoring.grafana.com/v1alpha1.DropStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
DropStageSpec is a filtering stage that lets you drop certain logs. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from the extract data to parse. If empty, uses the log message.  |
|`expression`<br/>_string_|  RE2 regular expression.  If source is provided, the regex attempts to match the source.  If no source is provided, then the regex attempts to attach the log line.  If the provided regex matches the log line or a provided source, the line is dropped.  |
|`value`<br/>_string_|  Value can only be specified when source is specified. If the value provided is an exact match for the given source then the line will be dropped.  Mutually exclusive with expression.  |
|`olderThan`<br/>_string_|  OlderThan will be parsed as a Go duration. If the log line&#39;s timestamp is older than the current time minus the provided duration, it will be dropped.  |
|`longerThan`<br/>_string_|  LongerThan will drop a log line if it its content is longer than this value (in bytes). Can be expressed as an integer (8192) or a number with a suffix (8kb).  |
|`dropCounterReason`<br/>_string_|  Every time a log line is dropped, the metric logentry_dropped_lines_total is incremented. A &#34;reason&#34; label is added, and can be customized by providing a custom value here. Defaults to &#34;drop_stage&#34;.  |
### GrafanaAgentSpec <a name="monitoring.grafana.com/v1alpha1.GrafanaAgentSpec"></a>
(Appears on:[GrafanaAgent](#monitoring.grafana.com/v1alpha1.GrafanaAgent))
GrafanaAgentSpec is a specification of the desired behavior of the Grafana Agent cluster. 
#### Fields
|Field|Description|
|-|-|
|`logLevel`<br/>_string_|  LogLevel controls the log level of the generated pods. Defaults to &#34;info&#34; if not set.  |
|`logFormat`<br/>_string_|  LogFormat controls the logging format of the generated pods. Defaults to &#34;logfmt&#34; if not set.  |
|`apiServer`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.APIServerConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.APIServerConfig)_|  APIServerConfig lets you specify a host and auth methods to access the Kubernetes API server. If left empty, the Agent assumes that it is running inside of the cluster and will discover API servers automatically and use the pod&#39;s CA certificate and bearer token file at /var/run/secrets/kubernetes.io/serviceaccount.  |
|`podMetadata`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.EmbeddedObjectMetadata](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.EmbeddedObjectMetadata)_|  PodMetadata configures Labels and Annotations which are propagated to created Grafana Agent pods.  |
|`version`<br/>_string_|  Version of Grafana Agent to be deployed.  |
|`paused`<br/>_bool_|  Paused prevents actions except for deletion to be performed on the underlying managed objects.  |
|`image`<br/>_string_|  Image, when specified, overrides the image used to run Agent. Specify the image along with a tag. You still need to set the version to ensure Grafana Agent Operator knows which version of Grafana Agent is being configured.  |
|`configReloaderVersion`<br/>_string_|  Version of Config Reloader to be deployed.  |
|`configReloaderImage`<br/>_string_|  Image, when specified, overrides the image used to run Config Reloader. Specify the image along with a tag. You still need to set the version to ensure Grafana Agent Operator knows which version of Grafana Agent is being configured.  |
|`imagePullSecrets`<br/>_[[]Kubernetes core/v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core)_|  ImagePullSecrets holds an optional list of references to Secrets within the same namespace used for pulling the Grafana Agent image from registries. More info: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod  |
|`storage`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.StorageSpec](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.StorageSpec)_|  Storage spec to specify how storage will be used.  |
|`volumes`<br/>_[[]Kubernetes core/v1.Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core)_|  Volumes allows configuration of additional volumes on the output StatefulSet definition. The volumes specified are appended to other volumes that are generated as a result of StorageSpec objects.  |
|`volumeMounts`<br/>_[[]Kubernetes core/v1.VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core)_|  VolumeMounts lets you configure additional VolumeMounts on the output StatefulSet definition. Specified VolumeMounts are appended to other VolumeMounts generated as a result of StorageSpec objects in the Grafana Agent container.  |
|`resources`<br/>_[Kubernetes core/v1.ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core)_|  Resources holds requests and limits for individual pods.  |
|`nodeSelector`<br/>_map[string]string_|  NodeSelector defines which nodes pods should be scheduling on.  |
|`serviceAccountName`<br/>_string_|  ServiceAccountName is the name of the ServiceAccount to use for running Grafana Agent pods.  |
|`secrets`<br/>_[]string_|  Secrets is a list of secrets in the same namespace as the GrafanaAgent object which will be mounted into each running Grafana Agent pod. The secrets are mounted into /var/lib/grafana-agent/extra-secrets/&lt;secret-name&gt;.  |
|`configMaps`<br/>_[]string_|  ConfigMaps is a list of config maps in the same namespace as the GrafanaAgent object which will be mounted into each running Grafana Agent pod. The ConfigMaps are mounted into /var/lib/grafana-agent/extra-configmaps/&lt;configmap-name&gt;.  |
|`affinity`<br/>_[Kubernetes core/v1.Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#affinity-v1-core)_|  Affinity, if specified, controls pod scheduling constraints.  |
|`tolerations`<br/>_[[]Kubernetes core/v1.Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#toleration-v1-core)_|  Tolerations, if specified, controls the pod&#39;s tolerations.  |
|`topologySpreadConstraints`<br/>_[[]Kubernetes core/v1.TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#topologyspreadconstraint-v1-core)_|  TopologySpreadConstraints, if specified, controls the pod&#39;s topology spread constraints.  |
|`securityContext`<br/>_[Kubernetes core/v1.PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#podsecuritycontext-v1-core)_|  SecurityContext holds pod-level security attributes and common container settings. When unspecified, defaults to the default PodSecurityContext.  |
|`containers`<br/>_[[]Kubernetes core/v1.Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core)_|  Containers lets you inject additional containers or modify operator-generated containers. This can be used to add an authentication proxy to a Grafana Agent pod or to change the behavior of an operator-generated container. Containers described here modify an operator-generated container if they share the same name and if modifications are done via a strategic merge patch. The current container names are: `grafana-agent` and `config-reloader`. Overriding containers is entirely outside the scope of what the Grafana Agent team supports and by doing so, you accept that this behavior may break at any time without notice.  |
|`initContainers`<br/>_[[]Kubernetes core/v1.Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core)_|  InitContainers let you add initContainers to the pod definition. These can be used to, for example, fetch secrets for injection into the Grafana Agent configuration from external sources. Errors during the execution of an initContainer cause the pod to restart. More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Using initContainers for any use case other than secret fetching is entirely outside the scope of what the Grafana Agent maintainers support and by doing so, you accept that this behavior may break at any time without notice.  |
|`priorityClassName`<br/>_string_|  PriorityClassName is the priority class assigned to pods.  |
|`runtimeClassName`<br/>_string_|  RuntimeClassName is the runtime class assigned to pods.  |
|`portName`<br/>_string_|  Port name used for the pods and governing service. This defaults to agent-metrics.  |
|`metrics`<br/>_[MetricsSubsystemSpec](#monitoring.grafana.com/v1alpha1.MetricsSubsystemSpec)_|  Metrics controls the metrics subsystem of the Agent and settings unique to metrics-specific pods that are deployed.  |
|`logs`<br/>_[LogsSubsystemSpec](#monitoring.grafana.com/v1alpha1.LogsSubsystemSpec)_|  Logs controls the logging subsystem of the Agent and settings unique to logging-specific pods that are deployed.  |
|`integrations`<br/>_[IntegrationsSubsystemSpec](#monitoring.grafana.com/v1alpha1.IntegrationsSubsystemSpec)_|  Integrations controls the integration subsystem of the Agent and settings unique to deployed integration-specific pods.  |
|`enableConfigReadAPI`<br/>_bool_|  enableConfigReadAPI enables the read API for viewing the currently running config port 8080 on the agent. &#43;kubebuilder:default=false  |
|`disableReporting`<br/>_bool_|  disableReporting disables reporting of enabled feature flags to Grafana. &#43;kubebuilder:default=false  |
|`disableSupportBundle`<br/>_bool_|  disableSupportBundle disables the generation of support bundles. &#43;kubebuilder:default=false  |
### Integration <a name="monitoring.grafana.com/v1alpha1.Integration"></a>
(Appears on:[IntegrationsDeployment](#monitoring.grafana.com/v1alpha1.IntegrationsDeployment))
Integration runs a single Grafana Agent integration. Integrations that generate telemetry must be configured to send that telemetry somewhere, such as autoscrape for exporter-based integrations.  Integrations have access to the LogsInstances and MetricsInstances in the same GrafanaAgent resource set, referenced by the &lt;namespace&gt;/&lt;name&gt; of the Instance resource.  For example, if there is a default/production MetricsInstance, you can configure a supported integration&#39;s autoscrape block with:  	autoscrape: 	  enable: true 	  metrics_instance: default/production  There is currently no way for telemetry created by an Operator-managed integration to be collected from outside of the integration itself. 
#### Fields
|Field|Description|
|-|-|
|`metadata`<br/>_[Kubernetes meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_|     Refer to the Kubernetes API documentation for the fields of the `metadata` field. |
|`spec`<br/>_[IntegrationSpec](#monitoring.grafana.com/v1alpha1.IntegrationSpec)_|  Specifies the desired behavior of the Integration.  |
|`name`<br/>_string_|  Name of the integration to run (e.g., &#34;node_exporter&#34;, &#34;mysqld_exporter&#34;).  |
|`type`<br/>_[IntegrationType](#monitoring.grafana.com/v1alpha1.IntegrationType)_|  Type informs Grafana Agent Operator about how to manage the integration being configured.  |
|`config`<br/>_[k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON](https://pkg.go.dev/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1#JSON)_|  The configuration for the named integration. Note that Integrations are deployed with the integrations-next feature flag, which has different common settings:    https://grafana.com/docs/agent/latest/configuration/integrations/integrations-next/  |
|`volumes`<br/>_[[]Kubernetes core/v1.Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core)_|  An extra list of Volumes to be associated with the Grafana Agent pods running this integration. Volume names are mutated to be unique across all Integrations. Note that the specified volumes should be able to tolerate existing on multiple pods at once when type is daemonset.  Don&#39;t use volumes for loading Secrets or ConfigMaps from the same namespace as the Integration; use the Secrets and ConfigMaps fields instead.  |
|`volumeMounts`<br/>_[[]Kubernetes core/v1.VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core)_|  An extra list of VolumeMounts to be associated with the Grafana Agent pods running this integration. VolumeMount names are mutated to be unique across all used IntegrationSpecs.  Mount paths should include the namespace/name of the Integration CR to avoid potentially colliding with other resources.  |
|`secrets`<br/>_[[]Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  An extra list of keys from Secrets in the same namespace as the Integration which will be mounted into the Grafana Agent pod running this Integration.  Secrets will be mounted at /etc/grafana-agent/integrations/secrets/&lt;secret_namespace&gt;/&lt;secret_name&gt;/&lt;key&gt;.  |
|`configMaps`<br/>_[[]Kubernetes core/v1.ConfigMapKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#configmapkeyselector-v1-core)_|  An extra list of keys from ConfigMaps in the same namespace as the Integration which will be mounted into the Grafana Agent pod running this Integration.  ConfigMaps are mounted at /etc/grafana-agent/integrations/configMaps/&lt;configmap_namespace&gt;/&lt;configmap_name&gt;/&lt;key&gt;.  |
### IntegrationSpec <a name="monitoring.grafana.com/v1alpha1.IntegrationSpec"></a>
(Appears on:[Integration](#monitoring.grafana.com/v1alpha1.Integration))
IntegrationSpec specifies the desired behavior of a metrics integration. 
#### Fields
|Field|Description|
|-|-|
|`name`<br/>_string_|  Name of the integration to run (e.g., &#34;node_exporter&#34;, &#34;mysqld_exporter&#34;).  |
|`type`<br/>_[IntegrationType](#monitoring.grafana.com/v1alpha1.IntegrationType)_|  Type informs Grafana Agent Operator about how to manage the integration being configured.  |
|`config`<br/>_[k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON](https://pkg.go.dev/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1#JSON)_|  The configuration for the named integration. Note that Integrations are deployed with the integrations-next feature flag, which has different common settings:    https://grafana.com/docs/agent/latest/configuration/integrations/integrations-next/  |
|`volumes`<br/>_[[]Kubernetes core/v1.Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volume-v1-core)_|  An extra list of Volumes to be associated with the Grafana Agent pods running this integration. Volume names are mutated to be unique across all Integrations. Note that the specified volumes should be able to tolerate existing on multiple pods at once when type is daemonset.  Don&#39;t use volumes for loading Secrets or ConfigMaps from the same namespace as the Integration; use the Secrets and ConfigMaps fields instead.  |
|`volumeMounts`<br/>_[[]Kubernetes core/v1.VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#volumemount-v1-core)_|  An extra list of VolumeMounts to be associated with the Grafana Agent pods running this integration. VolumeMount names are mutated to be unique across all used IntegrationSpecs.  Mount paths should include the namespace/name of the Integration CR to avoid potentially colliding with other resources.  |
|`secrets`<br/>_[[]Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  An extra list of keys from Secrets in the same namespace as the Integration which will be mounted into the Grafana Agent pod running this Integration.  Secrets will be mounted at /etc/grafana-agent/integrations/secrets/&lt;secret_namespace&gt;/&lt;secret_name&gt;/&lt;key&gt;.  |
|`configMaps`<br/>_[[]Kubernetes core/v1.ConfigMapKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#configmapkeyselector-v1-core)_|  An extra list of keys from ConfigMaps in the same namespace as the Integration which will be mounted into the Grafana Agent pod running this Integration.  ConfigMaps are mounted at /etc/grafana-agent/integrations/configMaps/&lt;configmap_namespace&gt;/&lt;configmap_name&gt;/&lt;key&gt;.  |
### IntegrationType <a name="monitoring.grafana.com/v1alpha1.IntegrationType"></a>
(Appears on:[IntegrationSpec](#monitoring.grafana.com/v1alpha1.IntegrationSpec))
IntegrationType determines specific behaviors of a configured integration. 
#### Fields
|Field|Description|
|-|-|
|`allNodes`<br/>_bool_|  When true, the configured integration should be run on every Node in the cluster. This is required for Integrations that generate Node-specific metrics like node_exporter, otherwise it must be false to avoid generating duplicate metrics.  |
|`unique`<br/>_bool_|  Whether this integration can only be defined once for a Grafana Agent process, such as statsd_exporter. It is invalid for a GrafanaAgent to discover multiple unique Integrations with the same Integration name (i.e., a single GrafanaAgent cannot deploy two statsd_exporters).  |
### IntegrationsSubsystemSpec <a name="monitoring.grafana.com/v1alpha1.IntegrationsSubsystemSpec"></a>
(Appears on:[GrafanaAgentSpec](#monitoring.grafana.com/v1alpha1.GrafanaAgentSpec))
IntegrationsSubsystemSpec defines global settings to apply across the integrations subsystem. 
#### Fields
|Field|Description|
|-|-|
|`selector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Label selector to find Integration resources to run. When nil, no integration resources will be defined.  |
|`namespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Label selector for namespaces to search when discovering integration resources. If nil, integration resources are only discovered in the namespace of the GrafanaAgent resource.  Set to `{}` to search all namespaces.  |
### JSONStageSpec <a name="monitoring.grafana.com/v1alpha1.JSONStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
JSONStageSpec is a parsing stage that reads the log line as JSON and accepts JMESPath expressions to extract data. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from the extracted data to parse as JSON. If empty, uses entire log message.  |
|`expressions`<br/>_map[string]string_|  Set of the key/value pairs of JMESPath expressions. The key will be the key in the extracted data while the expression will be the value, evaluated as a JMESPath from the source data.  Literal JMESPath expressions can be used by wrapping a key in double quotes, which then must be wrapped again in single quotes in YAML so they get passed to the JMESPath parser.  |
### LimitStageSpec <a name="monitoring.grafana.com/v1alpha1.LimitStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
The limit stage is a rate-limiting stage that throttles logs based on several options. 
#### Fields
|Field|Description|
|-|-|
|`rate`<br/>_int_|  The rate limit in lines per second that Promtail will push to Loki.  |
|`burst`<br/>_int_|  The cap in the quantity of burst lines that Promtail will push to Loki.  |
|`drop`<br/>_bool_|  When drop is true, log lines that exceed the current rate limit are discarded. When drop is false, log lines that exceed the current rate limit wait to enter the back pressure mode.  Defaults to false.  |
### LogsBackoffConfigSpec <a name="monitoring.grafana.com/v1alpha1.LogsBackoffConfigSpec"></a>
(Appears on:[LogsClientSpec](#monitoring.grafana.com/v1alpha1.LogsClientSpec))
LogsBackoffConfigSpec configures timing for retrying failed requests. 
#### Fields
|Field|Description|
|-|-|
|`minPeriod`<br/>_string_|  Initial backoff time between retries. Time between retries is increased exponentially.  |
|`maxPeriod`<br/>_string_|  Maximum backoff time between retries.  |
|`maxRetries`<br/>_int_|  Maximum number of retries to perform before giving up a request.  |
### LogsClientSpec <a name="monitoring.grafana.com/v1alpha1.LogsClientSpec"></a>
(Appears on:[LogsInstanceSpec](#monitoring.grafana.com/v1alpha1.LogsInstanceSpec), [LogsSubsystemSpec](#monitoring.grafana.com/v1alpha1.LogsSubsystemSpec))
LogsClientSpec defines the client integration for logs, indicating which Loki server to send logs to. 
#### Fields
|Field|Description|
|-|-|
|`url`<br/>_string_|  URL is the URL where Loki is listening. Must be a full HTTP URL, including protocol. Required. Example: https://logs-prod-us-central1.grafana.net/loki/api/v1/push.  |
|`tenantId`<br/>_string_|  Tenant ID used by default to push logs to Loki. If omitted assumes remote Loki is running in single-tenant mode or an authentication layer is used to inject an X-Scope-OrgID header.  |
|`batchWait`<br/>_string_|  Maximum amount of time to wait before sending a batch, even if that batch isn&#39;t full.  |
|`batchSize`<br/>_int_|  Maximum batch size (in bytes) of logs to accumulate before sending the batch to Loki.  |
|`basicAuth`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.BasicAuth](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.BasicAuth)_|  BasicAuth for the Loki server.  |
|`oauth2`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.OAuth2](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.OAuth2)_|  Oauth2 for URL  |
|`bearerToken`<br/>_string_|  BearerToken used for remote_write.  |
|`bearerTokenFile`<br/>_string_|  BearerTokenFile used to read bearer token.  |
|`proxyUrl`<br/>_string_|  ProxyURL to proxy requests through. Optional.  |
|`tlsConfig`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.TLSConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.TLSConfig)_|  TLSConfig to use for the client. Only used when the protocol of the URL is https.  |
|`backoffConfig`<br/>_[LogsBackoffConfigSpec](#monitoring.grafana.com/v1alpha1.LogsBackoffConfigSpec)_|  Configures how to retry requests to Loki when a request fails. Defaults to a minPeriod of 500ms, maxPeriod of 5m, and maxRetries of 10.  |
|`externalLabels`<br/>_map[string]string_|  ExternalLabels are labels to add to any time series when sending data to Loki.  |
|`timeout`<br/>_string_|  Maximum time to wait for a server to respond to a request.  |
### LogsInstance <a name="monitoring.grafana.com/v1alpha1.LogsInstance"></a>
(Appears on:[LogsDeployment](#monitoring.grafana.com/v1alpha1.LogsDeployment))
LogsInstance controls an individual logs instance within a Grafana Agent deployment. 
#### Fields
|Field|Description|
|-|-|
|`metadata`<br/>_[Kubernetes meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_|     Refer to the Kubernetes API documentation for the fields of the `metadata` field. |
|`spec`<br/>_[LogsInstanceSpec](#monitoring.grafana.com/v1alpha1.LogsInstanceSpec)_|  Spec holds the specification of the desired behavior for the logs instance.  |
|`clients`<br/>_[[]LogsClientSpec](#monitoring.grafana.com/v1alpha1.LogsClientSpec)_|  Clients controls where logs are written to for this instance.  |
|`podLogsSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Determines which PodLogs should be selected for including in this instance.  |
|`podLogsNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Set of labels to determine which namespaces should be watched for PodLogs. If not provided, checks only namespace of the instance.  |
|`additionalScrapeConfigs`<br/>_[Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  AdditionalScrapeConfigs allows specifying a key of a Secret containing additional Grafana Agent logging scrape configurations. Scrape configurations specified are appended to the configurations generated by the Grafana Agent Operator.  Job configurations specified must have the form as specified in the official Promtail documentation:  https://grafana.com/docs/loki/latest/clients/promtail/configuration/#scrape_configs  As scrape configs are appended, the user is responsible to make sure it is valid. Note that using this feature may expose the possibility to break upgrades of Grafana Agent. It is advised to review both Grafana Agent and Promtail release notes to ensure that no incompatible scrape configs are going to break Grafana Agent after the upgrade.  |
|`targetConfig`<br/>_[LogsTargetConfigSpec](#monitoring.grafana.com/v1alpha1.LogsTargetConfigSpec)_|  Configures how tailed targets are watched.  |
### LogsInstanceSpec <a name="monitoring.grafana.com/v1alpha1.LogsInstanceSpec"></a>
(Appears on:[LogsInstance](#monitoring.grafana.com/v1alpha1.LogsInstance))
LogsInstanceSpec controls how an individual instance will be used to discover LogMonitors. 
#### Fields
|Field|Description|
|-|-|
|`clients`<br/>_[[]LogsClientSpec](#monitoring.grafana.com/v1alpha1.LogsClientSpec)_|  Clients controls where logs are written to for this instance.  |
|`podLogsSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Determines which PodLogs should be selected for including in this instance.  |
|`podLogsNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Set of labels to determine which namespaces should be watched for PodLogs. If not provided, checks only namespace of the instance.  |
|`additionalScrapeConfigs`<br/>_[Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  AdditionalScrapeConfigs allows specifying a key of a Secret containing additional Grafana Agent logging scrape configurations. Scrape configurations specified are appended to the configurations generated by the Grafana Agent Operator.  Job configurations specified must have the form as specified in the official Promtail documentation:  https://grafana.com/docs/loki/latest/clients/promtail/configuration/#scrape_configs  As scrape configs are appended, the user is responsible to make sure it is valid. Note that using this feature may expose the possibility to break upgrades of Grafana Agent. It is advised to review both Grafana Agent and Promtail release notes to ensure that no incompatible scrape configs are going to break Grafana Agent after the upgrade.  |
|`targetConfig`<br/>_[LogsTargetConfigSpec](#monitoring.grafana.com/v1alpha1.LogsTargetConfigSpec)_|  Configures how tailed targets are watched.  |
### LogsSubsystemSpec <a name="monitoring.grafana.com/v1alpha1.LogsSubsystemSpec"></a>
(Appears on:[GrafanaAgentSpec](#monitoring.grafana.com/v1alpha1.GrafanaAgentSpec))
LogsSubsystemSpec defines global settings to apply across the logging subsystem. 
#### Fields
|Field|Description|
|-|-|
|`clients`<br/>_[[]LogsClientSpec](#monitoring.grafana.com/v1alpha1.LogsClientSpec)_|  A global set of clients to use when a discovered LogsInstance does not have any clients defined.  |
|`logsExternalLabelName`<br/>_string_|  LogsExternalLabelName is the name of the external label used to denote Grafana Agent cluster. Defaults to &#34;cluster.&#34; External label will _not_ be added when value is set to the empty string.  |
|`instanceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  InstanceSelector determines which LogInstances should be selected for running. Each instance runs its own set of Prometheus components, including service discovery, scraping, and remote_write.  |
|`instanceNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  InstanceNamespaceSelector are the set of labels to determine which namespaces to watch for LogInstances. If not provided, only checks own namespace.  |
|`ignoreNamespaceSelectors`<br/>_bool_|  IgnoreNamespaceSelectors, if true, will ignore NamespaceSelector settings from the PodLogs configs, and they will only discover endpoints within their current namespace.  |
|`enforcedNamespaceLabel`<br/>_string_|  EnforcedNamespaceLabel enforces adding a namespace label of origin for each metric that is user-created. The label value will always be the namespace of the object that is being created.  |
### LogsTargetConfigSpec <a name="monitoring.grafana.com/v1alpha1.LogsTargetConfigSpec"></a>
(Appears on:[LogsInstanceSpec](#monitoring.grafana.com/v1alpha1.LogsInstanceSpec))
LogsTargetConfigSpec configures how tailed targets are watched. 
#### Fields
|Field|Description|
|-|-|
|`syncPeriod`<br/>_string_|  Period to resync directories being watched and files being tailed to discover new ones or stop watching removed ones.  |
### MatchStageSpec <a name="monitoring.grafana.com/v1alpha1.MatchStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
MatchStageSpec is a filtering stage that conditionally applies a set of stages or drop entries when a log entry matches a configurable LogQL stream selector and filter expressions. 
#### Fields
|Field|Description|
|-|-|
|`selector`<br/>_string_|  LogQL stream selector and filter expressions. Required.  |
|`pipelineName`<br/>_string_|  Names the pipeline. When defined, creates an additional label in the pipeline_duration_seconds histogram, where the value is concatenated with job_name using an underscore.  |
|`action`<br/>_string_|  Determines what action is taken when the selector matches the log line. Can be keep or drop. Defaults to keep. When set to drop, entries are dropped and no later metrics are recorded. Stages must be empty when dropping metrics.  |
|`dropCounterReason`<br/>_string_|  Every time a log line is dropped, the metric logentry_dropped_lines_total is incremented. A &#34;reason&#34; label is added, and can be customized by providing a custom value here. Defaults to &#34;match_stage.&#34;  |
|`stages`<br/>_string_|  Nested set of pipeline stages to execute when action is keep and the log line matches selector.  An example value for stages may be:    stages: |     - json: {}     - labelAllow: [foo, bar]  Note that stages is a string because SIG API Machinery does not support recursive types, and so it cannot be validated for correctness. Be careful not to mistype anything.  |
### MetadataConfig <a name="monitoring.grafana.com/v1alpha1.MetadataConfig"></a>
(Appears on:[RemoteWriteSpec](#monitoring.grafana.com/v1alpha1.RemoteWriteSpec))
MetadataConfig configures the sending of series metadata to remote storage. 
#### Fields
|Field|Description|
|-|-|
|`send`<br/>_bool_|  Send enables metric metadata to be sent to remote storage.  |
|`sendInterval`<br/>_string_|  SendInterval controls how frequently metric metadata is sent to remote storage.  |
### MetricsInstance <a name="monitoring.grafana.com/v1alpha1.MetricsInstance"></a>
(Appears on:[MetricsDeployment](#monitoring.grafana.com/v1alpha1.MetricsDeployment))
MetricsInstance controls an individual Metrics instance within a Grafana Agent deployment. 
#### Fields
|Field|Description|
|-|-|
|`metadata`<br/>_[Kubernetes meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_|     Refer to the Kubernetes API documentation for the fields of the `metadata` field. |
|`spec`<br/>_[MetricsInstanceSpec](#monitoring.grafana.com/v1alpha1.MetricsInstanceSpec)_|  Spec holds the specification of the desired behavior for the Metrics instance.  |
|`walTruncateFrequency`<br/>_string_|  WALTruncateFrequency specifies how frequently to run the WAL truncation process. Higher values cause the WAL to increase and for old series to stay in the WAL longer, but reduces the chance of data loss when remote_write fails for longer than the given frequency.  |
|`minWALTime`<br/>_string_|  MinWALTime is the minimum amount of time that series and samples can exist in the WAL before being considered for deletion.  |
|`maxWALTime`<br/>_string_|  MaxWALTime is the maximum amount of time that series and samples can exist in the WAL before being forcibly deleted.  |
|`remoteFlushDeadline`<br/>_string_|  RemoteFlushDeadline is the deadline for flushing data when an instance shuts down.  |
|`writeStaleOnShutdown`<br/>_bool_|  WriteStaleOnShutdown writes staleness markers on shutdown for all series.  |
|`serviceMonitorSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ServiceMonitorSelector determines which ServiceMonitors to select for target discovery.  |
|`serviceMonitorNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ServiceMonitorNamespaceSelector is the set of labels that determine which namespaces to watch for ServiceMonitor discovery. If nil, it only checks its own namespace.  |
|`podMonitorSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  PodMonitorSelector determines which PodMonitors to selected for target discovery. Experimental.  |
|`podMonitorNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  PodMonitorNamespaceSelector are the set of labels to determine which namespaces to watch for PodMonitor discovery. If nil, it only checks its own namespace.  |
|`probeSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ProbeSelector determines which Probes to select for target discovery.  |
|`probeNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ProbeNamespaceSelector is the set of labels that determines which namespaces to watch for Probe discovery. If nil, it only checks own namespace.  |
|`remoteWrite`<br/>_[[]RemoteWriteSpec](#monitoring.grafana.com/v1alpha1.RemoteWriteSpec)_|  RemoteWrite controls remote_write settings for this instance.  |
|`additionalScrapeConfigs`<br/>_[Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  AdditionalScrapeConfigs lets you specify a key of a Secret containing additional Grafana Agent Prometheus scrape configurations. The specified scrape configurations are appended to the configurations generated by Grafana Agent Operator. Specified job configurations must have the form specified in the official Prometheus documentation: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config. As scrape configs are appended, you must make sure the configuration is still valid. Note that it&#39;s possible that this feature will break future upgrades of Grafana Agent. Review both Grafana Agent and Prometheus release notes to ensure that no incompatible scrape configs will break Grafana Agent after the upgrade.  |
### MetricsInstanceSpec <a name="monitoring.grafana.com/v1alpha1.MetricsInstanceSpec"></a>
(Appears on:[MetricsInstance](#monitoring.grafana.com/v1alpha1.MetricsInstance))
MetricsInstanceSpec controls how an individual instance is used to discover PodMonitors. 
#### Fields
|Field|Description|
|-|-|
|`walTruncateFrequency`<br/>_string_|  WALTruncateFrequency specifies how frequently to run the WAL truncation process. Higher values cause the WAL to increase and for old series to stay in the WAL longer, but reduces the chance of data loss when remote_write fails for longer than the given frequency.  |
|`minWALTime`<br/>_string_|  MinWALTime is the minimum amount of time that series and samples can exist in the WAL before being considered for deletion.  |
|`maxWALTime`<br/>_string_|  MaxWALTime is the maximum amount of time that series and samples can exist in the WAL before being forcibly deleted.  |
|`remoteFlushDeadline`<br/>_string_|  RemoteFlushDeadline is the deadline for flushing data when an instance shuts down.  |
|`writeStaleOnShutdown`<br/>_bool_|  WriteStaleOnShutdown writes staleness markers on shutdown for all series.  |
|`serviceMonitorSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ServiceMonitorSelector determines which ServiceMonitors to select for target discovery.  |
|`serviceMonitorNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ServiceMonitorNamespaceSelector is the set of labels that determine which namespaces to watch for ServiceMonitor discovery. If nil, it only checks its own namespace.  |
|`podMonitorSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  PodMonitorSelector determines which PodMonitors to selected for target discovery. Experimental.  |
|`podMonitorNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  PodMonitorNamespaceSelector are the set of labels to determine which namespaces to watch for PodMonitor discovery. If nil, it only checks its own namespace.  |
|`probeSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ProbeSelector determines which Probes to select for target discovery.  |
|`probeNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  ProbeNamespaceSelector is the set of labels that determines which namespaces to watch for Probe discovery. If nil, it only checks own namespace.  |
|`remoteWrite`<br/>_[[]RemoteWriteSpec](#monitoring.grafana.com/v1alpha1.RemoteWriteSpec)_|  RemoteWrite controls remote_write settings for this instance.  |
|`additionalScrapeConfigs`<br/>_[Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  AdditionalScrapeConfigs lets you specify a key of a Secret containing additional Grafana Agent Prometheus scrape configurations. The specified scrape configurations are appended to the configurations generated by Grafana Agent Operator. Specified job configurations must have the form specified in the official Prometheus documentation: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config. As scrape configs are appended, you must make sure the configuration is still valid. Note that it&#39;s possible that this feature will break future upgrades of Grafana Agent. Review both Grafana Agent and Prometheus release notes to ensure that no incompatible scrape configs will break Grafana Agent after the upgrade.  |
### MetricsStageSpec <a name="monitoring.grafana.com/v1alpha1.MetricsStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
MetricsStageSpec is an action stage that allows for defining and updating metrics based on data from the extracted map. Created metrics are not pushed to Loki or Prometheus and are instead exposed via the /metrics endpoint of the Grafana Agent pod. The Grafana Agent Operator should be configured with a MetricsInstance that discovers the logging DaemonSet to collect metrics created by this stage. 
#### Fields
|Field|Description|
|-|-|
|`type`<br/>_string_|  The metric type to create. Must be one of counter, gauge, histogram. Required.  |
|`description`<br/>_string_|  Sets the description for the created metric.  |
|`prefix`<br/>_string_|  Sets the custom prefix name for the metric. Defaults to &#34;promtail_custom_&#34;.  |
|`source`<br/>_string_|  Key from the extracted data map to use for the metric. Defaults to the metrics name if not present.  |
|`maxIdleDuration`<br/>_string_|  Label values on metrics are dynamic which can cause exported metrics to go stale. To prevent unbounded cardinality, any metrics not updated within MaxIdleDuration are removed.  Must be greater or equal to 1s. Defaults to 5m.  |
|`matchAll`<br/>_bool_|  If true, all log lines are counted without attempting to match the source to the extracted map. Mutually exclusive with value.  Only valid for type: counter.  |
|`countEntryBytes`<br/>_bool_|  If true all log line bytes are counted. Can only be set with matchAll: true and action: add.  Only valid for type: counter.  |
|`value`<br/>_string_|  Filters down source data and only changes the metric if the targeted value matches the provided string exactly. If not present, all data matches.  |
|`action`<br/>_string_|  The action to take against the metric. Required.  Must be either &#34;inc&#34; or &#34;add&#34; for type: counter or type: histogram. When type: gauge, must be one of &#34;set&#34;, &#34;inc&#34;, &#34;dec&#34;, &#34;add&#34;, or &#34;sub&#34;.  &#34;add&#34;, &#34;set&#34;, or &#34;sub&#34; requires the extracted value to be convertible to a positive float.  |
|`buckets`<br/>_[]string_|  Buckets to create. Bucket values must be convertible to float64s. Extremely large or small numbers are subject to some loss of precision. Only valid for type: histogram.  |
### MetricsSubsystemSpec <a name="monitoring.grafana.com/v1alpha1.MetricsSubsystemSpec"></a>
(Appears on:[GrafanaAgentSpec](#monitoring.grafana.com/v1alpha1.GrafanaAgentSpec))
MetricsSubsystemSpec defines global settings to apply across the Metrics subsystem. 
#### Fields
|Field|Description|
|-|-|
|`remoteWrite`<br/>_[[]RemoteWriteSpec](#monitoring.grafana.com/v1alpha1.RemoteWriteSpec)_|  RemoteWrite controls default remote_write settings for all instances. If an instance does not provide its own RemoteWrite settings, these will be used instead.  |
|`replicas`<br/>_int32_|  Replicas of each shard to deploy for metrics pods. Number of replicas multiplied by the number of shards is the total number of pods created.  |
|`shards`<br/>_int32_|  Shards to distribute targets onto. Number of replicas multiplied by the number of shards is the total number of pods created. Note that scaling down shards does not reshard data onto remaining instances; it must be manually moved. Increasing shards does not reshard data either, but it will continue to be available from the same instances. Sharding is performed on the content of the __address__ target meta-label.  |
|`replicaExternalLabelName`<br/>_string_|  ReplicaExternalLabelName is the name of the metrics external label used to denote the replica name. Defaults to __replica__. The external label is _not_ added when the value is set to the empty string.  |
|`metricsExternalLabelName`<br/>_string_|  MetricsExternalLabelName is the name of the external label used to denote Grafana Agent cluster. Defaults to &#34;cluster.&#34; The external label is _not_ added when the value is set to the empty string.  |
|`scrapeInterval`<br/>_string_|  ScrapeInterval is the time between consecutive scrapes.  |
|`scrapeTimeout`<br/>_string_|  ScrapeTimeout is the time to wait for a target to respond before marking a scrape as failed.  |
|`externalLabels`<br/>_map[string]string_|  ExternalLabels are labels to add to any time series when sending data over remote_write.  |
|`arbitraryFSAccessThroughSMs`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.ArbitraryFSAccessThroughSMsConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.ArbitraryFSAccessThroughSMsConfig)_|  ArbitraryFSAccessThroughSMs configures whether configuration based on a ServiceMonitor can access arbitrary files on the file system of the Grafana Agent container, e.g., bearer token files.  |
|`overrideHonorLabels`<br/>_bool_|  OverrideHonorLabels, if true, overrides all configured honor_labels read from ServiceMonitor or PodMonitor and sets them to false.  |
|`overrideHonorTimestamps`<br/>_bool_|  OverrideHonorTimestamps allows global enforcement for honoring timestamps in all scrape configs.  |
|`ignoreNamespaceSelectors`<br/>_bool_|  IgnoreNamespaceSelectors, if true, ignores NamespaceSelector settings from the PodMonitor and ServiceMonitor configs, so that they only discover endpoints within their current namespace.  |
|`enforcedNamespaceLabel`<br/>_string_|  EnforcedNamespaceLabel enforces adding a namespace label of origin for each metric that is user-created. The label value is always the namespace of the object that is being created.  |
|`enforcedSampleLimit`<br/>_uint64_|  EnforcedSampleLimit defines a global limit on the number of scraped samples that are accepted. This overrides any SampleLimit set per ServiceMonitor and/or PodMonitor. It is meant to be used by admins to enforce the SampleLimit to keep the overall number of samples and series under the desired limit. Note that if a SampleLimit from a ServiceMonitor or PodMonitor is lower, that value is used instead.  |
|`enforcedTargetLimit`<br/>_uint64_|  EnforcedTargetLimit defines a global limit on the number of scraped targets. This overrides any TargetLimit set per ServiceMonitor and/or PodMonitor. It is meant to be used by admins to enforce the TargetLimit to keep the overall number of targets under the desired limit. Note that if a TargetLimit from a ServiceMonitor or PodMonitor is higher, that value is used instead.  |
|`instanceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  InstanceSelector determines which MetricsInstances should be selected for running. Each instance runs its own set of Metrics components, including service discovery, scraping, and remote_write.  |
|`instanceNamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  InstanceNamespaceSelector is the set of labels that determines which namespaces to watch for MetricsInstances. If not provided, it only checks its own namespace.  |
### MultilineStageSpec <a name="monitoring.grafana.com/v1alpha1.MultilineStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
MultilineStageSpec merges multiple lines into a multiline block before passing it on to the next stage in the pipeline. 
#### Fields
|Field|Description|
|-|-|
|`firstLine`<br/>_string_|  RE2 regular expression. Creates a new multiline block when matched. Required.  |
|`maxWaitTime`<br/>_string_|  Maximum time to wait before passing on the multiline block to the next stage if no new lines are received. Defaults to 3s.  |
|`maxLines`<br/>_int_|  Maximum number of lines a block can have. A new block is started if the number of lines surpasses this value. Defaults to 128.  |
### ObjectSelector <a name="monitoring.grafana.com/v1alpha1.ObjectSelector"></a>
ObjectSelector is a set of selectors to use for finding an object in the resource hierarchy. When NamespaceSelector is nil, search for objects directly in the ParentNamespace. 
#### Fields
|Field|Description|
|-|-|
|`ObjectType`<br/>_[sigs.k8s.io/controller-runtime/pkg/client.Object](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#Object)_|    |
|`ParentNamespace`<br/>_string_|    |
|`NamespaceSelector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|    |
|`Labels`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|    |
### OutputStageSpec <a name="monitoring.grafana.com/v1alpha1.OutputStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
OutputStageSpec is an action stage that takes data from the extracted map and changes the log line that will be sent to Loki. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from extract data to use for the log entry. Required.  |
### PackStageSpec <a name="monitoring.grafana.com/v1alpha1.PackStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
PackStageSpec is a transform stage that lets you embed extracted values and labels into the log line by packing the log line and labels inside of a JSON object. 
#### Fields
|Field|Description|
|-|-|
|`labels`<br/>_[]string_|  Name from extracted data or line labels. Required. Labels provided here are automatically removed from output labels.  |
|`ingestTimestamp`<br/>_bool_|  If the resulting log line should use any existing timestamp or use time.Now() when the line was created. Set to true when combining several log streams from different containers to avoid out of order errors.  |
### PipelineStageSpec <a name="monitoring.grafana.com/v1alpha1.PipelineStageSpec"></a>
(Appears on:[PodLogsSpec](#monitoring.grafana.com/v1alpha1.PodLogsSpec))
PipelineStageSpec defines an individual pipeline stage. Each stage type is mutually exclusive and no more than one may be set per stage.  More information on pipelines can be found in the Promtail documentation: https://grafana.com/docs/loki/latest/clients/promtail/pipelines/ 
#### Fields
|Field|Description|
|-|-|
|`cri`<br/>_[CRIStageSpec](#monitoring.grafana.com/v1alpha1.CRIStageSpec)_|  CRI is a parsing stage that reads log lines using the standard CRI logging format. Supply cri: {} to enable.  |
|`docker`<br/>_[DockerStageSpec](#monitoring.grafana.com/v1alpha1.DockerStageSpec)_|  Docker is a parsing stage that reads log lines using the standard Docker logging format. Supply docker: {} to enable.  |
|`drop`<br/>_[DropStageSpec](#monitoring.grafana.com/v1alpha1.DropStageSpec)_|  Drop is a filtering stage that lets you drop certain logs.  |
|`json`<br/>_[JSONStageSpec](#monitoring.grafana.com/v1alpha1.JSONStageSpec)_|  JSON is a parsing stage that reads the log line as JSON and accepts JMESPath expressions to extract data.  Information on JMESPath: http://jmespath.org/  |
|`labelAllow`<br/>_[]string_|  LabelAllow is an action stage that only allows the provided labels to be included in the label set that is sent to Loki with the log entry.  |
|`labelDrop`<br/>_[]string_|  LabelDrop is an action stage that drops labels from the label set that is sent to Loki with the log entry.  |
|`labels`<br/>_map[string]string_|  Labels is an action stage that takes data from the extracted map and modifies the label set that is sent to Loki with the log entry.  The key is REQUIRED and represents the name for the label that will be created. Value is optional and will be the name from extracted data to use for the value of the label. If the value is not provided, it defaults to match the key.  |
|`limit`<br/>_[LimitStageSpec](#monitoring.grafana.com/v1alpha1.LimitStageSpec)_|  Limit is a rate-limiting stage that throttles logs based on several options.  |
|`match`<br/>_[MatchStageSpec](#monitoring.grafana.com/v1alpha1.MatchStageSpec)_|  Match is a filtering stage that conditionally applies a set of stages or drop entries when a log entry matches a configurable LogQL stream selector and filter expressions.  |
|`metrics`<br/>_[map[string]github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1.MetricsStageSpec](#monitoring.grafana.com/v1alpha1.MetricsStageSpec)_|  Metrics is an action stage that supports defining and updating metrics based on data from the extracted map. Created metrics are not pushed to Loki or Prometheus and are instead exposed via the /metrics endpoint of the Grafana Agent pod. The Grafana Agent Operator should be configured with a MetricsInstance that discovers the logging DaemonSet to collect metrics created by this stage.  |
|`multiline`<br/>_[MultilineStageSpec](#monitoring.grafana.com/v1alpha1.MultilineStageSpec)_|  Multiline stage merges multiple lines into a multiline block before passing it on to the next stage in the pipeline.  |
|`output`<br/>_[OutputStageSpec](#monitoring.grafana.com/v1alpha1.OutputStageSpec)_|  Output stage is an action stage that takes data from the extracted map and changes the log line that will be sent to Loki.  |
|`pack`<br/>_[PackStageSpec](#monitoring.grafana.com/v1alpha1.PackStageSpec)_|  Pack is a transform stage that lets you embed extracted values and labels into the log line by packing the log line and labels inside of a JSON object.  |
|`regex`<br/>_[RegexStageSpec](#monitoring.grafana.com/v1alpha1.RegexStageSpec)_|  Regex is a parsing stage that parses a log line using a regular expression.  Named capture groups in the regex allows for adding data into the extracted map.  |
|`replace`<br/>_[ReplaceStageSpec](#monitoring.grafana.com/v1alpha1.ReplaceStageSpec)_|  Replace is a parsing stage that parses a log line using a regular expression and replaces the log line. Named capture groups in the regex allows for adding data into the extracted map.  |
|`template`<br/>_[TemplateStageSpec](#monitoring.grafana.com/v1alpha1.TemplateStageSpec)_|  Template is a transform stage that manipulates the values in the extracted map using Go&#39;s template syntax.  |
|`tenant`<br/>_[TenantStageSpec](#monitoring.grafana.com/v1alpha1.TenantStageSpec)_|  Tenant is an action stage that sets the tenant ID for the log entry picking it from a field in the extracted data map. If the field is missing, the default LogsClientSpec.tenantId will be used.  |
|`timestamp`<br/>_[TimestampStageSpec](#monitoring.grafana.com/v1alpha1.TimestampStageSpec)_|  Timestamp is an action stage that can change the timestamp of a log line before it is sent to Loki. If not present, the timestamp of a log line defaults to the time when the log line was read.  |
### PodLogs <a name="monitoring.grafana.com/v1alpha1.PodLogs"></a>
(Appears on:[LogsDeployment](#monitoring.grafana.com/v1alpha1.LogsDeployment))
PodLogs defines how to collect logs for a pod. 
#### Fields
|Field|Description|
|-|-|
|`metadata`<br/>_[Kubernetes meta/v1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta)_|     Refer to the Kubernetes API documentation for the fields of the `metadata` field. |
|`spec`<br/>_[PodLogsSpec](#monitoring.grafana.com/v1alpha1.PodLogsSpec)_|  Spec holds the specification of the desired behavior for the PodLogs.  |
|`jobLabel`<br/>_string_|  The label to use to retrieve the job name from.  |
|`podTargetLabels`<br/>_[]string_|  PodTargetLabels transfers labels on the Kubernetes Pod onto the target.  |
|`selector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Selector to select Pod objects. Required.  |
|`namespaceSelector`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.NamespaceSelector](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.NamespaceSelector)_|  Selector to select which namespaces the Pod objects are discovered from.  |
|`pipelineStages`<br/>_[[]PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec)_|  Pipeline stages for this pod. Pipeline stages support transforming and filtering log lines.  |
|`relabelings`<br/>_[[]github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.RelabelConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.RelabelConfig)_|  RelabelConfigs to apply to logs before delivering. Grafana Agent Operator automatically adds relabelings for a few standard Kubernetes fields and replaces original scrape job name with __tmp_logs_job_name.  More info: https://grafana.com/docs/loki/latest/clients/promtail/configuration/#relabel_configs  |
### PodLogsSpec <a name="monitoring.grafana.com/v1alpha1.PodLogsSpec"></a>
(Appears on:[PodLogs](#monitoring.grafana.com/v1alpha1.PodLogs))
PodLogsSpec defines how to collect logs for a pod. 
#### Fields
|Field|Description|
|-|-|
|`jobLabel`<br/>_string_|  The label to use to retrieve the job name from.  |
|`podTargetLabels`<br/>_[]string_|  PodTargetLabels transfers labels on the Kubernetes Pod onto the target.  |
|`selector`<br/>_[Kubernetes meta/v1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#labelselector-v1-meta)_|  Selector to select Pod objects. Required.  |
|`namespaceSelector`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.NamespaceSelector](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.NamespaceSelector)_|  Selector to select which namespaces the Pod objects are discovered from.  |
|`pipelineStages`<br/>_[[]PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec)_|  Pipeline stages for this pod. Pipeline stages support transforming and filtering log lines.  |
|`relabelings`<br/>_[[]github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.RelabelConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.RelabelConfig)_|  RelabelConfigs to apply to logs before delivering. Grafana Agent Operator automatically adds relabelings for a few standard Kubernetes fields and replaces original scrape job name with __tmp_logs_job_name.  More info: https://grafana.com/docs/loki/latest/clients/promtail/configuration/#relabel_configs  |
### QueueConfig <a name="monitoring.grafana.com/v1alpha1.QueueConfig"></a>
(Appears on:[RemoteWriteSpec](#monitoring.grafana.com/v1alpha1.RemoteWriteSpec))
QueueConfig allows the tuning of remote_write queue_config parameters. 
#### Fields
|Field|Description|
|-|-|
|`capacity`<br/>_int_|  Capacity is the number of samples to buffer per shard before samples start being dropped.  |
|`minShards`<br/>_int_|  MinShards is the minimum number of shards, i.e., the amount of concurrency.  |
|`maxShards`<br/>_int_|  MaxShards is the maximum number of shards, i.e., the amount of concurrency.  |
|`maxSamplesPerSend`<br/>_int_|  MaxSamplesPerSend is the maximum number of samples per send.  |
|`batchSendDeadline`<br/>_string_|  BatchSendDeadline is the maximum time a sample will wait in the buffer.  |
|`maxRetries`<br/>_int_|  MaxRetries is the maximum number of times to retry a batch on recoverable errors.  |
|`minBackoff`<br/>_string_|  MinBackoff is the initial retry delay. MinBackoff is doubled for every retry.  |
|`maxBackoff`<br/>_string_|  MaxBackoff is the maximum retry delay.  |
|`retryOnRateLimit`<br/>_bool_|  RetryOnRateLimit retries requests when encountering rate limits.  |
### RegexStageSpec <a name="monitoring.grafana.com/v1alpha1.RegexStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
RegexStageSpec is a parsing stage that parses a log line using a regular expression. Named capture groups in the regex allows for adding data into the extracted map. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from extracted data to parse. If empty, defaults to using the log message.  |
|`expression`<br/>_string_|  RE2 regular expression. Each capture group MUST be named. Required.  |
### RemoteWriteSpec <a name="monitoring.grafana.com/v1alpha1.RemoteWriteSpec"></a>
(Appears on:[MetricsInstanceSpec](#monitoring.grafana.com/v1alpha1.MetricsInstanceSpec), [MetricsSubsystemSpec](#monitoring.grafana.com/v1alpha1.MetricsSubsystemSpec))
RemoteWriteSpec defines the remote_write configuration for Prometheus. 
#### Fields
|Field|Description|
|-|-|
|`name`<br/>_string_|  Name of the remote_write queue. Must be unique if specified. The name is used in metrics and logging in order to differentiate queues.  |
|`url`<br/>_string_|  URL of the endpoint to send samples to.  |
|`remoteTimeout`<br/>_string_|  RemoteTimeout is the timeout for requests to the remote_write endpoint.  |
|`headers`<br/>_map[string]string_|  Headers is a set of custom HTTP headers to be sent along with each remote_write request. Be aware that any headers set by Grafana Agent itself can&#39;t be overwritten.  |
|`writeRelabelConfigs`<br/>_[[]github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.RelabelConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.RelabelConfig)_|  WriteRelabelConfigs holds relabel_configs to relabel samples before they are sent to the remote_write endpoint.  |
|`basicAuth`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.BasicAuth](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.BasicAuth)_|  BasicAuth for the URL.  |
|`oauth2`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.OAuth2](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.OAuth2)_|  Oauth2 for URL  |
|`bearerToken`<br/>_string_|  BearerToken used for remote_write.  |
|`bearerTokenFile`<br/>_string_|  BearerTokenFile used to read bearer token.  |
|`sigv4`<br/>_[SigV4Config](#monitoring.grafana.com/v1alpha1.SigV4Config)_|  SigV4 configures SigV4-based authentication to the remote_write endpoint. SigV4-based authentication is used if SigV4 is defined, even with an empty object.  |
|`tlsConfig`<br/>_[github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1.TLSConfig](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.TLSConfig)_|  TLSConfig to use for remote_write.  |
|`proxyUrl`<br/>_string_|  ProxyURL to proxy requests through. Optional.  |
|`queueConfig`<br/>_[QueueConfig](#monitoring.grafana.com/v1alpha1.QueueConfig)_|  QueueConfig allows tuning of the remote_write queue parameters.  |
|`metadataConfig`<br/>_[MetadataConfig](#monitoring.grafana.com/v1alpha1.MetadataConfig)_|  MetadataConfig configures the sending of series metadata to remote storage.  |
### ReplaceStageSpec <a name="monitoring.grafana.com/v1alpha1.ReplaceStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
ReplaceStageSpec is a parsing stage that parses a log line using a regular expression and replaces the log line. Named capture groups in the regex allows for adding data into the extracted map. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from extracted data to parse. If empty, defaults to using the log message.  |
|`expression`<br/>_string_|  RE2 regular expression. Each capture group MUST be named. Required.  |
|`replace`<br/>_string_|  Value to replace the captured group with.  |
### SigV4Config <a name="monitoring.grafana.com/v1alpha1.SigV4Config"></a>
(Appears on:[RemoteWriteSpec](#monitoring.grafana.com/v1alpha1.RemoteWriteSpec))
SigV4Config specifies configuration to perform SigV4 authentication. 
#### Fields
|Field|Description|
|-|-|
|`region`<br/>_string_|  Region of the AWS endpoint. If blank, the region from the default credentials chain is used.  |
|`accessKey`<br/>_[Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  AccessKey holds the secret of the AWS API access key to use for signing. If not provided, the environment variable AWS_ACCESS_KEY_ID is used.  |
|`secretKey`<br/>_[Kubernetes core/v1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core)_|  SecretKey of the AWS API to use for signing. If blank, the environment variable AWS_SECRET_ACCESS_KEY is used.  |
|`profile`<br/>_string_|  Profile is the named AWS profile to use for authentication.  |
|`roleARN`<br/>_string_|  RoleARN is the AWS Role ARN to use for authentication, as an alternative for using the AWS API keys.  |
### TemplateStageSpec <a name="monitoring.grafana.com/v1alpha1.TemplateStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
TemplateStageSpec is a transform stage that manipulates the values in the extracted map using Go&#39;s template syntax. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from extracted data to parse. Required. If empty, defaults to using the log message.  |
|`template`<br/>_string_|  Go template string to use. Required. In addition to normal template functions, ToLower, ToUpper, Replace, Trim, TrimLeft, TrimRight, TrimPrefix, and TrimSpace are also available.  |
### TenantStageSpec <a name="monitoring.grafana.com/v1alpha1.TenantStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
TenantStageSpec is an action stage that sets the tenant ID for the log entry picking it from a field in the extracted data map. 
#### Fields
|Field|Description|
|-|-|
|`label`<br/>_string_|  Name from labels whose value should be set as tenant ID. Mutually exclusive with source and value.  |
|`source`<br/>_string_|  Name from extracted data to use as the tenant ID. Mutually exclusive with label and value.  |
|`value`<br/>_string_|  Value to use for the template ID. Useful when this stage is used within a conditional pipeline such as match. Mutually exclusive with label and source.  |
### TimestampStageSpec <a name="monitoring.grafana.com/v1alpha1.TimestampStageSpec"></a>
(Appears on:[PipelineStageSpec](#monitoring.grafana.com/v1alpha1.PipelineStageSpec))
TimestampStageSpec is an action stage that can change the timestamp of a log line before it is sent to Loki. 
#### Fields
|Field|Description|
|-|-|
|`source`<br/>_string_|  Name from extracted data to use as the timestamp. Required.  |
|`format`<br/>_string_|  Determines format of the time string. Required. Can be one of: ANSIC, UnixDate, RubyDate, RFC822, RFC822Z, RFC850, RFC1123, RFC1123Z, RFC3339, RFC3339Nano, Unix, UnixMs, UnixUs, UnixNs.  |
|`fallbackFormats`<br/>_[]string_|  Fallback formats to try if format fails.  |
|`location`<br/>_string_|  IANA Timezone Database string.  |
|`actionOnFailure`<br/>_string_|  Action to take when the timestamp can&#39;t be extracted or parsed. Can be skip or fudge. Defaults to fudge.  |
