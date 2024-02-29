local marshal = import 'ext/marshal.libsonnet';
local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';
local k8s = import 'utils/k8s.libsonnet';

local new_client = import 'component/logs/client.libsonnet';
local new_pod_logs = import 'component/logs/pod_logs.libsonnet';

// Generates a logs_instance.
//
// @param {GrafanaAgent} agent
// @param {LogsSubsystemSpec} global - global logs settings & defaults
// @param {LogInstance} instance
// @param {APIServerConfig} apiServer
// @param {boolean} ignoreNamespaceSelectors
// @param {string} enforcedNamespaceLabel
function(
  agent,
  global,
  instance,
  apiServer,
  ignoreNamespaceSelectors,
  enforcedNamespaceLabel,
) {
  local agentNamespace = agent.ObjectMeta.Namespace,
  local meta = instance.Instance.ObjectMeta,
  local spec = instance.Instance.Spec,

  name: '%s/%s' % [meta.Namespace, meta.Name],

  // Figure out what set of clients to use and what namespace they're in.
  // We'll only use the global set of clients if the local LogsInstance doesn't
  // have a set of clients defined.
  //
  // Local clients come from the namespace of the LogsInstance and global
  // clients from the Agent's namespace.
  local clients =
    if std.length(spec.Clients) != 0
    then { ns: meta.Namespace, list: spec.Clients }
    else { ns: agentNamespace, list: global.Clients },

  clients: optionals.array(std.map(
    function(spec) new_client(agent, clients.ns, spec),
    clients.list,
  )),

  scrape_configs: optionals.array(
    // Iterate over PodLogs. Each PodMonitors converts into a
    // single scrape_config.
    std.map(
      function(podLogs) new_pod_logs(
        agentNamespace=agentNamespace,
        podLogs=podLogs,
        apiServer=apiServer,
        ignoreNamespaceSelectors=ignoreNamespaceSelectors,
        enforcedNamespaceLabel=enforcedNamespaceLabel,
      ),
      k8s.array(instance.PodLogs)
    ) +

    // If the user specified additional scrape configs, we need to extract
    // their value from the secret and then unmarshal them into the array.
    k8s.array(
      if spec.AdditionalScrapeConfigs != null then (
        local rawYAML = secrets.valueForSecret(meta.Namespace, spec.AdditionalScrapeConfigs);
        marshal.fromYAML(rawYAML)
      )
    ),
  ),

  target_config: if spec.TargetConfig != null then {
    sync_period: optionals.string(spec.TargetConfig.SyncPeriod),
  },
}
