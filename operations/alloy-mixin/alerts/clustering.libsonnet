local alert = import './utils/alert.jsonnet';

{
  newAlloyClusterAlertsGroup(enableK8sCluster=true)::
    alert.newGroup(
      'alloy_clustering',
      [
        // Cluster not converging.
        alert.newRule(
          'ClusterNotConverging',
          if enableK8sCluster then 
            'stddev by (cluster, namespace, job) (sum without (state) (cluster_node_peers)) != 0' 
          else 
            'stddev by (job) (sum without (state) (cluster_node_peers)) != 0',
          'Cluster is not converging.',
          'Cluster is not converging: nodes report different number of peers in the cluster. Job is {{ $labels.job }}',
          '10m',
        ),

        alert.newRule(
          'ClusterNodeCountMismatch',
          // Assert that the number of known peers (regardless of state) reported by each
          // Alloy instance matches the number of running Alloy instances in the
          // same cluster and namespace as reported by a count of Prometheus
          // metrics.
          if enableK8sCluster then |||
            sum without (state) (cluster_node_peers) !=
            on (cluster, namespace, job) group_left
            count by (cluster, namespace, job) (cluster_node_info)
          ||| else |||
            sum without (state) (cluster_node_peers) !=
            on (job) group_left
            count by (job) (cluster_node_info)
          |||
          ,
          'Nodes report different number of peers vs. the count of observed Alloy metrics.',
          'Nodes report different number of peers vs. the count of observed Alloy metrics. Some Alloy metrics may be missing or the cluster is in a split brain state. Job is {{ $labels.job }}',          
          '15m',
        ),

        // Nodes health score is not zero.
        alert.newRule(
          'ClusterNodeUnhealthy',        
          |||
            cluster_node_gossip_health_score > 0
          |||,
          'Cluster unhealthy.',
          'Cluster node is reporting a gossip protocol health score > 0. Job is {{ $labels.job }}',
          '10m',
        ),

        // Node tried to join the cluster with an already-present node name.
        alert.newRule(
          'ClusterNodeNameConflict',
          if enableK8sCluster then 
            'sum by (cluster, namespace, job) (rate(cluster_node_gossip_received_events_total{event="node_conflict"}[2m])) > 0'
          else
            'sum by (job) (rate(cluster_node_gossip_received_events_total{event="node_conflict"}[2m])) > 0'
          ,
          'Cluster Node Name Conflict.',
          'A node tried to join the cluster with a name conflicting with an existing peer. Job is {{ $labels.job }}',          
          '10m',
        ),

        // Node stuck in Terminating state.
        alert.newRule(
          'ClusterNodeStuckTerminating',
          if enableK8sCluster then
            'sum by (cluster, namespace, job, instance) (cluster_node_peers{state="terminating"}) > 0'
          else
            'sum by (job, instance) (cluster_node_peers{state="terminating"}) > 0'
          ,
          'Cluster node stuck in Terminating state.',
          'There is a node within the cluster that is stuck in Terminating state. Job is {{ $labels.job }}',
          '10m',
        ),

        // Nodes are not using the same configuration file.
        alert.newRule(
          'ClusterConfigurationDrift',
          if enableK8sCluster then |||
            count without (sha256) (
                max by (cluster, namespace, sha256, job) (alloy_config_hash and on(cluster, namespace, job) cluster_node_info)
            ) > 1
          ||| else |||
            count without (sha256) (
                max by (sha256, job) (alloy_config_hash and on(job) cluster_node_info)
            ) > 1
          |||
          ,
          'Cluster configuration drifting.',
          'Cluster nodes are not using the same configuration file. Job is {{ $labels.job }}',
          '5m',
        ),
      ]
    )  
}
