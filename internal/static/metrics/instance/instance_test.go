package instance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/require"
)

func TestConfig_Unmarshal_Defaults(t *testing.T) {
	global := DefaultGlobalConfig
	cfgText := `name: test
scrape_configs:
  - job_name: local_scrape
    static_configs:
      - targets: ['127.0.0.1:12345']
        labels:
          cluster: 'localhost'
remote_write:
  - url: http://localhost:9009/api/prom/push`

	cfg, err := UnmarshalConfig(strings.NewReader(cfgText))
	require.NoError(t, err)

	err = cfg.ApplyDefaults(global)
	require.NoError(t, err)

	require.Equal(t, DefaultConfig.HostFilter, cfg.HostFilter)
	require.Equal(t, DefaultConfig.WALTruncateFrequency, cfg.WALTruncateFrequency)
	require.Equal(t, DefaultConfig.RemoteFlushDeadline, cfg.RemoteFlushDeadline)
	require.Equal(t, DefaultConfig.WriteStaleOnShutdown, cfg.WriteStaleOnShutdown)

	for _, sc := range cfg.ScrapeConfigs {
		require.Equal(t, sc.ScrapeInterval, global.Prometheus.ScrapeInterval)
		require.Equal(t, sc.ScrapeTimeout, global.Prometheus.ScrapeTimeout)
	}
}

func TestConfig_ApplyDefaults_Validations(t *testing.T) {
	global := DefaultGlobalConfig
	cfg := DefaultConfig
	cfg.Name = "instance"
	cfg.ScrapeConfigs = []*config.ScrapeConfig{{
		JobName: "scrape",
		ServiceDiscoveryConfigs: discovery.Configs{
			discovery.StaticConfig{{
				Targets: []model.LabelSet{{
					model.AddressLabel: model.LabelValue("127.0.0.1:12345"),
				}},
				Labels: model.LabelSet{"cluster": "localhost"},
			}},
		},
	}}
	cfg.RemoteWrite = []*config.RemoteWriteConfig{{Name: "write"}}

	tt := []struct {
		name     string
		mutation func(c *Config)
		err      error
	}{
		{
			"valid config",
			nil,
			nil,
		},
		{
			"requires name",
			func(c *Config) { c.Name = "" },
			fmt.Errorf("missing instance name"),
		},
		{
			"missing scrape",
			func(c *Config) { c.ScrapeConfigs[0] = nil },
			fmt.Errorf("empty or null scrape config section"),
		},
		{
			"missing wal truncate frequency",
			func(c *Config) { c.WALTruncateFrequency = 0 },
			fmt.Errorf("wal_truncate_frequency must be greater than 0s"),
		},
		{
			"missing remote flush deadline",
			func(c *Config) { c.RemoteFlushDeadline = 0 },
			fmt.Errorf("remote_flush_deadline must be greater than 0s"),
		},
		{
			"scrape timeout too high",
			func(c *Config) { c.ScrapeConfigs[0].ScrapeTimeout = global.Prometheus.ScrapeInterval + 1 },
			fmt.Errorf("scrape timeout greater than scrape interval for scrape config with job name \"scrape\""),
		},
		{
			"scrape interval greater than truncate frequency",
			func(c *Config) { c.ScrapeConfigs[0].ScrapeInterval = model.Duration(c.WALTruncateFrequency + 1) },
			fmt.Errorf("scrape interval greater than wal_truncate_frequency for scrape config with job name \"scrape\""),
		},
		{
			"multiple scrape configs with same name",
			func(c *Config) {
				c.ScrapeConfigs = append(c.ScrapeConfigs, &config.ScrapeConfig{
					JobName: "scrape",
				})
			},
			fmt.Errorf("found multiple scrape configs with job name \"scrape\""),
		},
		{
			"empty remote write",
			func(c *Config) { c.RemoteWrite = append(c.RemoteWrite, nil) },
			fmt.Errorf("empty or null remote write config section"),
		},
		{
			"multiple remote writes with same name",
			func(c *Config) {
				c.RemoteWrite = []*config.RemoteWriteConfig{
					{Name: "foo"},
					{Name: "foo"},
				}
			},
			fmt.Errorf("found duplicate remote write configs with name \"foo\""),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Copy the input and all of its slices
			input := cfg

			var scrapeConfigs []*config.ScrapeConfig
			for _, sc := range input.ScrapeConfigs {
				scCopy := *sc
				scrapeConfigs = append(scrapeConfigs, &scCopy)
			}
			input.ScrapeConfigs = scrapeConfigs

			var remoteWrites []*config.RemoteWriteConfig
			for _, rw := range input.RemoteWrite {
				rwCopy := *rw
				remoteWrites = append(remoteWrites, &rwCopy)
			}
			input.RemoteWrite = remoteWrites

			if tc.mutation != nil {
				tc.mutation(&input)
			}

			err := input.ApplyDefaults(global)
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestConfig_ApplyDefaults_HashedName(t *testing.T) {
	cfgText := `
name: default
host_filter: false
remote_write:
- url: http://localhost:9009/api/prom/push
  sigv4: {}`

	cfg, err := UnmarshalConfig(strings.NewReader(cfgText))
	require.NoError(t, err)
	require.NoError(t, cfg.ApplyDefaults(DefaultGlobalConfig))
	require.NotEmpty(t, cfg.RemoteWrite[0].Name)
}
