package kubetail

import corev1 "k8s.io/api/core/v1"

type containerType uint8

const (
	containerTypeNone containerType = iota
	containerTypeApp
	containerTypeInit
	containerTypeEphemeral
)

func findContainerStatus(pod *corev1.Pod, containerName string) (status corev1.ContainerStatus, typ containerType, ok bool) {
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == containerName {
			return container, containerTypeApp, true
		}
	}
	for _, container := range pod.Status.InitContainerStatuses {
		if container.Name == containerName {
			return container, containerTypeInit, true
		}
	}
	for _, container := range pod.Status.EphemeralContainerStatuses {
		if container.Name == containerName {
			return container, containerTypeEphemeral, true
		}
	}
	return corev1.ContainerStatus{}, containerTypeNone, false
}
