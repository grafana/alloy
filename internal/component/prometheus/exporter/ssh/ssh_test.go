package ssh

import (
    "testing"

    "github.com/grafana/alloy/internal/static/integrations/ssh_exporter"
    "github.com/grafana/alloy/syntax"
    "github.com/stretchr/testify/require"

)


func TestAlloyUnmarshal(t *testing.T) {
    alloyConfig := `
verbose_logging = true

targets {
  address         = "192.168.1.10"
  port            = 22
  username        = "admin"
  password        = "password"
  command_timeout = 10

  custom_metrics {
    name    = "load_average"
    command = "cat /proc/loadavg | awk '{print $1}'"
    type    = "gauge"
    help    = "Load average over 1 minute"
  }
}

targets {
  address         = "192.168.1.11"
  port            = 22
  username        = "monitor"
  key_file        = "/path/to/private.key"
  command_timeout = 15
}
`

    var args Arguments
    err := syntax.Unmarshal([]byte(alloyConfig), &args)
    require.NoError(t, err)

    expected := Arguments{
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
            {
                Address:        "192.168.1.11",
                Port:           22,
                Username:       "monitor",
                KeyFile:        "/path/to/private.key",
                CommandTimeout: 15,
            },
        },
    }

    require.Equal(t, expected, args)
}


func TestArgumentsValidate(t *testing.T) {
    tests := []struct {
        name    string
        args    Arguments
        wantErr bool
        errMsg  string
    }{
        {
            name: "no targets",
            args: Arguments{
                Targets: nil,
            },
            wantErr: true,
            errMsg:  "at least one target must be specified",
        },
        {
            name: "empty target address",
            args: Arguments{
                Targets: []Target{
                    {
                        Address:  "",
                        Port:     22,
                        Username: "admin",
                    },
                },
            },
            wantErr: true,
            errMsg:  "target address cannot be empty",
        },
        {
            name: "missing username",
            args: Arguments{
                Targets: []Target{
                    {
                        Address:  "192.168.1.10",
                        Port:     22,
                        Username: "",
                    },
                },
            },
            wantErr: true,
            errMsg:  "username cannot be empty",
        },
        {
            name: "invalid port number",
            args: Arguments{
                Targets: []Target{
                    {
                        Address:  "192.168.1.10",
                        Port:     -1,
                        Username: "admin",
                    },
                },
            },
            wantErr: true,
            errMsg:  "invalid port",
        },
        {
            name: "unsupported metric type",
            args: Arguments{
                Targets: []Target{
                    {
                        Address:  "192.168.1.10",
                        Port:     22,
                        Username: "admin",
                        CustomMetrics: []CustomMetric{
                            {
                                Name:    "invalid_metric",
                                Command: "echo 42",
                                Type:    "histogram", // Assuming only "gauge" and "counter" are supported
                            },
                        },
                    },
                },
            },
            wantErr: true,
            errMsg:  "unsupported metric type",
        },
        {
            name: "valid configuration",
            args: Arguments{
                Targets: []Target{
                    {
                        Address:        "192.168.1.10",
                        Port:           22,
                        Username:       "admin",
                        Password:       "password",
                        CommandTimeout: 10,
                        CustomMetrics: []CustomMetric{
                            {
                                Name:    "metric1",
                                Command: "echo 42",
                                Type:    "gauge",
                                Help:    "Test metric",
                            },
                        },
                    },
                },
            },
            wantErr: false,
        },
        // ... you can add more test cases if needed ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.args.Validate()
            if tt.wantErr {
                require.Error(t, err)
                require.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}


func TestConvert(t *testing.T) {
    args := Arguments{
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
                        Name:    "metric1",
                        Command: "echo 42",
                        Type:    "gauge",
                        Help:    "Test metric",
                    },
                },
            },
        },
    }

    res := args.Convert()

    expected := &ssh_exporter.Config{
        VerboseLogging: true,
        Targets: []ssh_exporter.Target{
            {
                Address:        "192.168.1.10",
                Port:           22,
                Username:       "admin",
                Password:       "password",
                CommandTimeout: 10,
                CustomMetrics: []ssh_exporter.CustomMetric{
                    {
                        Name:    "metric1",
                        Command: "echo 42",
                        Type:    "gauge",
                        Help:    "Test metric",
                    },
                },
            },
        },
    }

    require.Equal(t, expected, res)
}


func TestAlloyUnmarshal_MultipleTargets(t *testing.T) {
    alloyConfig := `
verbose_logging = true

targets {
  address         = "192.168.1.10"
  port            = 22
  username        = "admin"
  password        = "password"
  command_timeout = 10

  custom_metrics {
    name    = "cpu_usage"
    command = "top -bn1 | grep 'Cpu(s)' | awk '{print $2 + $4}'"
    type    = "gauge"
    help    = "CPU usage percentage"
  }

  custom_metrics {
    name    = "memory_available"
    command = "free -m | awk '/Mem:/ {print $7}'"
    type    = "gauge"
    help    = "Available memory in MB"
  }
}

targets {
  address         = "192.168.1.11"
  port            = 22
  username        = "monitor"
  key_file        = "/path/to/private.key"
  command_timeout = 15

  custom_metrics {
    name    = "disk_usage"
    command = "df / | tail -1 | awk '{print $5}'"
    type    = "gauge"
    help    = "Disk usage percentage"
    parse_regex = "(\\d+)%"
  }
}

targets {
  address         = "192.168.1.12"
  port            = 22
  username        = "user"
  password        = "secret"
  command_timeout = 20

  custom_metrics {
    name    = "network_in"
    command = "ifconfig eth0 | grep 'RX packets' | awk '{print $5}'"
    type    = "counter"
    help    = "Network input packets"
  }
}
`

    var args Arguments
    err := syntax.Unmarshal([]byte(alloyConfig), &args)
    require.NoError(t, err)

    expected := Arguments{
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
                        Name:    "cpu_usage",
                        Command: "top -bn1 | grep 'Cpu(s)' | awk '{print $2 + $4}'",
                        Type:    "gauge",
                        Help:    "CPU usage percentage",
                    },
                    {
                        Name:    "memory_available",
                        Command: "free -m | awk '/Mem:/ {print $7}'",
                        Type:    "gauge",
                        Help:    "Available memory in MB",
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
                        Command:    "df / | tail -1 | awk '{print $5}'",
                        Type:       "gauge",
                        Help:       "Disk usage percentage",
                        ParseRegex: `(\d+)%`,
                    },
                },
            },
            {
                Address:        "192.168.1.12",
                Port:           22,
                Username:       "user",
                Password:       "secret",
                CommandTimeout: 20,
                CustomMetrics: []CustomMetric{
                    {
                        Name:    "network_in",
                        Command: "ifconfig eth0 | grep 'RX packets' | awk '{print $5}'",
                        Type:    "counter",
                        Help:    "Network input packets",
                    },
                },
            },
        },
    }

    require.Equal(t, expected, args)
}
