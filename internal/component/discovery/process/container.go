//go:build linux

package process

import (
	"bufio"
	"io"
	"regexp"
	"strings"

	"github.com/grafana/alloy/internal/component/discovery"
)

var (
	// cgroupContainerIDRe matches a container ID from a /proc/{pid}}/cgroup
	cgroupContainerIDRe = regexp.MustCompile(`^.*/(?:.*-)?([0-9a-f]{64})(?:\.|\s*$)`)
)

func getContainerIDFromCGroup(cgroup io.Reader) string {
	scanner := bufio.NewScanner(cgroup)
	for scanner.Scan() {
		line := scanner.Bytes()
		matches := cgroupContainerIDRe.FindSubmatch(line)
		if len(matches) <= 1 {
			continue
		}
		return string(matches[1])
	}
	return ""
}

var knownContainerIDPrefixes = []string{"docker://", "containerd://", "cri-o://"}

// get container id from __meta_kubernetes_pod_container_id label
func getContainerIDFromK8S(k8sContainerID string) string {
	for _, p := range knownContainerIDPrefixes {
		if after, ok := strings.CutPrefix(k8sContainerID, p); ok {
			return after
		}
	}
	return ""
}

func getContainerIDFromTarget(target discovery.Target) string {
	cid, ok := target.Get(labelProcessContainerID)
	if ok && cid != "" {
		return cid
	}
	cid, ok = target.Get("__meta_kubernetes_pod_container_id")
	if ok && cid != "" {
		return getContainerIDFromK8S(cid)
	}
	cid, ok = target.Get("__meta_docker_container_id")
	if ok && cid != "" {
		return cid
	}
	return ""
}
