local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';
local k8s = import 'utils/k8s.libsonnet';

local new_kube_sd_config = import './kube_sd_config.libsonnet';
local new_relabel_config = import './relabel_config.libsonnet';
local new_tls_config = import './tls_config.libsonnet';

// Genrates a scrape_config from a Probe.
//
// @param {string} agentNamespace - Namespace the GrafanaAgent CR is in.
// @param {Probe} probe
// @param {APIServerConfig} apiServer
// @param {boolean} overrideHonorTimestamps
// @param {boolean} ignoreNamespaceSelectors
// @param {string} enforcedNamespaceLabel
// @param {*number} enforcedSampleLimit
// @param {*number} enforcedTargetLimit
// @param {number} shards
function(
  agentNamespace,
  probe,
  apiServer,
  overrideHonorTimestamps,
  ignoreNamespaceSelectors,
  enforcedNamespaceLabel,
  enforcedSampleLimit,
  enforcedTargetLimit,
  shards,
) {
  local meta = probe.ObjectMeta,

  job_name: 'probe/%s/%s' % [meta.Namespace, meta.Name],

  honor_timestamps:
    local honor = k8s.honorTimestamps(true, overrideHonorTimestamps);
    if honor != null then honor,

  local path =
    if probe.Spec.ProberSpec.Path == ''
    then '/probe'
    else probe.Spec.ProberSpec.Path,
  metrics_path: path,

  scrape_interval: optionals.string(probe.Spec.Interval),
  scrape_timeout: optionals.string(probe.Spec.ScrapeTimeout),
  scheme: optionals.string(probe.Spec.ProberSpec.Scheme),
  params: {
    module: [probe.Spec.Module],
  },

  tls_config:
    if probe.Spec.TLSConfig != null then new_tls_config(meta.Namespace, probe.Spec.TLSConfig),
  bearer_token:
    if probe.Spec.BearerTokenSecret.LocalObjectReference.Name != ''
    then secrets.valueForSecret(meta.Namespace, probe.Spec.BearerTokenSecret),

  basic_auth: if probe.Spec.BasicAuth != null then {
    username: secrets.valueForSecret(meta.Namespace, probe.Spec.BasicAuth.Username),
    password: secrets.valueForSecret(meta.Namespace, probe.Spec.BasicAuth.Password),
  },

  // Generate static_configs section if StaticConfig is provided.
  static_configs: optionals.array(if probe.Spec.Targets.StaticConfig != null then [{
    targets: probe.Spec.Targets.StaticConfig.Targets,
    labels: (
      if probe.Spec.Targets.StaticConfig.Labels != null
      then probe.Spec.Targets.StaticConfig.Labels {
        namespace: meta.Namespace,
      }
      else { namespace: meta.Namespace }
    ),
  }]),

  // Generate kubernetes_sd_configs section if StaticConfig is *not* provided.
  kubernetes_sd_configs: optionals.array(if probe.Spec.Targets.StaticConfig == null then [
    new_kube_sd_config(
      namespace=agentNamespace,
      namespaces=k8s.namespacesFromSelector(
        probe.Spec.Targets.Ingress.NamespaceSelector,
        meta.Namespace,
        ignoreNamespaceSelectors,
      ),
      apiServer=apiServer,
      role='ingress',
    ),
  ]),

  relabel_configs: (
    [{ source_labels: ['job'], target_label: '__tmp_prometheus_job_name' }] +

    std.filter(function(e) e != null, [
      if probe.Spec.JobName != '' then {
        target_label: 'job',
        replacement: probe.Spec.JobName,
      },
    ]) +

    // Relabelings for static_config.
    k8s.array(
      if probe.Spec.Targets.StaticConfig != null then
        [{
          source_labels: ['__address__'],
          target_label: '__param_target',
        }, {
          source_labels: ['__param_target'],
          target_label: 'instance',
        }, {
          target_label: '__address__',
          replacement: probe.Spec.ProberSpec.URL,
        }] +

        // Add configured relablings
        std.map(
          function(r) new_relabel_config(r),
          k8s.array(probe.Spec.Targets.StaticConfig.RelabelConfigs),
        )
    ) +

    // Relablings for kubernetes_sd_config.
    k8s.array(
      if probe.Spec.Targets.StaticConfig == null then
        // Match on service labels.
        std.map(
          function(k) {
            source_labels: ['__meta_kubernetes_ingress_label_' + k8s.sanitize(k)],
            regex: probe.Spec.Targets.Ingress.Selector.MatchLabels[k],
            action: 'keep',
          },
          // Keep the output consistent by sorting the keys first.
          std.sort(std.objectFields(
            if probe.Spec.Targets.Ingress.Selector.MatchLabels != null
            then probe.Spec.Targets.Ingress.Selector.MatchLabels
            else {}
          )),
        ) +

        // Set-based label matching. we have to map the valid relations
        // `In`, `NotIn`, `Exists`, and `DoesNotExist` into relabling rules.
        std.map(
          function(exp) (
            if exp.Operator == 'In' then {
              source_labels: ['__meta_kubernetes_ingress_label_' + k8s.sanitize(exp.Key)],
              regex: std.join('|', exp.Values),
              action: 'keep',
            } else if exp.Operator == 'NotIn' then {
              source_labels: ['__meta_kubernetes_ingress_label_' + k8s.sanitize(exp.Key)],
              regex: std.join('|', exp.Values),
              action: 'drop',
            } else if exp.Operator == 'Exists' then {
              source_labels: ['__meta_kubernetes_ingress_labelpresent_' + k8s.sanitize(exp.Key)],
              regex: 'true',
              action: 'keep',
            } else if exp.Operator == 'DoesNotExist' then {
              source_labels: ['__meta_kubernetes_ingress_labelpresent_' + k8s.sanitize(exp.Key)],
              regex: 'true',
              action: 'drop',
            }
          ),
          k8s.array(probe.Spec.Targets.Ingress.Selector.MatchExpressions),
        ) +

        // Relablings for ingress SD
        [
          {
            source_labels: [
              '__meta_kubernetes_ingress_scheme',
              '__address__',
              '__meta_kubernetes_ingress_path',
            ],
            separator: ';',
            regex: '(.+);(.+);(.+)',
            target_label: '__param_target',
            replacement: '$1://$2$3',
            action: 'replace',
          },
          {
            source_labels: ['__meta_kubernetes_namespace'],
            target_label: 'namespace',
          },
          {
            source_labels: ['__meta_kubernetes_ingress_name'],
            target_label: 'ingress',
          },
        ] +

        // Relablings for prober
        [
          {
            source_labels: ['__param_target'],
            target_label: 'instance',
          },
          {
            target_label: '__address__',
            replacement: probe.Spec.ProberSpec.URL,
          },
        ] +

        // Add configured relablings.
        std.map(
          function(r) new_relabel_config(r),
          k8s.array(probe.Spec.Targets.Ingress.RelabelConfigs),
        )
    ) +

    // Because of security risks, whenever enforcedNamespaceLabel is set,
    // we want to append it to the relabel_configs as the last relabling to
    // ensure it overrides all other relabelings.
    std.filter(function(e) e != null, [
      if enforcedNamespaceLabel != '' then {
        target_label: enforcedNamespaceLabel,
        replacement: probe.ObjectMeta.Namespace,
      },
    ])
  ),
}
