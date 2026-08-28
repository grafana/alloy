package metrics

import (
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/static/metrics/instance"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_Validate(t *testing.T) {
	valid := Config{
		WALDir: "/tmp/data",
		Configs: []instance.Config{
			makeInstanceConfig("instance"),
		},
		InstanceMode: instance.DefaultMode,
	}

	tt := []struct {
		name    string
		mutator func(c *Config)
		expect  error
	}{
		{
			name:    "complete config should be valid",
			mutator: func(c *Config) {},
			expect:  nil,
		},
		{
			name:    "no wal dir",
			mutator: func(c *Config) { c.WALDir = "" },
			expect:  errors.New("no wal_directory configured"),
		},
		{
			name:    "missing instance name",
			mutator: func(c *Config) { c.Configs[0].Name = "" },
			expect:  errors.New("error validating instance at index 0: missing instance name"),
		},
		{
			name: "duplicate config name",
			mutator: func(c *Config) {
				c.Configs = append(c.Configs,
					makeInstanceConfig("newinstance"),
					makeInstanceConfig("instance"),
				)
			},
			expect: errors.New("prometheus instance names must be unique. found multiple instances with name instance"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := copyConfig(t, valid)
			tc.mutator(&cfg)

			err := cfg.ApplyDefaults()
			if tc.expect == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expect.Error())
			}
		})
	}
}

func copyConfig(t *testing.T, c Config) Config {
	t.Helper()

	bb, err := yaml.Marshal(c)
	require.NoError(t, err)

	var cp Config
	err = yaml.Unmarshal(bb, &cp)
	require.NoError(t, err)
	return cp
}

func TestConfigNonzeroDefaultScrapeInterval(t *testing.T) {
	cfgText := `
wal_directory: ./wal
configs:
  - name: testconfig
    scrape_configs:
      - job_name: 'node'
        static_configs:
          - targets: ['localhost:9100']
`

	var cfg Config

	err := yaml.Unmarshal([]byte(cfgText), &cfg)
	require.NoError(t, err)
	err = cfg.ApplyDefaults()
	require.NoError(t, err)

	require.Equal(t, len(cfg.Configs), 1)
	instanceConfig := cfg.Configs[0]
	require.Equal(t, len(instanceConfig.ScrapeConfigs), 1)
	scrapeConfig := instanceConfig.ScrapeConfigs[0]
	require.Greater(t, int64(scrapeConfig.ScrapeInterval), int64(0))
}

func makeInstanceConfig(name string) instance.Config {
	cfg := instance.DefaultConfig
	cfg.Name = name
	return cfg
}

func TestAgent_MarshalYAMLOmitDefaultConfigFields(t *testing.T) {
	cfg := DefaultConfig
	yml, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.NotContains(t, string(yml), "scraping_service_client")
	require.NotContains(t, string(yml), "scraping_service")
	require.NotContains(t, string(yml), "global")
}
