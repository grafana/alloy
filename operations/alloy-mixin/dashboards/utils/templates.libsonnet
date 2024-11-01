local dashboard = import './dashboard.jsonnet';

{
    newTemplateVariablesList(filterSelector='', enableK8sCluster=true, enableAlloyCluster=true, includeInstance=true, setenceCaseLabels=false):: (        

        local clusterTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components, cluster)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s}, cluster)
            ||| % filterSelector;

        local namespaceTemplateQuery =
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster"}, namespace)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster"}, namespace)
            ||| % filterSelector;
        
        local k8sAlloyClusterTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster", namespace=~"$namespace"}, cluster_name)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster", namespace=~"$namespace"}, cluster_name)
            ||| % filterSelector;
        
        local k8sJobTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster", namespace=~"$namespace"}, job)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster", namespace=~"$namespace"}, job)
            ||| % filterSelector;
        
        local k8sInstanceTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)
            ||| 
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)
            ||| % filterSelector;

        local alloyClusterTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components, cluster_name)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s}, cluster_name)
            ||| % filterSelector;

        local jobTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components, job)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s}, job)
            ||| % filterSelector;
        
        local instanceTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{job=~"$job"}, instance)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, job=~"$job"}, instance)
            ||| % filterSelector;
        
        if enableK8sCluster then
            [
                dashboard.newTemplateVariable(
                name='cluster', 
                query=clusterTemplateQuery,
                setenceCaseLabels=setenceCaseLabels),
                dashboard.newTemplateVariable(
                name='namespace', 
                query=namespaceTemplateQuery,
                setenceCaseLabels=setenceCaseLabels),
                dashboard.newTemplateVariable(
                name='job', 
                query=k8sJobTemplateQuery,
                setenceCaseLabels=setenceCaseLabels),
            ] + 
            if enableAlloyCluster then
                [
                    dashboard.newTemplateVariable(
                    name='alloyCluster',
                    query=k8sAlloyClusterTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels),
                ]
            else [] +
            if includeInstance then
                [   
                    dashboard.newMultiTemplateVariable(
                    name='instance', 
                    query=k8sInstanceTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels) 
                ]
            else []
        else
            [
                dashboard.newTemplateVariable(
                name='job', 
                query=jobTemplateQuery,
                setenceCaseLabels=setenceCaseLabels),                            
            ] + 
            if enableAlloyCluster then
                [
                    dashboard.newTemplateVariable(
                    name='alloyCluster',
                    query=alloyClusterTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels), 
                ]
            else [] +
            if includeInstance then
                [
                    dashboard.newMultiTemplateVariable(
                    name='instance', 
                    query=instanceTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels)
                ]
            else []
    )
}