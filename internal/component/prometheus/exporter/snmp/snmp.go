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

	a := args.(Arguments)
	for _, tgt := range a.Targets {
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
		if tgt.Auth != "" {
			target["__param_auth"] = tgt.Auth
		}

		targets = append(targets, target)
	}

	return targets
}

// SNMPTarget defines a target to be used by the exporter.
type SNMPTarget struct {
	Name       string `alloy:",label"`
	Target     string `alloy:"address,attr"`
	Module     string `alloy:"module,attr,optional"`
	Auth       string `alloy:"auth,attr,optional"`
	WalkParams string `alloy:"walk_params,attr,optional"`
}

type TargetBlock []SNMPTarget

// Convert converts the component's TargetBlock to a slice of integration's SNMPTarget.
func (t TargetBlock) Convert() []snmp_exporter.SNMPTarget {
	targets := make([]snmp_exporter.SNMPTarget, 0, len(t))
	for _, target := range t {
		targets = append(targets, snmp_exporter.SNMPTarget{
			Name:       target.Name,
			Target:     target.Target,
			Module:     target.Module,
			Auth:       target.Auth,
			WalkParams: target.WalkParams,
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
	ConfigFiles  []string                  `alloy:"config_files,attr,optional"`
	Config       alloytypes.OptionalSecret `alloy:"config,attr,optional"`
	Targets      TargetBlock               `alloy:"target,block"`
	WalkParams   WalkParams                `alloy:"walk_param,block,optional"`
	ConfigStruct snmp_config.Config
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

	err := yaml.UnmarshalStrict([]byte(a.Config.Value), &a.ConfigStruct)
	if err != nil {
		return fmt.Errorf("invalid snmp_exporter config: %s", err)
	}

	return nil
}

// Convert converts the component's Arguments to the integration's Config.
func (a *Arguments) Convert() *snmp_exporter.Config {
	return &snmp_exporter.Config{
		SnmpConfigFile:  a.ConfigFile,
		SnmpConfigFiles: a.ConfigFiles,
		SnmpTargets:     a.Targets.Convert(),
		WalkParams:      a.WalkParams.Convert(),
		SnmpConfig:      a.ConfigStruct,
	}
}
