package snmp_exporter

import (
	"testing"

	snmp_config "github.com/prometheus/snmp_exporter/config"
	"github.com/stretchr/testify/require"
)

const embeddedModulesCount = 49
const embeddedAuthCount = 2

// TestLoadSNMPConfig tests the LoadSNMPConfig function covers all the cases.
func TestLoadSNMPConfig(t *testing.T) {
	tests := []struct {
		name               string
		cfg                Config
		expectedNumModules int
		expectedNumAuths   int
	}{
		{
			name: "passing a config file",
			cfg: Config{
				SnmpConfigFile:          "common/snmp.yml",
				SnmpConfigMergeStrategy: "replace",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount,
			expectedNumAuths:   embeddedAuthCount,
		},
		{
			name: "passing a snmp config",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Modules: map[string]*snmp_config.Module{"if_mib": {Walk: []string{"1.3.6.1.2.1.2"}}}},
				SnmpConfigMergeStrategy: "replace",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}},
			},
			expectedNumModules: 1,
			expectedNumAuths:   0,
		},
		{
			name: "using embedded config",
			cfg: Config{
				SnmpConfigMergeStrategy: "replace",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount,
			expectedNumAuths:   embeddedAuthCount,
		},
		{
			name: "merging embedded config and custom",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Modules: map[string]*snmp_config.Module{"if_mib_custom": {Walk: []string{"1.3.6.1.2.1.2"}}}},
				SnmpConfigMergeStrategy: "merge",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount + 1,
			expectedNumAuths:   embeddedAuthCount,
		},
		{
			name: "merging embedded config and custom (overriding existing if_mib module)",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Modules: map[string]*snmp_config.Module{"if_mib": {Walk: []string{"1.3.6.1.2.1.2"}}}},
				SnmpConfigMergeStrategy: "merge",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount,
			expectedNumAuths:   embeddedAuthCount,
		},
		{
			name: "merging embedded config and custom (add auth)",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Auths: map[string]*snmp_config.Auth{"private": {Community: "private", Version: 2}}},
				SnmpConfigMergeStrategy: "merge",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount,
			expectedNumAuths:   embeddedAuthCount + 1,
		},
		{
			name: "merging embedded config and custom (override auth)",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Auths: map[string]*snmp_config.Auth{"public_v2": {Community: "private", Version: 2}}},
				SnmpConfigMergeStrategy: "merge",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount,
			expectedNumAuths:   embeddedAuthCount,
		},
		{
			name: "replacing embedded config with custom (add module)",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Modules: map[string]*snmp_config.Module{"if_mib": {Walk: []string{"1.3.6.1.2.1.2"}}}},
				SnmpConfigMergeStrategy: "replace",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: 1,
			expectedNumAuths:   0,
		},
		{
			name: "replacing embedded config with custom (add auth)",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{Auths: map[string]*snmp_config.Auth{"public_v2": {Community: "private", Version: 2}}},
				SnmpConfigMergeStrategy: "replace",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: 0,
			expectedNumAuths:   1,
		},
		{
			name: "merging embedded config and custom (empty config)",
			cfg: Config{
				SnmpConfig:              snmp_config.Config{},
				SnmpConfigMergeStrategy: "merge",
				SnmpTargets:             []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: embeddedModulesCount,
			expectedNumAuths:   embeddedAuthCount,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadSNMPConfig(tt.cfg.SnmpConfigFile, &tt.cfg.SnmpConfig, tt.cfg.SnmpConfigMergeStrategy)
			require.NoError(t, err)

			require.Equal(t, tt.expectedNumModules, len(cfg.Modules))
			require.Equal(t, tt.expectedNumAuths, len(cfg.Auths))
		})
	}
}
