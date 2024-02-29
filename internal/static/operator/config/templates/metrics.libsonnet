local marshal = import 'ext/marshal.libsonnet';
local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';
local k8s = import 'utils/k8s.libsonnet';

local new_pod_monitor = import 'component/metrics/pod_monitor.libsonnet';
local new_probe = import 'component/metrics/probe.libsonnet';
local new_remote_write = import 'component/metrics/remote_write.libsonnet';
local new_service_monitor = import 'component/metrics/service_monitor.libsonnet';

// Generates a metrics_instance.
//
// @param {string} agentNamespace - namespace of the GrafanaAgent
// @param {MetricsInstance} instance
// @param {APIServerConfig} apiServer
// @param {boolean} overrideHonorLabels
// @param {boolean} overrideHonorTimestamps
// @param {boolean} ignoreNamespaceSelectors
// @param {string} enforcedNamespaceLabel
// @param {boolean} enforcedSampleLimit
// @param {boolean} enforcedTargetLimit
// @param {number} shards
function(
  agentNamespace,
  instance,
  apiServer,
  overrideHonorLabels,
  overrideHonorTimestamps,
  ignoreNamespaceSelectors,
  enforcedNamespaceLabel,
  enforcedSampleLimit,
  enforcedTargetLimit,
  shards,
) {
  local namespace = instance.Instance.ObjectMeta.Namespace,
  local spec = instance.Instance.Spec,

  name: '%s/%s' % [namespace, instance.Instance.ObjectMeta.Name],
  wal_truncate_frequency: optionals.string(spec.WALTruncateFrequency),
  min_wal_time: optionals.string(spec.MinWALTime),
  max_wal_time: optionals.string(spec.MaxWALTime),
  remote_flush_deadline: optionals.string(spec.RemoteFlushDeadline),

  // WriteStaleOnShutdown is a *bool in the code. We need to check for null-ness here.
  write_stale_on_shutdown:
    if spec.WriteStaleOnShutdown != null then optionals.bool(spec.WriteStaleOnShutdown),

  remote_write: optionals.array(std.map(
    function(rw) new_remote_write(namespace, rw),
    spec.RemoteWrite,
  )),

  // This is probably the most complicated code fragment in the whole Jsonnet
  // codebase.
  //
  // We've pulled a set of ServiceMonitors, PodMonitors, Probes.
  // We need to iterate over all of these and convert them into scrape_configs.
  scrape_configs: optionals.array(
    // Iterate over ServiceMonitors. ServiceMonitors have a set of Endpoints,
    // each of which should be its own scrape_configs, so we have to do a nested
    // iteration here.
    std.flatMap(
      function(sMon) std.mapWithIndex(
        function(i, ep) new_service_monitor(
          agentNamespace=agentNamespace,
          monitor=sMon,
          endpoint=ep,
          index=i,
          apiServer=apiServer,
          overrideHonorLabels=overrideHonorLabels,
          overrideHonorTimestamps=overrideHonorTimestamps,
          ignoreNamespaceSelectors=ignoreNamespaceSelectors,
          enforcedNamespaceLabel=enforcedNamespaceLabel,
          enforcedSampleLimit=enforcedSampleLimit,
          enforcedTargetLimit=enforcedTargetLimit,
          shards=shards,
        ),
        k8s.array(sMon.Spec.Endpoints),
      ),
      k8s.array(instance.ServiceMonitors),
    ) +

    // Iterate over PodMonitors. PodMonitors have a set of PodMetricsEndpoints,
    // each of which should be its own scrape_configs, so we have to do a
    // nested iteration here.
    std.flatMap(
      function(pMon) std.mapWithIndex(
        function(i, ep) new_pod_monitor(
          agentNamespace=agentNamespace,
          monitor=pMon,
          endpoint=ep,
          index=i,
          apiServer=apiServer,
          overrideHonorLabels=overrideHonorLabels,
          overrideHonorTimestamps=overrideHonorTimestamps,
          ignoreNamespaceSelectors=ignoreNamespaceSelectors,
          enforcedNamespaceLabel=enforcedNamespaceLabel,
          enforcedSampleLimit=enforcedSampleLimit,
          enforcedTargetLimit=enforcedTargetLimit,
          shards=shards,
        ),
        k8s.array(pMon.Spec.PodMetricsEndpoints),
      ),
      k8s.array(instance.PodMonitors),
    ) +

    // Iterate over Probes. Each probe only converts into one scrape_config.
    std.map(
      function(probe) new_probe(
        agentNamespace=agentNamespace,
        probe=probe,
        apiServer=apiServer,
        overrideHonorTimestamps=overrideHonorTimestamps,
        ignoreNamespaceSelectors=ignoreNamespaceSelectors,
        enforcedNamespaceLabel=enforcedNamespaceLabel,
        enforcedSampleLimit=enforcedSampleLimit,
        enforcedTargetLimit=enforcedTargetLimit,
        shards=shards,
      ),
      k8s.array(instance.Probes),
    ) +

    // Finally, if the user specified additional scrape configs, we need to
    // extract their value from the secret and then unmarshal them into the
    // array.
    k8s.array(
      if spec.AdditionalScrapeConfigs != null then (
        local rawYAML = secrets.valueForSecret(namespace, spec.AdditionalScrapeConfigs);
        marshal.fromYAML(rawYAML)
      )
    ),
  ),
}
