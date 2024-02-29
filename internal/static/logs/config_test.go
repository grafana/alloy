package logs

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pc "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_ApplyDefaults_Validations(t *testing.T) {
	tt := []struct {
		name string
		cfg  string
		err  error
	}{
		{
			name: "two configs with different names",
			err:  nil,
			cfg: untab(`
				positions_directory: /tmp
				configs:
				- name: config-a
				- name: config-b
		  `),
		},
		{
			name: "two configs with same name",
			err:  fmt.Errorf("found two Loki configs with name config-a"),
			cfg: untab(`
				positions_directory: /tmp
				configs:
				- name: config-a
				- name: config-b
				- name: config-a
		  `),
		},
		{
			name: "two configs, different positions path",
			err:  nil,
			cfg: untab(`
				configs:
				- name: config-a
				  positions:
					  filename: /tmp/file-a.yml
				- name: config-b
				  positions:
					  filename: /tmp/file-b.yml
		  `),
		},
		{
			name: "re-used positions path",
			err:  fmt.Errorf("Loki configs config-a and config-c must have different positions file paths"),
			cfg: untab(`
				configs:
				- name: config-a
				  positions:
					  filename: /tmp/file-a.yml
				- name: config-b
				  positions:
					  filename: /tmp/file-b.yml
				- name: config-c
				  positions:
					  filename: /tmp/file-a.yml
		  `),
		},
		{
			name: "empty name",
			err:  fmt.Errorf("Loki config index 1 must have a name"),
			cfg: untab(`
				positions_directory: /tmp
				configs:
				- name: config-a
				- name:
				- name: config-a
		  `),
		},
		{
			name: "generated positions file path without positions_directory",
			err:  fmt.Errorf("cannot generate Loki positions file path for config-b because positions_directory is not configured"),
			cfg: untab(`
				configs:
				- name: config-a
				  positions:
					  filename: /tmp/config-a.yaml
				- name: config-b
		  `),
		},
		{
			name: "global filewatcher",
			err:  nil,
			cfg: untab(`
				global:
    				file_watch_config:
      					min_poll_frequency: 1s
      					max_poll_frequency: 20s
				positions_directory: /tmp
				configs:
				- name: config-a
				- name: config-b
		  `),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			err := yaml.UnmarshalStrict([]byte(tc.cfg), &cfg)
			require.NoError(t, err)
			err = cfg.ApplyDefaults()
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestConfig_ApplyDefaults_Defaults(t *testing.T) {
	cfgText := untab(`
    positions_directory: /tmp
    global:
      clients:
        - basic_auth:
            password: password_default
            username: username_default
          url: https://default.com
    configs:
    - name: config-a
      positions:
        filename: /config-a.yml
    - name: config-b
    - name: config-c
      clients:
      - basic_auth:
          password: password
          username: username
        url: https://example.com
  `)
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(cfgText), &cfg)
	require.NoError(t, err)
	err = cfg.ApplyDefaults()
	require.NoError(t, err)

	var (
		pathA = cfg.Configs[0].PositionsConfig.PositionsFile
		pathB = cfg.Configs[1].PositionsConfig.PositionsFile

		clientB = cfg.Configs[1].ClientConfigs[0]
		clientC = cfg.Configs[2].ClientConfigs[0]
	)

	require.Equal(t, "/config-a.yml", pathA)
	require.Equal(t, filepath.Join("/tmp", "config-b.yml"), pathB)
	require.Equal(t, "https://default.com", clientB.URL.String())
	require.Equal(t, &pc.BasicAuth{
		Password: "password_default",
		Username: "username_default",
	}, clientB.Client.BasicAuth)

	require.Equal(t, "https://example.com", clientC.URL.String())
	require.Equal(t, &pc.BasicAuth{
		Password: "password",
		Username: "username",
	}, clientC.Client.BasicAuth)
}

// untab is a utility function to make it easier to write YAML tests, where some editors
// will insert tabs into strings by default.
func untab(s string) string {
	return strings.ReplaceAll(s, "\t", "  ")
}

func TestInstanceConfig_Initialize(t *testing.T) {
	cfgText := `
name: config-c
`
	var cfg InstanceConfig
	err := yaml.UnmarshalStrict([]byte(cfgText), &cfg)
	require.NoError(t, err)

	// Make sure the default values from flags are applied
	require.Equal(t, 10*time.Second, cfg.PositionsConfig.SyncPeriod)
	require.Equal(t, "", cfg.PositionsConfig.PositionsFile)
	require.Equal(t, false, cfg.PositionsConfig.IgnoreInvalidYaml)
	require.Equal(t, false, cfg.TargetConfig.Stdin)
}
