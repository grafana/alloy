package java

import (
	"fmt"

	"github.com/grafana/agent/internal/component/discovery"
)

const (
	labelServiceName    = "service_name"
	labelServiceNameK8s = "__meta_kubernetes_pod_annotation_pyroscope_io_service_name"
)

func inferServiceName(target discovery.Target) string {
	k8sServiceName := target[labelServiceNameK8s]
	if k8sServiceName != "" {
		return k8sServiceName
	}
	k8sNamespace := target["__meta_kubernetes_namespace"]
	k8sContainer := target["__meta_kubernetes_pod_container_name"]
	if k8sNamespace != "" && k8sContainer != "" {
		return fmt.Sprintf("java/%s/%s", k8sNamespace, k8sContainer)
	}
	dockerContainer := target["__meta_docker_container_name"]
	if dockerContainer != "" {
		return dockerContainer
	}
	if swarmService := target["__meta_dockerswarm_container_label_service_name"]; swarmService != "" {
		return swarmService
	}
	if swarmService := target["__meta_dockerswarm_service_name"]; swarmService != "" {
		return swarmService
	}
	return "unspecified"
}
