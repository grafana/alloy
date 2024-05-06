{
    _config+:: {
        enableK8sCluster: true,
        enableAlloyCluster: true,
        enableLokiLogs: true,
        filterSelector: 'job="$job"',
        groupSelector: if self.enableK8sCluster then self.filterSelector + ', ' + self.k8sClusterSelector else self.filterSelector,
        instanceSelector: self.groupSelector + ', instance="$instance"',
        k8sClusterSelector: 'cluster="$cluster", namespace="$namespace", ',
        dashboardTag: 'alloy-mixin'
    }
}