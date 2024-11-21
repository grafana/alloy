package ssh_exporter

import (
    "os"
    "testing"

    "github.com/go-kit/log"
    "github.com/go-kit/log/level"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/require"
    "gopkg.in/yaml.v2"
)
func TestConfig_UnmarshalYAML_MultipleTargets(t *testing.T) {
    yamlConfig := `
verbose_logging: true
targets:
  - address: "192.168.1.10"
    port: 22
    username: "admin"
    password: "password"
    command_timeout: 10
    custom_metrics:
      - name: "load_average"
        command: "echo 1.23"
        type: "gauge"
        help: "Load average over 1 minute"
  - address: "192.168.1.11"
    port: 22
    username: "monitor"
    key_file: "/path/to/private.key"
    command_timeout: 15
    custom_metrics:
      - name: "disk_usage"
        command: "echo '50%'"
        type: "gauge"
        help: "Disk usage percentage"
        parse_regex: '(\d+)%'
`

    var c Config

    require.NoError(t, yaml.UnmarshalStrict([]byte(yamlConfig), &c))

    expectedConfig := Config{
        VerboseLogging: true,
        Targets: []Target{
            {
                Address:        "192.168.1.10",
                Port:           22,
                Username:       "admin",
                Password:       "password",
                CommandTimeout: 10,
                CustomMetrics: []CustomMetric{
                    {
                        Name:    "load_average",
                        Command: "echo 1.23",
                        Type:    "gauge",
                        Help:    "Load average over 1 minute",
                    },
                },
            },
            {
                Address:        "192.168.1.11",
                Port:           22,
                Username:       "monitor",
                KeyFile:        "/path/to/private.key",
                CommandTimeout: 15,
                CustomMetrics: []CustomMetric{
                    {
                        Name:       "disk_usage",
                        Command:    "echo '50%'",
                        Type:       "gauge",
                        Help:       "Disk usage percentage",
                        ParseRegex: `(\d+)%`, // Using backticks for raw string
                    },
                },
            },
        },
    }

    require.Equal(t, expectedConfig, c)
}

func TestConfig_UnmarshalYAML(t *testing.T) {
    yamlConfig := `
verbose_logging: true
targets:
  - address: "192.168.1.10"
    port: 22
    username: "admin"
    password: "password"
    command_timeout: 10
    custom_metrics:
      - name: "load_average"
        command: "cat /proc/loadavg | awk '{print $1}'"
        type: "gauge"
        help: "Load average over 1 minute"
        labels:
          host: "server1"
      - name: "disk_usage"
        command: "df / | tail -1 | awk '{print $5}'"
        type: "gauge"
        help: "Disk usage percentage"
        parse_regex: '(\d+)%'
`

    var c Config

    require.NoError(t, yaml.UnmarshalStrict([]byte(yamlConfig), &c))

    expectedConfig := Config{
        VerboseLogging: true,
        Targets: []Target{
            {
                Address:        "192.168.1.10",
                Port:           22,
                Username:       "admin",
                Password:       "password",
                CommandTimeout: 10,
                CustomMetrics: []CustomMetric{
                    {
                        Name:    "load_average",
                        Command: "cat /proc/loadavg | awk '{print $1}'",
                        Type:    "gauge",
                        Help:    "Load average over 1 minute",
                        Labels:  map[string]string{"host": "server1"},
                    },
                    {
                        Name:       "disk_usage",
                        Command:    "df / | tail -1 | awk '{print $5}'",
                        Type:       "gauge",
                        Help:       "Disk usage percentage",
                        ParseRegex: `(\d+)%`, // Using backticks for raw string
                    },
                },
            },
        },
    }

    require.Equal(t, expectedConfig, c)
}
func TestConfig_NewIntegration(t *testing.T) {
    c := &Config{
        VerboseLogging: true,
        Targets: []Target{
            {
                Address:        "192.168.1.10",
                Port:           22,
                Username:       "admin",
                Password:       "password",
                CommandTimeout: 10,
                CustomMetrics: []CustomMetric{
                    {
                        Name:    "load_average",
                        Command: "cat /proc/loadavg | awk '{print $1}'",
                        Type:    "gauge",
                        Help:    "Load average over 1 minute",
                    },
                },
            },
        },
    }

    logger := log.NewJSONLogger(os.Stdout)
    i, err := c.NewIntegration(logger)
    require.NoError(t, err)
    require.NotNil(t, i)

    // You can test the logger configuration here
    lvl := level.NewFilter(logger, level.AllowAll())
    level.Debug(lvl).Log("msg", "test debug log")
}

func TestCustomMetric_Validate(t *testing.T) {
    validMetric := CustomMetric{
        Name:    "test_metric",
        Command: "echo 42",
        Type:    "gauge",
        Help:    "Test metric",
    }
    require.NoError(t, validMetric.Validate())

    invalidMetric := CustomMetric{
        Name:    "",
        Command: "",
        Type:    "unknown",
        Help:    "",
    }
    err := invalidMetric.Validate()
    require.Error(t, err)
}

type MockSSHClient struct {
    Output string
}

func (m *MockSSHClient) RunCommand(command string) (string, error) {
    return m.Output, nil
}

func TestSSHCollector_Collect(t *testing.T) {
    target := Target{
        Address:  "localhost",
        Port:     22,
        Username: "testuser",
        CustomMetrics: []CustomMetric{
            {
                Name:    "test_metric",
                Command: "echo 42",
                Type:    "gauge",
                Help:    "Test metric",
            },
        },
    }

    collector, err := NewSSHCollector(log.NewNopLogger(), target)
    require.NoError(t, err)

    // Replace the client with a mock
    collector.client = &MockSSHClient{
        Output: "42",
    }

    ch := make(chan prometheus.Metric)
    go func() {
        collector.Collect(ch)
        close(ch)
    }()

    var metrics []prometheus.Metric
    for metric := range ch {
        metrics = append(metrics, metric)
    }

    require.Len(t, metrics, 1)
}

func TestNewSSHClient_KeyFileNotFound(t *testing.T) {
    target := Target{
        Address:  "192.168.1.10",
        Port:     22,
        Username: "admin",
        KeyFile:  "/non/existent/keyfile",
    }

    _, err := NewSSHClient(target)
    require.Error(t, err)
    require.Contains(t, err.Error(), "unable to read private key")
}
