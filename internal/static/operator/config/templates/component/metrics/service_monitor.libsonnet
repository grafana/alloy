local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';
local k8s = import 'utils/k8s.libsonnet';

local new_kube_sd_config = import './kube_sd_config.libsonnet';
local new_relabel_config = import './relabel_config.libsonnet';
local new_tls_config = import './tls_config.libsonnet';

// Genrates a scrape_config from a ServiceMonitor.
//
// @param {string} agentNamespace - Namespace the GrafanaAgent CR is in.
// @param {ServiceMonitor} monitor
// @param {Endpoint} endpoint - endpoint within the monitor
// @param {number} index - index of the endpoint
// @param {APIServerConfig} apiServer
// @param {boolean} overrideHonorLabels
// @param {boolean} overrideHonorTimestamps
// @param {boolean} ignoreNamespaceSelectors
// @param {string} enforcedNamespaceLabel
// @param {*number} enforcedSampleLimit
// @param {*number} enforcedTargetLimit
// @param {number} shards
function(
  agentNamespace,
  monitor,
  endpoint,
  index,
  apiServer,
  overrideHonorLabels,
  overrideHonorTimestamps,
  ignoreNamespaceSelectors,
  enforcedNamespaceLabel,
  enforcedSampleLimit,
  enforcedTargetLimit,
  shards,
) {
  local meta = monitor.ObjectMeta,

  job_name: 'serviceMonitor/%s/%s/%d' % [meta.Namespace, meta.Name, index],
  honor_labels: k8s.honorLabels(endpoint.HonorLabels, overrideHonorLabels),

  // We only want to provide honorTimestamps in the file when it's not null.
  honor_timestamps:
    local honor = k8s.honorTimestamps(endpoint.HonorTimestamps, overrideHonorTimestamps);
    if honor != null then honor,

  kubernetes_sd_configs: [
    new_kube_sd_config(
      namespace=agentNamespace,
      namespaces=k8s.namespacesFromSelector(
        monitor.Spec.NamespaceSelector,
        meta.Namespace,
        ignoreNamespaceSelectors,
      ),
      apiServer=apiServer,
      role='endpoints',
    ),
  ],

  scrape_interval: optionals.string(endpoint.Interval),
  scrape_timeout: optionals.string(endpoint.ScrapeTimeout),
  metrics_path: optionals.string(endpoint.Path),
  proxy_url: optionals.string(endpoint.ProxyURL),
  params: optionals.object(endpoint.Params),
  scheme: optionals.string(endpoint.Scheme),
  enable_http2: optionals.bool(endpoint.EnableHttp2,true),

  tls_config:
    if endpoint.TLSConfig != null then new_tls_config(meta.Namespace, endpoint.TLSConfig),
  bearer_token_file: optionals.string(endpoint.BearerTokenFile),
  bearer_token:
    if endpoint.BearerTokenSecret.LocalObjectReference.Name != ''
    then secrets.valueForSecret(meta.Namespace, endpoint.BearerTokenSecret),

  basic_auth: if endpoint.BasicAuth != null then {
    username: secrets.valueForSecret(meta.Namespace, endpoint.BasicAuth.Username),
    password: secrets.valueForSecret(meta.Namespace, endpoint.BasicAuth.Password),
  },

  relabel_configs: (
    [{ source_labels: ['job'], target_label: '__tmp_prometheus_job_name' }] +

    // Match on service labels.
    std.map(
      function(k) {
        source_labels: ['__meta_kubernetes_service_label_' + k8s.sanitize(k)],
        regex: monitor.Spec.Selector.MatchLabels[k],
        action: 'keep',
      },
      // Keep the output consistent by sorting the keys first.
      std.sort(std.objectFields(
        if monitor.Spec.Selector.MatchLabels != null
        then monitor.Spec.Selector.MatchLabels
        else {}
      )),
    ) +

    // Set-based label matching. we have to map the valid relations
    // `In`, `NotIn`, `Exists`, and `DoesNotExist` into relabling rules.
    std.map(
      function(exp) (
        if exp.Operator == 'In' then {
          source_labels: ['__meta_kubernetes_service_label_' + k8s.sanitize(exp.Key)],
          regex: std.join('|', exp.Values),
          action: 'keep',
        } else if exp.Operator == 'NotIn' then {
          source_labels: ['__meta_kubernetes_service_label_' + k8s.sanitize(exp.Key)],
          regex: std.join('|', exp.Values),
          action: 'drop',
        } else if exp.Operator == 'Exists' then {
          source_labels: ['__meta_kubernetes_service_labelpresent_' + k8s.sanitize(exp.Key)],
          regex: 'true',
          action: 'keep',
        } else if exp.Operator == 'DoesNotExist' then {
          source_labels: ['__meta_kubernetes_service_labelpresent_' + k8s.sanitize(exp.Key)],
          regex: 'true',
          action: 'drop',
        }
      ),
      k8s.array(monitor.Spec.Selector.MatchExpressions),
    ) +

    // First targets based on correct port for the endpoint. If ep.Port,
    // ep.TargetPort.StrVal, or ep.TargetPort.IntVal aren't set, then
    // we'll have a null relabel_configs, which will be filtered out.
    //
    // We do this to avoid having an array with a null element inside of it.
    std.filter(function(element) element != null, [
      if endpoint.Port != '' then {
        source_labels: ['__meta_kubernetes_endpoint_port_name'],
        regex: endpoint.Port,
        action: 'keep',
      } else if endpoint.TargetPort != null then (
        if endpoint.TargetPort.StrVal != '' then {
          source_labels: ['__meta_kubernetes_pod_container_port_name'],
          regex: endpoint.TargetPort.StrVal,
          action: 'keep',
        } else if endpoint.TargetPort.IntVal != 0 then {
          source_labels: ['__meta_kubernetes_pod_container_port_number'],
          regex: std.toString(endpoint.TargetPort.IntVal),
          action: 'keep',
        }
      ),
    ]) +

    // Relabel namespace, pod, and service metalabels into proper labels.
    [{
      source_labels: [
        '__meta_kubernetes_endpoint_address_target_kind',
        '__meta_kubernetes_endpoint_address_target_name',
      ],
      target_label: 'node',
      separator: ';',
      regex: 'Node;(.*)',
      replacement: '$1',
    }, {
      source_labels: [
        '__meta_kubernetes_endpoint_address_target_kind',
        '__meta_kubernetes_endpoint_address_target_name',
      ],
      target_label: 'pod',
      separator: ';',
      regex: 'Pod;(.*)',
      replacement: '$1',
    }, {
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

    (if endpoint.FilterRunning != null && endpoint.FilterRunning then [{
      source_labels: ['__meta_kubernetes_pod_phase'],
			regex: '(Failed|Succeeded)',
			action: 'drop',
    }] else [] ) +

    // Relabel targetLabels from the service onto the target.
    std.map(
      function(l) {
        source_labels: ['__meta_kubernetes_service_label_' + k8s.sanitize(l)],
        target_label: k8s.sanitize(l),
        regex: '(.+)',
        replacement: '$1',
      },
      k8s.array(monitor.Spec.TargetLabels)
    ) +
    std.map(
      function(l) {
        source_labels: ['__meta_kubernetes_pod_label_' + k8s.sanitize(l)],
        target_label: k8s.sanitize(l),
        regex: '(.+)',
        replacement: '$1',
      },
      k8s.array(monitor.Spec.PodTargetLabels)
    ) +

    // By default, generate a safe job name from the service name. We also keep
    // this around if a jobLabel is set just in case targets don't actually have
    // a value for it. A single service may potentially have multiple metrics
    // endpoints, therefore the endpoints labels is filled with the ports name
    // or (as a fallback) the port number.
    std.filter(function(e) e != null, [
      {
        source_labels: ['__meta_kubernetes_service_name'],
        target_label: 'job',
        replacement: '$1',
      },
      if monitor.Spec.JobLabel != '' then {
        source_labels: ['__meta_kubernetes_service_label_' + k8s.sanitize(monitor.Spec.JobLabel)],
        target_label: 'job',
        regex: '(.+)',
        replacement: '$1',
      },
    ]) +

    std.filter(function(e) e != null, [
      if endpoint.Port != '' then {
        target_label: 'endpoint',
        replacement: endpoint.Port,
      } else if k8s.intOrString(endpoint.TargetPort) != '' then {
        target_label: 'endpoint',
        replacement: k8s.intOrString(endpoint.TargetPort),
      },
    ]) +

    std.map(
      function(c) new_relabel_config(c),
      k8s.array(endpoint.RelabelConfigs),
    ) +

    // Because of security risks, whenever enforcedNamespaceLabel is set,
    // we want to append it to the relabel_configs as the last relabling to
    // ensure it overrides all other relabelings.
    std.filter(function(e) e != null, [
      if enforcedNamespaceLabel != '' then {
        target_label: enforcedNamespaceLabel,
        replacement: monitor.ObjectMeta.Namespace,
      },

      // Shard rules
      {
        source_labels: ['__address__'],
        target_label: '__tmp_hash',
        modulus: shards,
        action: 'hashmod',
      },
      {
        source_labels: ['__tmp_hash'],
        regex: '$(SHARD)',
        action: 'keep',
      },
    ])
  ),

  metric_relabel_configs: if endpoint.MetricRelabelConfigs != null then optionals.array(
    std.filterMap(
      function(c) !(c.TargetLabel != '' && enforcedNamespaceLabel != '' && c.TargetLabel == enforcedNamespaceLabel),
      function(c) new_relabel_config(c),
      k8s.array(endpoint.MetricRelabelConfigs),
    )
  ),

  sample_limit:
    if monitor.Spec.SampleLimit > 0 || enforcedSampleLimit != null
    then k8s.limit(monitor.Spec.SampleLimit, enforcedSampleLimit),
  target_limit:
    if monitor.Spec.TargetLimit > 0 || enforcedTargetLimit != null
    then k8s.limit(monitor.Spec.TargetLimit, enforcedTargetLimit),
}
