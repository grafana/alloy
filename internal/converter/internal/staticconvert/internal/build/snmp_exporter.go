package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/snmp"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/static/integrations/snmp_exporter"
	snmp_exporter_v2 "github.com/grafana/agent/internal/static/integrations/v2/snmp_exporter"
	"github.com/grafana/river/rivertypes"
	snmp_config "github.com/prometheus/snmp_exporter/config"
)

func (b *ConfigBuilder) appendSnmpExporter(config *snmp_exporter.Config) discovery.Exports {
	args := toSnmpExporter(config)
	return b.appendExporterBlock(args, config.Name(), nil, "snmp")
}

func toSnmpExporter(config *snmp_exporter.Config) *snmp.Arguments {
	targets := make([]snmp.SNMPTarget, len(config.SnmpTargets))
	for i, t := range config.SnmpTargets {
		targets[i] = snmp.SNMPTarget{
			Name:       common.SanitizeIdentifierPanics(t.Name),
			Target:     t.Target,
			Module:     t.Module,
			Auth:       t.Auth,
			WalkParams: t.WalkParams,
		}
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
		ConfigFile: config.SnmpConfigFile,
		Config:     rivertypes.OptionalSecret{},
		Targets:    targets,
		WalkParams: walkParams,
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
	targets := make([]snmp.SNMPTarget, len(config.SnmpTargets))
	for i, t := range config.SnmpTargets {
		targets[i] = snmp.SNMPTarget{
			Name:       common.SanitizeIdentifierPanics(t.Name),
			Target:     t.Target,
			Module:     t.Module,
			Auth:       t.Auth,
			WalkParams: t.WalkParams,
		}
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
		ConfigFile: config.SnmpConfigFile,
		Config:     rivertypes.OptionalSecret{},
		Targets:    targets,
		WalkParams: walkParams,
		ConfigStruct: snmp_config.Config{
			Auths:   config.SnmpConfig.Auths,
			Modules: config.SnmpConfig.Modules,
			Version: config.SnmpConfig.Version,
		},
	}
}
