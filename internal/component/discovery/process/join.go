//go:build linux

package process

import (
	"github.com/grafana/alloy/internal/component/discovery"
)

func join(processes, containers []discovery.Target) []discovery.Target {
	res := make([]discovery.Target, 0, len(processes)+len(containers))

	cid2container := make(map[string]discovery.Target, len(containers))
	for _, container := range containers {
		cid := getContainerIDFromTarget(container)
		if cid != "" {
			cid2container[cid] = container
		} else {
			res = append(res, container)
		}
	}
	for _, p := range processes {
		cid := getContainerIDFromTarget(p)
		if cid == "" {
			res = append(res, p)
			continue
		}
		container, ok := cid2container[cid]
		if !ok {
			res = append(res, p)
			continue
		}
		mergedBuilder := discovery.NewTargetBuilderFrom(p)
		mergedBuilder.MergeWith(container)
		res = append(res, mergedBuilder.Target())
	}
	for _, target := range cid2container {
		res = append(res, target)
	}
	return res
}
