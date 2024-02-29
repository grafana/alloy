package clientutil

import (
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// MergePatchContainers adds patches to base using a strategic merge patch and
// iterating by container name, failing on the first error.
//
// Copied from github.com/prometheus-operator/prometheus-operator/pkg/k8sutil.
func MergePatchContainers(base, patches []v1.Container) ([]v1.Container, error) {
	var out []v1.Container

	// map of containers that still need to be patched by name
	containersToPatch := make(map[string]v1.Container)
	for _, c := range patches {
		containersToPatch[c.Name] = c
	}

	for _, container := range base {
		// If we have a patch result, iterate over each container and try and calculate the patch
		if patchContainer, ok := containersToPatch[container.Name]; ok {
			// Get the json for the container and the patch
			containerBytes, err := json.Marshal(container)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal json for container %s: %w", container.Name, err)
			}
			patchBytes, err := json.Marshal(patchContainer)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal json for patch container %s: %w", container.Name, err)
			}

			// Calculate the patch result
			jsonResult, err := strategicpatch.StrategicMergePatch(containerBytes, patchBytes, v1.Container{})
			if err != nil {
				return nil, fmt.Errorf("failed to generate merge patch for %s: %w", container.Name, err)
			}
			var patchResult v1.Container
			if err := json.Unmarshal(jsonResult, &patchResult); err != nil {
				return nil, fmt.Errorf("failed to unmarshal merged container %s: %w", container.Name, err)
			}

			// Add the patch result and remove the corresponding key from the to do list
			out = append(out, patchResult)
			delete(containersToPatch, container.Name)
		} else {
			// This container didn't need to be patched
			out = append(out, container)
		}
	}

	// Iterate over the patches and add all the containers that were not previously part of a patch result
	for _, container := range patches {
		if _, ok := containersToPatch[container.Name]; ok {
			out = append(out, container)
		}
	}

	return out, nil
}
