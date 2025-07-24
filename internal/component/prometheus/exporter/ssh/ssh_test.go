package ssh

import (
	"testing"
	"time"

	ssh_exporter "github.com/grafana/alloy/internal/component/prometheus/exporter/ssh/ssh_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

// TestAlloyUnmarshal_StaticExample validates basic DSL parsing for a single target.
func TestAlloyUnmarshal_StaticExample(t *testing.T) {
	example := `
targets {
  address         = "192.168.1.10"
  port            = 22
  username        = "admin"
  password        = "password"
  command_timeout = "10s"

  custom_metrics {
    name    = "load_average"
    command = "cat /proc/loadavg | awk '{print $1}'"
    type    = "gauge"
    help    = "Load average over 1 minute"
    labels  = { host = "192.168.1.10" }
  }
}
`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(example), &args))
	require.Len(t, args.Targets, 1)
	tgt := args.Targets[0]
	require.Equal(t, "192.168.1.10", tgt.Address)
	require.Equal(t, 22, tgt.Port)
	require.Equal(t, "admin", tgt.Username)
	require.Equal(t, "password", tgt.Password)
	require.Equal(t, 10*time.Second, tgt.CommandTimeout)
	require.Len(t, tgt.CustomMetrics, 1)
	cm := tgt.CustomMetrics[0]
	require.Equal(t, "load_average", cm.Name)
	require.Equal(t, "cat /proc/loadavg | awk '{print $1}'", cm.Command)
	require.Equal(t, "gauge", cm.Type)
	require.Equal(t, "Load average over 1 minute", cm.Help)
	require.Equal(t, map[string]string{"host": "192.168.1.10"}, cm.Labels)
}

// TestAlloyUnmarshal_KeyFileInterpolation ensures interpolation literals are preserved.
func TestAlloyUnmarshal_KeyFileInterpolation(t *testing.T) {
	example := `
targets {
  address  = "localhost"
  port     = 22
  username = "user"
  key_file = "/path/${var.name}.pem"
  custom_metrics {
    name    = "m"
    command = "echo 1"
    type    = "gauge"
  }
}
`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(example), &args))
	require.Len(t, args.Targets, 1)
	require.Equal(t, "/path/${var.name}.pem", args.Targets[0].KeyFile)
}

// TestArgumentsValidate covers argument validation rules.
func TestArgumentsValidate(t *testing.T) {
	cases := []struct {
		name    string
		args    Arguments
		wantErr bool
	}{
		{"no targets", Arguments{}, true},
		{"empty address", Arguments{Targets: []Target{{}}}, true},
		{"missing username", Arguments{Targets: []Target{{Address: "a", Port: 22}}}, true},
		{"invalid port", Arguments{Targets: []Target{{Address: "a", Port: 0, Username: "u"}}}, true},
		{"valid config", Arguments{Targets: []Target{{Address: "a", Port: 22, Username: "u"}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestConvert ensures Arguments.Convert produces the expected exporter Config.
func TestConvert(t *testing.T) {
	args := Arguments{Targets: []Target{{
		Address:        "a",
		Port:           22,
		Username:       "u",
		Password:       "p",
		CommandTimeout: 5 * time.Second,
		CustomMetrics:  []CustomMetric{{Name: "m", Command: "echo 1", Type: "gauge"}},
	}}}
	cfg := args.Convert()
	expected := &ssh_exporter.Config{
		Targets: []ssh_exporter.Target{{
			SkipAuth:       true,
			Address:        "a",
			Port:           22,
			Username:       "u",
			Password:       "p",
			CommandTimeout: 5 * time.Second,
			CustomMetrics:  []ssh_exporter.CustomMetric{{Name: "m", Command: "echo 1", Type: "gauge", Help: "", Labels: nil, ParseRegex: ""}},
		}},
	}
	require.Equal(t, expected, cfg)
}
