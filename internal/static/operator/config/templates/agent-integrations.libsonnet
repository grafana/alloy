// agent-integrations.libsonnet is the entrypoint for rendering a Grafana Agent
// config file for integrations based on the Operator custom resources.
//
// When writing an object, any field will null will be removed from the final
// YAML. This is useful as we don't want to always translate unfilled values
// from the custom resources to a field in the YAML.
//
// A series of helper methods to convert default values into null (so they can
// be trimmed) are in ./ext/optionals.libsonnet.
//
// When writing a new function, please document the expected types of the
// arguments.

local marshal = import 'ext/marshal.libsonnet';
local optionals = import 'ext/optionals.libsonnet';

local new_integration = import './integrations.libsonnet';
local new_logs_instance = import './logs.libsonnet';
local new_metrics_instance = import './metrics.libsonnet';
local new_external_labels = import 'component/metrics/external_labels.libsonnet';
local new_remote_write = import 'component/metrics/remote_write.libsonnet';

local calculateShards(requested) =
  if requested == null then 1
  else if requested > 1 then requested
  else 1;

// Renders a new config for integrations. The ctx should have all
// MetricsInstances and LogsInstances so integrations can self-collect
// telemetry data, but *Monitor-like resources are ignored.
//
// @param {config.Deployment} ctx
function(ctx) marshal.YAML(optionals.trim({
  local spec = ctx.Agent.Spec,
  local metrics = spec.Metrics,
  local logs = spec.Logs,
  local namespace = ctx.Agent.ObjectMeta.Namespace,

  server: {
    log_level: optionals.string(spec.LogLevel),
    log_format: optionals.string(spec.LogFormat),
  },

  metrics: {
    local scrubbed_instances = std.map(
      function(inst) {
        Instance: inst.Instance,
        ServiceMonitors: [],
        PodMonitors: [],
        Probes: [],
      },
      ctx.Metrics,
    ),

    wal_directory: '/var/lib/grafana-agent/data',
    global: {
      // NOTE(rfratto): we don't want to add the replica label here, since
      // there will never be more than one HA replica for a running
      // integration. Adding a replica label will cause it to be subject to
      // HA dedupe and risk being discarded depending on what the active
      // replica is server-side.
      external_labels: optionals.object(new_external_labels(ctx, false)),
      scrape_interval: optionals.string(metrics.ScrapeInterval),
      scrape_timeout: optionals.string(metrics.ScrapeTimeout),
      remote_write: optionals.array(std.map(
        function(rw) new_remote_write(ctx.Agent.ObjectMeta.Namespace, rw),
        metrics.RemoteWrite,
      )),
    },
    configs: optionals.array(std.map(
      function(inst) new_metrics_instance(
        agentNamespace=ctx.Agent.ObjectMeta.Namespace,
        instance=inst,
        apiServer=spec.APIServerConfig,
        overrideHonorLabels=metrics.OverrideHonorLabels,
        overrideHonorTimestamps=metrics.OverrideHonorTimestamps,
        ignoreNamespaceSelectors=metrics.IgnoreNamespaceSelectors,
        enforcedNamespaceLabel=metrics.EnforcedNamespaceLabel,
        enforcedSampleLimit=metrics.EnforcedSampleLimit,
        enforcedTargetLimit=metrics.EnforcedTargetLimit,
        shards=calculateShards(metrics.Shards),
      ),
      scrubbed_instances,
    )),
  },

  logs: {
    local scrubbed_instances = std.map(
      function(inst) {
        Instance: inst.Instance,
        PodLogs: [],
      },
      ctx.Logs,
    ),

    positions_directory: '/var/lib/grafana-agent/data',
    configs: optionals.array(std.map(
      function(logs_inst) new_logs_instance(
        agent=ctx.Agent,
        global=logs,
        instance=logs_inst,
        apiServer=spec.APIServerConfig,
        ignoreNamespaceSelectors=logs.IgnoreNamespaceSelectors,
        enforcedNamespaceLabel=logs.EnforcedNamespaceLabel,
      ),
      scrubbed_instances,
    )),
  },

  integrations: {
    // Integrations should opt-in to autoscrape.
    metrics: {
      autoscrape: {
        enable: false,
      },
    },
  } + (
    // Iterate over our Integration CRs and map them to an object. All
    // integrations are stored in a <name>_configs array, even if they're
    // unique.
    std.foldl(
      function(acc, element) acc {
        [element.Instance.Spec.Name + '_configs']: (
          local key = element.Instance.Spec.Name + '_configs';
          local entry = new_integration(element.Instance);

          if std.objectHas(acc, key) then acc[key] + [entry]
          else [entry]
        ),
      },
      ctx.Integrations,
      {},
    )
  ),
}))
