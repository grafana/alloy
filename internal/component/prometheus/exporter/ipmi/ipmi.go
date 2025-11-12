package ipmi

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.ipmi",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.NewWithTargetBuilder(createExporter, "ipmi", buildIPMITargets),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// buildIPMITargets creates the exporter's discovery targets based on the defined IPMI targets.
func buildIPMITargets(baseTarget discovery.Target, args component.Arguments) []discovery.Target {
	var targets []discovery.Target

	ipmiArgs := args.(Arguments)

	// Handle local IPMI target if enabled
	if ipmiArgs.Local.Enabled {
		target := make(map[string]string, baseTarget.Len()+2)
		baseTarget.ForEachLabel(func(key string, value string) bool {
			target[key] = value
			return true
		})

		target["job"] = target["job"] + "/local"
		if ipmiArgs.Local.Module != "" {
			target["__param_module"] = ipmiArgs.Local.Module
		}

		targets = append(targets, discovery.NewTargetFromMap(target))
	}

	// Handle remote IPMI targets
	for _, tgt := range ipmiArgs.Targets {
		target := make(map[string]string, baseTarget.Len()+6)
		baseTarget.ForEachLabel(func(key string, value string) bool {
			target[key] = value
			return true
		})

		target["job"] = target["job"] + "/" + tgt.Name
		target["__param_target"] = tgt.Target
		if tgt.Module != "" {
			target["__param_module"] = tgt.Module
		}

		targets = append(targets, discovery.NewTargetFromMap(target))
	}

	return targets
}
