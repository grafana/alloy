local alert = import './utils/alert.jsonnet';

alert.newGroup(
  'clustering',
  [
    // Cluster not converging.
    alert.newRule(
      'ClusterNotConverging',
      'stddev by (cluster, namespace) (sum without (state) (cluster_node_peers)) != 0',
      'Cluster is not converging: nodes report different number of peers in the cluster.',
      '10m',
    ),

    alert.newRule(
      'ClusterNodeCountMismatch',
      // Assert that the number of known peers (regardless of state) reported by each
      // agent matches the number of running agents in the same cluster
      // and namespace as reported by a count of Prometheus metrics.
      |||
        sum without (state) (cluster_node_peers) !=
        on (cluster, namespace) group_left
        count by (cluster, namespace) (cluster_node_info)
      |||,
      'Nodes report different number of peers vs. the count of observed agent metrics. Some agent metrics may be missing or the cluster is in a split brain state.',
      '15m',
    ),

    // Nodes health score is not zero.
    alert.newRule(
      'ClusterNodeUnhealthy',
      |||
        cluster_node_gossip_health_score > 0
      |||,
      'Cluster node is reporting a gossip protocol health score > 0.',
      '10m',
    ),

    // Node tried to join the cluster with an already-present node name.
    alert.newRule(
      'ClusterNodeNameConflict',
      'sum by (cluster, namespace) (rate(cluster_node_gossip_received_events_total{event="node_conflict"}[2m])) > 0',
      'A node tried to join the cluster with a name conflicting with an existing peer.',
      '10m',
    ),

    // Node stuck in Terminating state.
    alert.newRule(
      'ClusterNodeStuckTerminating',
      'sum by (cluster, namespace, instance) (cluster_node_peers{state="terminating"}) > 0',
      'Cluster node stuck in Terminating state.',
      '10m',
    ),

    // Nodes are not using the same configuration file.
    alert.newRule(
      'ClusterConfigurationDrift',
      |||
        count without (sha256) (
            max by (cluster, namespace, sha256) (agent_config_hash and on(cluster, namespace) cluster_node_info)
        ) > 1
      |||,
      'Cluster nodes are not using the same configuration file.',
      '5m',
    ),
  ]
)
