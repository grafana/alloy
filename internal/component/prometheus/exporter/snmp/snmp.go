package snmp

import (
	"errors"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/snmp_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	snmp_config "github.com/prometheus/snmp_exporter/config"
	"gopkg.in/yaml.v2"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.snmp",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.NewWithTargetBuilder(createExporter, "snmp", buildSNMPTargets),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// buildSNMPTargets creates the exporter's discovery targets based on the defined SNMP targets.
func buildSNMPTargets(baseTarget discovery.Target, args component.Arguments) []discovery.Target {
	var targets []discovery.Target

	snmpTargets := args.(Arguments).Targets
	if len(snmpTargets) == 0 {
		// Converting to SNMPTarget to avoid duplicating logic
		snmpTargets = args.(Arguments).TargetsList.convert()
	}

	for _, tgt := range snmpTargets {
		target := make(discovery.Target)
		for k, v := range baseTarget {
			target[k] = v
		}

		target["job"] = target["job"] + "/" + tgt.Name
		target["__param_target"] = tgt.Target
		if tgt.Module != "" {
			target["__param_module"] = tgt.Module
		}
		if tgt.WalkParams != "" {
			target["__param_walk_params"] = tgt.WalkParams
		}
		if tgt.SNMPContext != "" {
			target["__param_snmp_context"] = tgt.SNMPContext
		}
		if tgt.Auth != "" {
			target["__param_auth"] = tgt.Auth
		}

		targets = append(targets, target)
	}

	return targets
}

// SNMPTarget defines a target to be used by the exporter.
type SNMPTarget struct {
	Name        string `alloy:",label"`
	Target      string `alloy:"address,attr"`
	Module      string `alloy:"module,attr,optional"`
	Auth        string `alloy:"auth,attr,optional"`
	WalkParams  string `alloy:"walk_params,attr,optional"`
	SNMPContext string `alloy:"snmp_context,attr,optional"`
}

type TargetBlock []SNMPTarget

// Convert converts the component's TargetBlock to a slice of integration's SNMPTarget.
func (t TargetBlock) Convert() []snmp_exporter.SNMPTarget {
	targets := make([]snmp_exporter.SNMPTarget, 0, len(t))
	for _, target := range t {
		targets = append(targets, snmp_exporter.SNMPTarget{
			Name:        target.Name,
			Target:      target.Target,
			Module:      target.Module,
			Auth:        target.Auth,
			WalkParams:  target.WalkParams,
			SNMPContext: target.SNMPContext,
		})
	}
	return targets
}

type WalkParam struct {
	Name                    string        `alloy:",label"`
	MaxRepetitions          uint32        `alloy:"max_repetitions,attr,optional"`
	Retries                 int           `alloy:"retries,attr,optional"`
	Timeout                 time.Duration `alloy:"timeout,attr,optional"`
	UseUnconnectedUDPSocket bool          `alloy:"use_unconnected_udp_socket,attr,optional"`
}

type WalkParams []WalkParam

// Convert converts the component's WalkParams to the integration's WalkParams.
func (w WalkParams) Convert() map[string]snmp_config.WalkParams {
	walkParams := make(map[string]snmp_config.WalkParams)
	for _, walkParam := range w {
		walkParams[walkParam.Name] = snmp_config.WalkParams{
			MaxRepetitions:          walkParam.MaxRepetitions,
			Retries:                 &walkParam.Retries,
			Timeout:                 walkParam.Timeout,
			UseUnconnectedUDPSocket: walkParam.UseUnconnectedUDPSocket,
		}
	}
	return walkParams
}

type Arguments struct {
	ConfigFile   string                    `alloy:"config_file,attr,optional"`
	Config       alloytypes.OptionalSecret `alloy:"config,attr,optional"`
	Targets      TargetBlock               `alloy:"target,block,optional"`
	WalkParams   WalkParams                `alloy:"walk_param,block,optional"`
	ConfigStruct snmp_config.Config

	// New way of passing targets. This allows the component to receive targets from other components.
	TargetsList TargetsList `alloy:"targets,attr,optional"`
}

type TargetsList []map[string]string

func (t TargetsList) Convert() []snmp_exporter.SNMPTarget {
	targets := make([]snmp_exporter.SNMPTarget, 0, len(t))
	for _, target := range t {
		address, _ := getAddress(target)
		targets = append(targets, snmp_exporter.SNMPTarget{
			Name:        target["name"],
			Target:      address,
			Module:      target["module"],
			Auth:        target["auth"],
			WalkParams:  target["walk_params"],
			SNMPContext: target["snmp_context"],
		})
	}
	return targets
}

func (t TargetsList) convert() []SNMPTarget {
	targets := make([]SNMPTarget, 0, len(t))
	for _, target := range t {
		address, _ := getAddress(target)
		targets = append(targets, SNMPTarget{
			Name:        target["name"],
			Target:      address,
			Module:      target["module"],
			Auth:        target["auth"],
			WalkParams:  target["walk_params"],
			SNMPContext: target["snmp_context"],
		})
	}
	return targets
}

// UnmarshalAlloy implements Alloy unmarshalling for Arguments.
func (a *Arguments) UnmarshalAlloy(f func(interface{}) error) error {
	type args Arguments
	if err := f((*args)(a)); err != nil {
		return err
	}

	if a.ConfigFile != "" && a.Config.Value != "" {
		return errors.New("config and config_file are mutually exclusive")
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

	err := yaml.UnmarshalStrict([]byte(a.Config.Value), &a.ConfigStruct)
	if err != nil {
		return fmt.Errorf("invalid snmp_exporter config: %s", err)
	}

	return nil
}

// Convert converts the component's Arguments to the integration's Config.
func (a *Arguments) Convert() *snmp_exporter.Config {
	var targets []snmp_exporter.SNMPTarget
	if len(a.Targets) != 0 {
		targets = a.Targets.Convert()
	} else {
		targets = a.TargetsList.Convert()
	}
	return &snmp_exporter.Config{
		SnmpConfigFile: a.ConfigFile,
		SnmpTargets:    targets,
		WalkParams:     a.WalkParams.Convert(),
		SnmpConfig:     a.ConfigStruct,
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
