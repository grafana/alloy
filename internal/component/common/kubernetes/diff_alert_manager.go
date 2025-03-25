package kubernetes

import (
	"bytes"

	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"gopkg.in/yaml.v3"
)

type AlertManagerConfigDiffKind string

const (
	AlertManagerConfigDiffKindAdd    AlertManagerConfigDiffKind = "add"
	AlertManagerConfigDiffKindRemove AlertManagerConfigDiffKind = "remove"
	AlertManagerConfigDiffKindUpdate AlertManagerConfigDiffKind = "update"
)

type AlertManagerConfigDiff struct {
	Kind    AlertManagerConfigDiffKind
	Actual  alertmgr_cfg.Config
	Desired alertmgr_cfg.Config
}

// TODO: Can we have more than one such config per namespace?
type AlertManagerConfigsByNamespace map[string][]alertmgr_cfg.Config
type AlertManagerConfigDiffsByNamespace map[string][]AlertManagerConfigDiff

// type AlertManagerConfigsByNamespace map[string]alertmgr_cfg.Config
// type AlertManagerConfigDiffsByNamespace map[string]AlertManagerConfigDiff

func DiffAlertManagerConfigs(desired, actual AlertManagerConfigsByNamespace) AlertManagerConfigDiffsByNamespace {
	seenNamespaces := map[string]bool{}

	diff := make(AlertManagerConfigDiffsByNamespace)

	for namespace, desiredAlertManagerConfigs := range desired {
		seenNamespaces[namespace] = true

		actualAlertManagerConfigs := actual[namespace]
		subDiff := diffAlertManagerConfigsNamespaceState(desiredAlertManagerConfigs, actualAlertManagerConfigs)

		if len(subDiff) == 0 {
			continue
		}

		diff[namespace] = subDiff
	}

	for namespace, actualAlertManagerConfigs := range actual {
		if seenNamespaces[namespace] {
			continue
		}

		subDiff := diffAlertManagerConfigsNamespaceState(nil, actualAlertManagerConfigs)

		diff[namespace] = subDiff
	}

	return diff
}

func diffAlertManagerConfigsNamespaceState(desired []alertmgr_cfg.Config, actual []alertmgr_cfg.Config) []AlertManagerConfigDiff {
	var diff []AlertManagerConfigDiff

	seenGroups := map[string]bool{}

desiredGroups:
	for _, desiredAlertManagerConfig := range desired {
		//TODO: Use a hash instead of the whole config string?
		desiredConfigId := desiredAlertManagerConfig.String()
		seenGroups[desiredConfigId] = true

		for _, actualAlertManagerConfig := range actual {
			//TODO: Use a hash instead of the whole config string?
			actualConfigId := actualAlertManagerConfig.String()
			if desiredConfigId == actualConfigId {
				if equalAlertManagerConfigs(desiredAlertManagerConfig, actualAlertManagerConfig) {
					continue desiredGroups
				}

				diff = append(diff, AlertManagerConfigDiff{
					Kind:    AlertManagerConfigDiffKindUpdate,
					Actual:  actualAlertManagerConfig,
					Desired: desiredAlertManagerConfig,
				})
				continue desiredGroups
			}
		}

		diff = append(diff, AlertManagerConfigDiff{
			Kind:    AlertManagerConfigDiffKindAdd,
			Desired: desiredAlertManagerConfig,
		})
	}

	for _, actualAlertManagerConfig := range actual {
		//TODO: Use a hash instead of the whole config string?
		actualConfigId := actualAlertManagerConfig.String()
		if seenGroups[actualConfigId] {
			continue
		}

		diff = append(diff, AlertManagerConfigDiff{
			Kind:   AlertManagerConfigDiffKindRemove,
			Actual: actualAlertManagerConfig,
		})
	}

	return diff
}

func equalAlertManagerConfigs(a, b alertmgr_cfg.Config) bool {
	aBuf, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bBuf, err := yaml.Marshal(b)
	if err != nil {
		return false
	}

	return bytes.Equal(aBuf, bBuf)
}
