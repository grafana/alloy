package java

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/discovery"
)

const (
	labelServiceName    = "service_name"
	labelServiceNameK8s = "__meta_kubernetes_pod_annotation_pyroscope_io_service_name"
)

func inferServiceName(target discovery.Target) string {
	if k8sServiceName, ok := target.Get(labelServiceNameK8s); ok {
		return k8sServiceName
	}
	k8sNamespace, nsOk := target.Get("__meta_kubernetes_namespace")
	k8sContainer, contOk := target.Get("__meta_kubernetes_pod_container_name")
	if nsOk && contOk {
		return fmt.Sprintf("java/%s/%s", k8sNamespace, k8sContainer)
	}

	if dockerContainer, ok := target.Get("__meta_docker_container_name"); ok {
		return dockerContainer
	}
	if swarmService, ok := target.Get("__meta_dockerswarm_container_label_service_name"); ok {
		return swarmService
	}
	if swarmService, ok := target.Get("__meta_dockerswarm_service_name"); ok {
		return swarmService
	}
	return "unspecified"
}
