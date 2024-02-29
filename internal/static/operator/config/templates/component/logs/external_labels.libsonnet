// Generates an external_label mapping. This includes the user-provided labels
// as well as the injected cluster label.
//
// @param {GrafanaAgent} agent
// @param {LogsClientSpec} client
function(agent, client) (
  local meta = agent.ObjectMeta;
  local logs = agent.Spec.Logs;

  // Provide the cluster label first. Doing it this way allows the user to
  // override with a value they choose.
  (
    local clusterValue = '%s/%s' % [meta.Namespace, meta.Name];
    local clusterLabel = logs.LogsExternalLabelName;

    if clusterLabel == null then { cluster: clusterValue }
    else if clusterLabel != '' then { [clusterLabel]: clusterValue }
    else {}
  ) +

  // Finally add in any user-configured labels.
  (
    if client.ExternalLabels != null
    then client.ExternalLabels
    else {}
  )
)
