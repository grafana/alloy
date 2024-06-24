local alert = import './utils/alert.jsonnet';

{
  newCustomAlerts(enableK8sCluster=true):
    alert.newGroup(
      'alloy_tpaschalis',
      [
        alert.newRule(
          'TpaschalisRefuseKitsune',
          'alloy_build_info{instance="kitsune"} == 1',
          'Saw a sneaky Alloy instance named kitsune.',
          '5m',
        ),
      ]
    )
}
