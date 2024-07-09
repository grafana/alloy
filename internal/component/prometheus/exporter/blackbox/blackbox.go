package blackbox

import (
	"errors"
	"fmt"
	"time"

	blackbox_config "github.com/prometheus/blackbox_exporter/config"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.blackbox",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.NewWithTargetBuilder(createExporter, "blackbox", buildBlackboxTargets),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// buildBlackboxTargets creates the exporter's discovery targets based on the defined blackbox targets.
func buildBlackboxTargets(baseTarget discovery.Target, args component.Arguments) []discovery.Target {
	var targets []discovery.Target

	blackboxTargets := args.(Arguments).Targets
	if len(blackboxTargets) == 0 {
		// Converting to BlackboxTarget to avoid duplicating logic
		blackboxTargets = args.(Arguments).TargetsList.convertInternal()
	}

	for _, tgt := range blackboxTargets {
		target := make(discovery.Target)
		// Set extra labels first, meaning that any other labels will override
		for k, v := range tgt.Labels {
			target[k] = v
		}
		for k, v := range baseTarget {
			target[k] = v
		}

		target["job"] = target["job"] + "/" + tgt.Name
		target["__param_target"] = tgt.Target
		if tgt.Module != "" {
			target["__param_module"] = tgt.Module
		}

		targets = append(targets, target)
	}

	return targets
}

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from Alloy.
var DefaultArguments = Arguments{
	ProbeTimeoutOffset: 500 * time.Millisecond,
}

// BlackboxTarget defines a target to be used by the exporter.
type BlackboxTarget struct {
	Name   string            `alloy:"name,attr"`
	Target string            `alloy:"address,attr"`
	Module string            `alloy:"module,attr,optional"`
	Labels map[string]string `alloy:"labels,attr,optional"`
}

type TargetBlock []BlackboxTarget

// Convert converts the component's TargetBlock to a slice of integration's BlackboxTarget.
func (t TargetBlock) Convert() []blackbox_exporter.BlackboxTarget {
	targets := make([]blackbox_exporter.BlackboxTarget, 0, len(t))
	for _, target := range t {
		targets = append(targets, blackbox_exporter.BlackboxTarget{
			Name:   target.Name,
			Target: target.Target,
			Module: target.Module,
		})
	}
	return targets
}

type Arguments struct {
	ConfigFile         string                    `alloy:"config_file,attr,optional"`
	Config             alloytypes.OptionalSecret `alloy:"config,attr,optional"`
	Targets            TargetBlock               `alloy:"target,block,optional"`
	ProbeTimeoutOffset time.Duration             `alloy:"probe_timeout_offset,attr,optional"`

	// New way of passing targets. This allows the component to receive targets from other components.
	TargetsList TargetsList `alloy:"targets,attr,optional"`
}

type TargetsList []map[string]string

func (t TargetsList) Convert() []blackbox_exporter.BlackboxTarget {
	targets := make([]blackbox_exporter.BlackboxTarget, 0, len(t))
	for _, target := range t {
		address, _ := getAddress(target)
		targets = append(targets, blackbox_exporter.BlackboxTarget{
			Name:   target["name"],
			Target: address,
			Module: target["module"],
		})
	}
	return targets
}

func (t TargetsList) convertInternal() []BlackboxTarget {
	targets := make([]BlackboxTarget, 0, len(t))
	for _, target := range t {

		// extract the extra labels
		labels := make(map[string]string)
		for key, value := range target {
			if key != "name" && key != "__address__" && key != "address" && key != "module" {
				labels[key] = value
			}
		}

		address, _ := getAddress(target)
		targets = append(targets, BlackboxTarget{
			Name:   target["name"],
			Target: address,
			Module: target["module"],
			Labels: labels,
		})
	}
	return targets
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.ConfigFile != "" && a.Config.Value != "" {
		return errors.New("config and config_file are mutually exclusive")
	}

	if a.ConfigFile == "" && a.Config.Value == "" {
		return errors.New("config or config_file must be set")
	}

	var blackboxConfig blackbox_config.Config
	err := yaml.UnmarshalStrict([]byte(a.Config.Value), &blackboxConfig)
	if err != nil {
		return fmt.Errorf("invalid blackbox_exporter config: %s", err)
	}

	if len(a.Targets) != 0 && len(a.TargetsList) != 0 {
		return fmt.Errorf("the block `target` and the attribute `targets` are mutually exclusive")
	}
	for _, target := range a.TargetsList {
		if _, hasName := target["name"]; !hasName {
			return fmt.Errorf("all targets must have a `name`")
		}
		if _, hasAddress := getAddress(target); !hasAddress {
			return fmt.Errorf("all targets must have an `address` or an `__address__` label")
		}
	}
	return nil
}

// Convert converts the component's Arguments to the integration's Config.
func (a *Arguments) Convert() *blackbox_exporter.Config {
	var targets []blackbox_exporter.BlackboxTarget
	if len(a.Targets) != 0 {
		targets = a.Targets.Convert()
	} else {
		targets = a.TargetsList.Convert()
	}
	return &blackbox_exporter.Config{
		BlackboxConfigFile: a.ConfigFile,
		BlackboxConfig:     util.RawYAML(a.Config.Value),
		BlackboxTargets:    targets,
		ProbeTimeoutOffset: a.ProbeTimeoutOffset.Seconds(),
	}
}

func getAddress(data map[string]string) (string, bool) {
	if value, ok := data["address"]; ok {
		return value, true
	}
	if value, ok := data["__address__"]; ok {
		return value, true
	}
	return "", false
}
