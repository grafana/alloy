package smartctl_exporter

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	config_integrations "github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config with defaults",
			cfg:     &Config{},
			wantErr: false,
		},
		{
			name: "valid config with explicit devices",
			cfg: &Config{
				Devices: []string{"/dev/sda", "/dev/nvme0n1"},
			},
			wantErr: false,
		},
		{
			name: "valid config with device exclude filter",
			cfg: &Config{
				DeviceExclude: "^(ram|loop)\\d+$",
			},
			wantErr: false,
		},
		{
			name: "valid config with device include filter",
			cfg: &Config{
				DeviceInclude: "^(sd|nvme)",
			},
			wantErr: false,
		},
		{
			name: "valid config with scan types",
			cfg: &Config{
				ScanDeviceTypes: []string{"sat", "nvme"},
			},
			wantErr: false,
		},
		{
			name: "valid config with powermode check",
			cfg: &Config{
				PowermodeCheck: "standby",
			},
			wantErr: false,
		},
		{
			name: "invalid config - both include and exclude",
			cfg: &Config{
				DeviceInclude: "^sd",
				DeviceExclude: "^loop",
			},
			wantErr: true,
			errMsg:  "device_exclude and device_include are mutually exclusive",
		},
		{
			name: "invalid config - invalid powermode",
			cfg: &Config{
				PowermodeCheck: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid powermode_check: invalid",
		},
		{
			name: "invalid config - invalid device exclude regex",
			cfg: &Config{
				DeviceExclude: "[invalid",
			},
			wantErr: true,
			errMsg:  "invalid device_exclude regex",
		},
		{
			name: "invalid config - invalid device include regex",
			cfg: &Config{
				DeviceInclude: "[invalid",
			},
			wantErr: true,
			errMsg:  "invalid device_include regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNopLogger()
			_, err := New(logger, tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Name(t *testing.T) {
	cfg := &Config{}
	require.Equal(t, "smartctl_exporter", cfg.Name())
}

func TestConfig_InstanceKey(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		defaultKey string
		want       string
	}{
		{
			name:       "always returns localhost for local monitoring",
			cfg:        &Config{},
			defaultKey: "default",
			want:       "localhost",
		},
		{
			name: "returns localhost even with specific devices",
			cfg: &Config{
				Devices: []string{"/dev/sda"},
			},
			defaultKey: "default",
			want:       "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.InstanceKey(tt.defaultKey)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_UnmarshalYAML(t *testing.T) {
	// Test that UnmarshalYAML applies defaults
	cfg := &Config{}

	// After unmarshaling with no values, should have defaults
	err := cfg.UnmarshalYAML(func(interface{}) error { return nil })
	require.NoError(t, err)

	require.Equal(t, DefaultConfig.SmartctlPath, cfg.SmartctlPath)
	require.Equal(t, DefaultConfig.ScanInterval, cfg.ScanInterval)
	require.Equal(t, DefaultConfig.RescanInterval, cfg.RescanInterval)
	require.Equal(t, DefaultConfig.PowermodeCheck, cfg.PowermodeCheck)
}

func TestIntegration_ScrapeConfigs(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		wantJobs  int
		checkFunc func(t *testing.T, configs []config_integrations.ScrapeConfig)
	}{
		{
			name:     "default config creates single job",
			cfg:      &Config{},
			wantJobs: 1,
			checkFunc: func(t *testing.T, configs []config_integrations.ScrapeConfig) {
				require.Equal(t, "smartctl_exporter", configs[0].JobName)
				require.Equal(t, "/metrics", configs[0].MetricsPath)
			},
		},
		{
			name: "config with specific devices creates single job",
			cfg: &Config{
				Devices: []string{"/dev/sda", "/dev/nvme0n1"},
			},
			wantJobs: 1,
			checkFunc: func(t *testing.T, configs []config_integrations.ScrapeConfig) {
				require.Equal(t, "smartctl_exporter", configs[0].JobName)
				require.Equal(t, "/metrics", configs[0].MetricsPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNopLogger()
			integration, err := New(logger, tt.cfg)
			require.NoError(t, err)

			configs := integration.ScrapeConfigs()
			require.Len(t, configs, tt.wantJobs)

			if tt.checkFunc != nil {
				tt.checkFunc(t, configs)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	require.Equal(t, "/usr/sbin/smartctl", DefaultConfig.SmartctlPath)
	require.Equal(t, 60*time.Second, DefaultConfig.ScanInterval)
	require.Equal(t, 10*time.Minute, DefaultConfig.RescanInterval)
	require.Equal(t, "standby", DefaultConfig.PowermodeCheck)
}
