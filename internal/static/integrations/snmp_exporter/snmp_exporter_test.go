package snmp_exporter

import (
	"testing"

	snmp_config "github.com/prometheus/snmp_exporter/config"
	"github.com/stretchr/testify/require"
)

// TestLoadSNMPConfig tests the LoadSNMPConfig function covers all the cases.
func TestLoadSNMPConfig(t *testing.T) {
	tests := []struct {
		name               string
		cfg                Config
		expectedNumModules int
	}{
		{
			name:               "passing a config file",
			cfg:                Config{SnmpConfigFile: "common/snmp.yml", SnmpTargets: []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: 22,
		},
		{
			name: "passing a snmp config",
			cfg: Config{
				SnmpConfig:  snmp_config.Config{Modules: map[string]*snmp_config.Module{"if_mib": {Walk: []string{"1.3.6.1.2.1.2"}}}},
				SnmpTargets: []SNMPTarget{{Name: "test", Target: "localhost"}},
			},
			expectedNumModules: 1,
		},
		{
			name:               "using embedded config",
			cfg:                Config{SnmpTargets: []SNMPTarget{{Name: "test", Target: "localhost"}}},
			expectedNumModules: 22,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadSNMPConfig(tt.cfg.SnmpConfigFile, &tt.cfg.SnmpConfig)
			require.NoError(t, err)

			require.Equal(t, tt.expectedNumModules, len(cfg.Modules))
		})
	}
}
