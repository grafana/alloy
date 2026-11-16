local dashboard = import './dashboard.jsonnet';

{
    newTemplateVariablesList(filterSelector='', enableK8sCluster=true, includeInstance=true, setenceCaseLabels=false, clusterLabel='cluster'):: (     

        local clusterTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components, %(clusterLabel)s)
            ||| % { clusterLabel: clusterLabel }
            else
            |||
                label_values(alloy_component_controller_running_components{%(filterSelector)s}, %(clusterLabel)s)
            ||| % { filterSelector: filterSelector, clusterLabel: clusterLabel };

        local namespaceTemplateQuery =
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{%(clusterLabel)s=~"$cluster"}, namespace)
            ||| % { clusterLabel: clusterLabel }
            else
            |||
                label_values(alloy_component_controller_running_components{%(filterSelector)s, %(clusterLabel)s=~"$cluster"}, namespace)
            ||| % { filterSelector: filterSelector, clusterLabel: clusterLabel };
        
        local k8sJobTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{%(clusterLabel)s=~"$cluster", namespace=~"$namespace"}, job)
            ||| % { clusterLabel: clusterLabel }
            else
            |||
                label_values(alloy_component_controller_running_components{%(filterSelector)s, %(clusterLabel)s=~"$cluster", namespace=~"$namespace"}, job)
            ||| % { filterSelector: filterSelector, clusterLabel: clusterLabel };
        
        local k8sInstanceTemplateQuery = 
            if std.isEmpty(filterSelector) then
            |||
                label_values(alloy_component_controller_running_components{%(clusterLabel)s=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)
            ||| % { clusterLabel: clusterLabel }
            else
            |||
                label_values(alloy_component_controller_running_components{%(filterSelector)s, %(clusterLabel)s=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)
            ||| % { filterSelector: filterSelector, clusterLabel: clusterLabel };

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

        local filtersTemplateVariable = {
            name: 'filters',
            type: 'adhoc',
            datasource: {
                type: 'prometheus',
                uid: '${datasource}',
            },
        };
        
        if enableK8sCluster then
            (
                [
                    dashboard.newMultiTemplateVariable(
                    name='cluster', 
                    query=clusterTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels,
                    defaultToAll=false),
                    dashboard.newMultiTemplateVariable(
                    name='namespace', 
                    query=namespaceTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels,
                    defaultToAll=false),
                    dashboard.newMultiTemplateVariable(
                    name='job', 
                    query=k8sJobTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels),
                ] + 
                if includeInstance then
                    [   
                        dashboard.newMultiTemplateVariable(
                        name='instance', 
                        query=k8sInstanceTemplateQuery,
                        setenceCaseLabels=setenceCaseLabels) 
                    ]
                else []
            ) + [filtersTemplateVariable]
        else
            (
                [
                    dashboard.newMultiTemplateVariable(
                    name='job', 
                    query=jobTemplateQuery,
                    setenceCaseLabels=setenceCaseLabels),                
                ] + 
                if includeInstance then
                    [
                        dashboard.newMultiTemplateVariable(
                        name='instance', 
                        query=instanceTemplateQuery,
                        setenceCaseLabels=setenceCaseLabels)
                    ]
                else []
            ) + [filtersTemplateVariable]
    )
}