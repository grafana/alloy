// +k8s:deepcopy-gen=package
// +groupName=monitoring.grafana.com

//go:generate controller-gen object paths=.
//go:generate controller-gen crd:crdVersions=v1 paths=. output:crd:dir=../../../../../../../../operations/helm/charts/grafana-agent/crds

package v1alpha2
