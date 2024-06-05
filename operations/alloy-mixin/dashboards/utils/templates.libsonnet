local dashboard = import './dashboard.jsonnet';

{
    newTemplateVariablesList(filterSelector='', enableK8sCluster=true, includeInstance=true, useSentenceCaseLabel=false):: {        

        local clusterTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components, cluster)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s}, cluster)
            ||| % filterSelector,

        local namespaceTemplateQuery =
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster"}, namespace)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster"}, namespace)
            ||| % filterSelector,
        
        local k8sJobTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster", namespace=~"$namespace"}, job)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster", namespace=~"$namespace"}, job)
            ||| % filterSelector,
        
        local k8sInstanceTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{cluster=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)
            ||| 
            else
            |||
                label_values(alloy_component_controller_running_components{%s, cluster=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)
            ||| % filterSelector,

        local jobTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components, job)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s}, job)
            ||| % filterSelector,
        
        local instanceTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{job=~"$job"}, instance)
            |||
            else
            |||
                label_values(alloy_component_controller_running_components{%s, job=~"$job"}, instance)
            ||| % filterSelector,

        variables:  
            if enableK8sCluster then
            [
                dashboard.newTemplateVariable(
                name='cluster', 
                query=clusterTemplateQuery,
                useSentenceCaseLabel=useSentenceCaseLabel),
                dashboard.newTemplateVariable(
                name='namespace', 
                query=namespaceTemplateQuery,
                useSentenceCaseLabel=useSentenceCaseLabel),
                dashboard.newMultiTemplateVariable(
                name='job', 
                query=k8sJobTemplateQuery,
                useSentenceCaseLabel=useSentenceCaseLabel),
                if includeInstance then
                    dashboard.newMultiTemplateVariable(
                    name='instance', 
                    query=k8sInstanceTemplateQuery,
                useSentenceCaseLabel=useSentenceCaseLabel),
            ]
            else
            [
                dashboard.newMultiTemplateVariable(
                name='job', 
                query=jobTemplateQuery,
                useSentenceCaseLabel=useSentenceCaseLabel),
                if includeInstance then
                    dashboard.newMultiTemplateVariable(
                    name='instance', 
                    query=instanceTemplateQuery,
                useSentenceCaseLabel=useSentenceCaseLabel),
            ],        
    }
}