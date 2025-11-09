package ipmi

import (
	"testing"

	"github.com/go-kit/log"
	config_integrations "github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/stretchr/testify/require"
)

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with local IPMI",
			args: Arguments{
				Local: LocalConfig{Enabled: true},
			},
		},
		{
			name: "valid with local IPMI and module",
			args: Arguments{
				Local: LocalConfig{Enabled: true, Module: "default"},
			},
		},
		{
			name: "valid with single remote target",
			args: Arguments{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100"},
				},
			},
		},
		{
			name: "valid with multiple remote targets",
			args: Arguments{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100", Module: "default"},
					{Name: "server2", Target: "192.168.1.101"},
				},
			},
		},
		{
			name: "valid with local and remote targets",
			args: Arguments{
				Local: LocalConfig{Enabled: true},
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100"},
				},
			},
		},
		{
			name:    "invalid - no local or remote targets",
			args:    Arguments{},
			wantErr: true,
			errMsg:  "either local IPMI collection must be enabled or at least one remote target must be configured",
		},
		{
			name: "invalid driver value",
			args: Arguments{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100", Driver: "INVALID"},
				},
			},
			wantErr: true,
			errMsg:  "invalid driver",
		},
		{
			name: "valid driver LAN",
			args: Arguments{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100", Driver: "LAN"},
				},
			},
		},
		{
			name: "valid driver LAN_2_0",
			args: Arguments{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100", Driver: "LAN_2_0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
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
	require.Equal(t, "ipmi_exporter", cfg.Name())
}

func TestConfig_InstanceKey(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		defaultKey string
		want       string
	}{
		{
			name: "local IPMI returns localhost",
			cfg: &Config{
				Local: LocalConfig{Enabled: true},
			},
			defaultKey: "default",
			want:       "localhost",
		},
		{
			name: "single remote target returns target address",
			cfg: &Config{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100"},
				},
			},
			defaultKey: "default",
			want:       "192.168.1.100",
		},
		{
			name: "multiple remote targets return default key",
			cfg: &Config{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100"},
					{Name: "server2", Target: "192.168.1.101"},
				},
			},
			defaultKey: "default",
			want:       "default",
		},
		{
			name:       "no targets return default key",
			cfg:        &Config{},
			defaultKey: "default",
			want:       "default",
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

func TestConfig_NewIntegration(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with local",
			cfg: &Config{
				Local: LocalConfig{Enabled: true},
			},
		},
		{
			name: "valid with remote target",
			cfg: &Config{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100"},
				},
			},
		},
		{
			name:    "invalid - no targets",
			cfg:     &Config{},
			wantErr: true,
			errMsg:  "either local IPMI collection must be enabled or at least one remote target must be configured",
		},
		{
			name: "invalid - missing target name",
			cfg: &Config{
				Targets: []IPMITarget{
					{Target: "192.168.1.100"},
				},
			},
			wantErr: true,
			errMsg:  "IPMI target must have both name and target fields set",
		},
		{
			name: "invalid - missing target address",
			cfg: &Config{
				Targets: []IPMITarget{
					{Name: "server1"},
				},
			},
			wantErr: true,
			errMsg:  "IPMI target must have both name and target fields set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNopLogger()
			_, err := tt.cfg.NewIntegration(logger)
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

func TestIntegration_ScrapeConfigs(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		wantJobs  int
		checkFunc func(t *testing.T, configs []config_integrations.ScrapeConfig)
	}{
		{
			name: "local IPMI creates local job",
			cfg: &Config{
				Local: LocalConfig{Enabled: true},
			},
			wantJobs: 1,
			checkFunc: func(t *testing.T, configs []config_integrations.ScrapeConfig) {
				require.Equal(t, "ipmi_exporter/local", configs[0].JobName)
				require.Equal(t, "/metrics", configs[0].MetricsPath)
				require.Nil(t, configs[0].QueryParams)
			},
		},
		{
			name: "local IPMI with module",
			cfg: &Config{
				Local: LocalConfig{Enabled: true, Module: "default"},
			},
			wantJobs: 1,
			checkFunc: func(t *testing.T, configs []config_integrations.ScrapeConfig) {
				require.Equal(t, "ipmi_exporter/local", configs[0].JobName)
				require.NotNil(t, configs[0].QueryParams)
				require.Equal(t, "default", configs[0].QueryParams.Get("module"))
			},
		},
		{
			name: "single remote target",
			cfg: &Config{
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100"},
				},
			},
			wantJobs: 1,
			checkFunc: func(t *testing.T, configs []config_integrations.ScrapeConfig) {
				require.Equal(t, "ipmi_exporter/server1", configs[0].JobName)
				require.Equal(t, "192.168.1.100", configs[0].QueryParams.Get("target"))
			},
		},
		{
			name: "local and remote targets",
			cfg: &Config{
				Local: LocalConfig{Enabled: true, Module: "local"},
				Targets: []IPMITarget{
					{Name: "server1", Target: "192.168.1.100", Module: "remote"},
				},
			},
			wantJobs: 2,
			checkFunc: func(t *testing.T, configs []config_integrations.ScrapeConfig) {
				require.Equal(t, "ipmi_exporter/local", configs[0].JobName)
				require.Equal(t, "local", configs[0].QueryParams.Get("module"))
				require.Equal(t, "ipmi_exporter/server1", configs[1].JobName)
				require.Equal(t, "192.168.1.100", configs[1].QueryParams.Get("target"))
				require.Equal(t, "remote", configs[1].QueryParams.Get("module"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNopLogger()
			intg, err := tt.cfg.NewIntegration(logger)
			require.NoError(t, err)

			configs := intg.ScrapeConfigs()
			require.Len(t, configs, tt.wantJobs)

			if tt.checkFunc != nil {
				tt.checkFunc(t, configs)
			}
		})
	}
}
