package redis_exporter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/static/config"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/stretchr/testify/require"
)

const addr string = "localhost:6379"
const redisExporterFile string = "./redis_exporter.go"
const redisPasswordMapFile string = "./testdata/password_map_file.json"

func TestRedisCases(t *testing.T) {
	tt := []struct {
		name                   string
		cfg                    Config
		expectedMetrics        []string
		expectConstructorError bool
	}{
		// Test that default config results in some metrics that can be parsed by
		// prometheus.
		{
			name: "Default config",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				return c
			})(),
			expectedMetrics: []string{},
		},
		// Test that exporter metrics are included when configured to do so.
		{
			name: "Include exporter metrics",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.IncludeExporterMetrics = true
				return c
			})(),
			expectedMetrics: []string{
				"promhttp_metric_handler_requests_total",
				"promhttp_metric_handler_requests_in_flight",
			},
		},
		// Test that some valid pre-constructor config logic doesn't cause errors.
		{
			name: "Lua script read OK",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.ScriptPath = redisExporterFile // file content is irrelevant
				return c
			})(),
		},
		// Test that multiple lua scripts in a csv doesn't cause errors.
		{
			name: "Multiple Lua scripts read OK",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.ScriptPath = fmt.Sprintf("%s,%s", redisExporterFile, redisPasswordMapFile) // file contents are irrelevant
				return c
			})(),
		},
		// Test that some invalid pre-constructor config logic causes an error.
		{
			name: "Lua script read fail",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.ScriptPath = "/does/not/exist"
				return c
			})(),
			expectConstructorError: true,
		},
		// Test exporter complains when no address given via env or config.
		{
			name:                   "no address given",
			cfg:                    Config{}, // no address in here
			expectConstructorError: true,
		},
		// Test exporter constructs ok when password file is defined and exists
		{
			name: "valid password file",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.RedisPasswordFile = redisExporterFile // contents not important
				return c
			})(),
		},
		// Test exporter construction fails when password file is defined and doesn't
		// exist
		{
			name: "invalid password file",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.RedisPasswordFile = "/does/not/exist"
				return c
			})(),
			expectConstructorError: true,
		},
		// Test exporter constructs ok when password map file is defined, exists, and is valid
		{
			name: "valid password map file",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.RedisPasswordMapFile = redisPasswordMapFile
				return c
			})(),
		},
		// Test exporter fails to construct when the password map file is not valid json
		{
			name: "invalid password map file",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.RedisPasswordMapFile = redisExporterFile
				return c
			})(),
			expectConstructorError: true,
		},
		// Test exporter construction fails when both redis_password_file and redis_password_map_file
		// are specified
		{
			name: "too many password files",
			cfg: (func() Config {
				c := DefaultConfig
				c.RedisAddr = addr
				c.RedisPasswordFile = redisExporterFile    // contents not important
				c.RedisPasswordMapFile = redisExporterFile // contents not important
				return c
			})(),
			expectConstructorError: true,
		},
	}

	logger := log.NewNopLogger()

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			integration, err := New(logger, &test.cfg)

			if test.expectConstructorError {
				require.Error(t, err, "expected failure when setting up redis_exporter")
				return
			}
			require.NoError(t, err, "failed to setup redis_exporter")

			r := mux.NewRouter()
			handler, err := integration.MetricsHandler()
			require.NoError(t, err)
			r.Handle("/metrics", handler)
			require.NoError(t, err)

			srv := httptest.NewServer(r)
			defer srv.Close()

			res, err := http.Get(srv.URL + "/metrics")
			require.NoError(t, err)

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			foundMetricNames := map[string]bool{}
			for _, name := range test.expectedMetrics {
				foundMetricNames[name] = false
			}

			p := textparse.NewPromParser(body, nil, false)
			for {
				entry, err := p.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)

				if entry == textparse.EntryHelp {
					matchMetricNames(foundMetricNames, p)
				}
			}

			for metric, exists := range foundMetricNames {
				require.True(t, exists, "could not find metric %s", metric)
			}
		})
	}
}

func TestConfig_SecretRedisPassword(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  redis_exporter:
    enabled: true
    redis_password: secret_password
`
	config.CheckSecret(t, stringCfg, "secret_password")
}

func TestConfig_DefaultExportKeyValues(t *testing.T) {
	stringCfg := `
enabled: true
redis_addr: localhost:6379`

	var config Config
	err := yaml.Unmarshal([]byte(stringCfg), &config)
	require.NoError(t, err)
	require.True(t, config.ExportKeyValues)
}

func matchMetricNames(names map[string]bool, p textparse.Parser) {
	for name := range names {
		metricName, _ := p.Help()
		if bytes.Equal([]byte(name), metricName) {
			names[name] = true
		}
	}
}
