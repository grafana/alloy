{
    _config+:: {
        enableK8sCluster: true,
        enableAlloyCluster: true,
        enableLokiLogs: true,
        filterSelector: '', #use it to filter specific metric label values, ie: job=~"integrations/alloy"
        k8sClusterSelector: 'cluster=~"$cluster", namespace=~"$namespace"',
        groupSelector: if self.enableK8sCluster then self.k8sClusterSelector + ', job=~"$job"' else 'job=~"$job"',
        instanceSelector: self.groupSelector + ', instance=~"$instance"',        
        logsFilterSelector: '', #use to filter logs originated from alloy, and avoid picking up other platform logs, ie: service_name="alloy"
        dashboardTag: 'alloy-mixin',
        useSetenceCaseTemplateLabels: false,
    }
}