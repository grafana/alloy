{
    _config+:: {
        enableK8sCluster: false,
        enableAlloyCluster: true,
        enableLokiLogs: true,
        filterSelector: 'job=~"$job"',
        groupSelector: if self.enableK8sCluster then self.k8sClusterSelector + ', ' + self.filterSelector else self.filterSelector,
        instanceSelector: self.groupSelector + ', instance=~"$instance"',
        k8sClusterSelector: 'cluster=~"$cluster", namespace=~"$namespace"',
        dashboardTag: 'alloy-mixin'
    }
}