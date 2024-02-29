// Generates an external_label mapping. This includes the
// user-provided labels as well as the injected cluster and
// replica labels.
//
// @param {config.Deployment} ctx
// @param {bool} addReplica
function(ctx, addReplica) (
  local meta = ctx.Agent.ObjectMeta;
  local metrics = ctx.Agent.Spec.Metrics;

  // Provide the cluster label first. Doing it this way allows the user to
  // override with a value they choose.
  (
    local clusterValue = '%s/%s' % [meta.Namespace, meta.Name];
    local clusterLabel = metrics.MetricsExternalLabelName;

    if clusterLabel == null then { cluster: clusterValue }
    else if clusterLabel != '' then { [clusterLabel]: clusterValue }
    else {}
  ) +

  // Then add in any user-configured labels.
  (
    if metrics.ExternalLabels == null then {}
    else metrics.ExternalLabels
  ) +

  // Finally, add the replica label. We don't want the user to overwrite the
  // replica label since it can cause duplicate sample problems.
  if !addReplica then {} else (
    local replicaValue = 'replica-$(STATEFULSET_ORDINAL_NUMBER)';
    local replicaLabel = metrics.ReplicaExternalLabelName;

    if replicaLabel == null then { __replica__: replicaValue }
    else if replicaLabel != '' then { [replicaLabel]: replicaValue }
    else {}
  )
)
