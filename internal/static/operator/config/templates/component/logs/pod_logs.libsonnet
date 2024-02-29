local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';
local k8s = import 'utils/k8s.libsonnet';

local new_relabel_config = import './relabel_config.libsonnet';
local new_safe_tls_config = import './safe_tls_config.libsonnet';
local new_pipeline_stage = import './stages.libsonnet';
local new_kube_sd_config = import 'component/metrics/kube_sd_config.libsonnet';

// Genrates a scrape_config from a PodLogs.
//
// @param {string} agentNamespace - Namespace the GrafanaAgent CR is in.
// @param {PodLogs} podLogs
// @param {APIServerConfig} apiServer
// @param {boolean} ignoreNamespaceSelectors
// @param {string} enforcedNamespaceLabel
function(
  agentNamespace,
  podLogs,
  apiServer,
  ignoreNamespaceSelectors,
  enforcedNamespaceLabel,
) {
  local meta = podLogs.ObjectMeta,

  job_name: 'podLogs/%s/%s' % [meta.Namespace, meta.Name],

  kubernetes_sd_configs: [
    new_kube_sd_config(
      namespace=agentNamespace,
      namespaces=k8s.namespacesFromSelector(
        podLogs.Spec.NamespaceSelector,
        meta.Namespace,
        ignoreNamespaceSelectors,
      ),
      apiServer=apiServer,
      role='pod',
    ),
  ],

  pipeline_stages: optionals.array(std.map(
    function(pipeline) new_pipeline_stage(pipeline),
    podLogs.Spec.PipelineStages,
  )),

  relabel_configs: (
    [{ source_labels: ['job'], target_label: '__tmp_prometheus_job_name' }] +

    // Match on service labels.
    std.map(
      function(k) {
        source_labels: ['__meta_kubernetes_pod_label_' + k8s.sanitize(k)],
        regex: podLogs.Spec.Selector.MatchLabels[k],
        action: 'keep',
      },
      // Keep the output consistent by sorting the keys first.
      std.sort(std.objectFields(
        if podLogs.Spec.Selector.MatchLabels != null
        then podLogs.Spec.Selector.MatchLabels
        else {}
      )),
    ) +

    // Set-based label matching. we have to map the valid relations
    // `In`, `NotIn`, `Exists`, and `DoesNotExist` into relabling rules.
    std.map(
      function(exp) (
        if exp.Operator == 'In' then {
          source_labels: ['__meta_kubernetes_pod_label_' + k8s.sanitize(exp.Key)],
          regex: std.join('|', exp.Values),
          action: 'keep',
        } else if exp.Operator == 'NotIn' then {
          source_labels: ['__meta_kubernetes_pod_label_' + k8s.sanitize(exp.Key)],
          regex: std.join('|', exp.Values),
          action: 'drop',
        } else if exp.Operator == 'Exists' then {
          source_labels: ['__meta_kubernetes_pod_labelpresent_' + k8s.sanitize(exp.Key)],
          regex: 'true',
          action: 'keep',
        } else if exp.Operator == 'DoesNotExist' then {
          source_labels: ['__meta_kubernetes_pod_labelpresent_' + k8s.sanitize(exp.Key)],
          regex: 'true',
          action: 'drop',
        }
      ),
      k8s.array(podLogs.Spec.Selector.MatchExpressions),
    ) +

    // Relabel namespace, pod, and service metalabels into proper labels.
    [{
      source_labels: ['__meta_kubernetes_namespace'],
      target_label: 'namespace',
    }, {
      source_labels: ['__meta_kubernetes_service_name'],
      target_label: 'service',
    }, {
      source_labels: ['__meta_kubernetes_pod_name'],
      target_label: 'pod',
    }, {
      source_labels: ['__meta_kubernetes_pod_container_name'],
      target_label: 'container',
    }] +

    // Relabel targetLabels from the service onto the target.
    std.map(
      function(l) {
        source_labels: ['__meta_kubernetes_pod_label_' + k8s.sanitize(l)],
        target_label: k8s.sanitize(l),
        regex: '(.+)',
        replacement: '$1',
      },
      k8s.array(podLogs.Spec.PodTargetLabels)
    ) +

    // By default, generate a safe job name from the service name.
    std.filter(function(e) e != null, [
      {
        target_label: 'job',
        replacement: '%s/%s' % [meta.Namespace, meta.Name],
      },
      if podLogs.Spec.JobLabel != '' then {
        source_labels: ['__meta_kubernetes_pod_label_' + k8s.sanitize(podLogs.Spec.JobLabel)],
        target_label: 'job',
        regex: '(.+)',
        replacement: '$1',
      },
    ]) +

    // Kubernetes puts logs under subdirectories keyed pod UID and container_name.
    [{
      source_labels: ['__meta_kubernetes_pod_uid', '__meta_kubernetes_pod_container_name'],
      target_label: '__path__',
      separator: '/',
      replacement: '/var/log/pods/*$1/*.log',
    }] +

    std.map(
      function(c) new_relabel_config(c),
      k8s.array(podLogs.Spec.RelabelConfigs),
    ) +

    // Because of security risks, whenever enforcedNamespaceLabel is set,
    // we want to append it to the relabel_configs as the last relabling to
    // ensure it overrides all other relabelings.
    std.filter(function(e) e != null, [
      if enforcedNamespaceLabel != '' then {
        target_label: enforcedNamespaceLabel,
        replacement: podLogs.ObjectMeta.Namespace,
      },
    ])
  ),
}
