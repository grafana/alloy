package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/snmp"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/static/integrations/snmp_exporter"
	snmp_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/snmp_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	snmp_config "github.com/prometheus/snmp_exporter/config"
)

func (b *ConfigBuilder) appendSnmpExporter(config *snmp_exporter.Config) discovery.Exports {
	args := toSnmpExporter(config)
	return b.appendExporterBlock(args, config.Name(), nil, "snmp")
}

func toSnmpExporter(config *snmp_exporter.Config) *snmp.Arguments {
	var targets snmp.TargetsList
	for _, t := range config.SnmpTargets {
		target := make(map[string]string)
		target["name"] = t.Name
		target["address"] = t.Target
		target["module"] = t.Module
		target["auth"] = t.Auth
		target["walk_params"] = t.WalkParams
		target["snmp_context"] = t.SNMPContext
		targets = append(targets, target)
	}

	walkParams := make([]snmp.WalkParam, len(config.WalkParams))
	index := 0
	for name, p := range config.WalkParams {
		retries := 0
		if p.Retries != nil {
			retries = *p.Retries
		}

		walkParams[index] = snmp.WalkParam{
			Name:                    common.SanitizeIdentifierPanics(name),
			MaxRepetitions:          p.MaxRepetitions,
			Retries:                 retries,
			Timeout:                 p.Timeout,
			UseUnconnectedUDPSocket: p.UseUnconnectedUDPSocket,
		}
		index++
	}

	return &snmp.Arguments{
		ConfigFile:  config.SnmpConfigFile,
		Config:      alloytypes.OptionalSecret{},
		TargetsList: targets,
		WalkParams:  walkParams,
		ConfigStruct: snmp_config.Config{
			Auths:   config.SnmpConfig.Auths,
			Modules: config.SnmpConfig.Modules,
			Version: config.SnmpConfig.Version,
		},
	}
}

func (b *ConfigBuilder) appendSnmpExporterV2(config *snmp_exporter_v2.Config) discovery.Exports {
	args := toSnmpExporterV2(config)
	return b.appendExporterBlock(args, config.Name(), config.Common.InstanceKey, "snmp")
}

func toSnmpExporterV2(config *snmp_exporter_v2.Config) *snmp.Arguments {
	var targets snmp.TargetsList
	for _, t := range config.SnmpTargets {
		target := make(map[string]string)
		target["name"] = t.Name
		target["address"] = t.Target
		target["module"] = t.Module
		target["auth"] = t.Auth
		target["walk_params"] = t.WalkParams
		target["snmp_context"] = t.SNMPContext
		targets = append(targets, target)
	}

	walkParams := make([]snmp.WalkParam, len(config.WalkParams))
	index := 0
	for name, p := range config.WalkParams {
		retries := 0
		if p.Retries != nil {
			retries = *p.Retries
		}

		walkParams[index] = snmp.WalkParam{
			Name:                    common.SanitizeIdentifierPanics(name),
			MaxRepetitions:          p.MaxRepetitions,
			Retries:                 retries,
			Timeout:                 p.Timeout,
			UseUnconnectedUDPSocket: p.UseUnconnectedUDPSocket,
		}
		index++
	}

	return &snmp.Arguments{
		ConfigFile:  config.SnmpConfigFile,
		Config:      alloytypes.OptionalSecret{},
		TargetsList: targets,
		WalkParams:  walkParams,
		ConfigStruct: snmp_config.Config{
			Auths:   config.SnmpConfig.Auths,
			Modules: config.SnmpConfig.Modules,
			Version: config.SnmpConfig.Version,
		},
	}
}
