{
    _config+:: {
        enableK8sCluster: true,
        enableAlloyCluster: true,
        enableLokiLogs: true,
        filterSelector: 'job=~"integrations/self"', #default job name used by alloy "self" exporter
        k8sClusterSelector: 'cluster=~"$cluster", namespace=~"$namespace"',
        groupSelector: if self.enableK8sCluster then self.k8sClusterSelector + ', job="$job"' else 'job="$job"',        
        instanceSelector: self.groupSelector + ', instance=~"$instance"',        
        logsFilterSelector: 'service_name="alloy"', #use to filter logs originated from alloy, and avoid picking up other platform logs
        dashboardTag: 'alloy-mixin'
    }
}