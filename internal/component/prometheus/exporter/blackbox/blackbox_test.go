package blackbox

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/syntax"
	blackbox_config "github.com/prometheus/blackbox_exporter/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestUnmarshalAlloy(t *testing.T) {
	alloyCfg := `
		config_file = "modules.yml"
		target {
			name = "target_a"
			address = "http://example.com"
			module = "http_2xx"
		}
		target {
			name = "target-b"
			address = "http://grafana.com"
			module = "http_2xx"
		}
		probe_timeout_offset = "0.5s"
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "modules.yml", args.ConfigFile)
	require.Equal(t, 2, len(args.Targets))
	require.Equal(t, 500*time.Millisecond, args.ProbeTimeoutOffset)
	require.Contains(t, "target_a", args.Targets[0].Name)
	require.Contains(t, "http://example.com", args.Targets[0].Target)
	require.Contains(t, "http_2xx", args.Targets[0].Module)
	require.Contains(t, "target-b", args.Targets[1].Name)
	require.Contains(t, "http://grafana.com", args.Targets[1].Target)
	require.Contains(t, "http_2xx", args.Targets[1].Module)
}

func TestUnmarshalAlloyTargets(t *testing.T) {
	alloyCfg := `
		config_file = "modules.yml"
		targets = [
			{
				"name" = "target_a", 
				"address" = "http://example.com", 
				"module" = "http_2xx",
				"some_label1" = "a",
				"some_label2" = "b",
			},
			{
				"name" = "target_b", 
				"address" = "http://grafana.com", 
				"module" = "http_2xx",
			},
		  ]	
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "modules.yml", args.ConfigFile)
	require.Equal(t, 2, len(args.TargetsList))

	require.Contains(t, "target_a", args.TargetsList[0]["name"])
	require.Contains(t, "http://example.com", args.TargetsList[0]["address"])
	require.Contains(t, "http_2xx", args.TargetsList[0]["module"])
	require.Contains(t, "a", args.TargetsList[0]["some_label1"])
	require.Contains(t, "b", args.TargetsList[0]["some_label2"])

	require.Contains(t, "target_b", args.TargetsList[1]["name"])
	require.Contains(t, "http://grafana.com", args.TargetsList[1]["address"])
	require.Contains(t, "http_2xx", args.TargetsList[1]["module"])
}

func TestUnmarshalAlloyWithInlineConfig(t *testing.T) {
	alloyCfg := `
		config = "{ modules: { http_2xx: { prober: http, timeout: 5s } } }"

		target {
			name = "target_a"
			address = "http://example.com"
			module = "http_2xx"
		}
		target {
			name = "target-b"
			address = "http://grafana.com"
			module = "http_2xx"
		}
		probe_timeout_offset = "0.5s"
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "", args.ConfigFile)
	var blackboxConfig blackbox_config.Config
	err = yaml.UnmarshalStrict([]byte(args.Config.Value), &blackboxConfig)
	require.NoError(t, err)
	require.Equal(t, blackboxConfig.Modules["http_2xx"].Prober, "http")
	require.Equal(t, blackboxConfig.Modules["http_2xx"].Timeout, 5*time.Second)
	require.Equal(t, 2, len(args.Targets))
	require.Equal(t, 500*time.Millisecond, args.ProbeTimeoutOffset)
	require.Contains(t, "target_a", args.Targets[0].Name)
	require.Contains(t, "http://example.com", args.Targets[0].Target)
	require.Contains(t, "http_2xx", args.Targets[0].Module)
	require.Contains(t, "target-b", args.Targets[1].Name)
	require.Contains(t, "http://grafana.com", args.Targets[1].Target)
	require.Contains(t, "http_2xx", args.Targets[1].Module)
}

func TestUnmarshalAlloyWithInlineConfigYaml(t *testing.T) {
	alloyCfg := `
		config = "modules:\n  http_2xx:\n    prober: http\n    timeout: 5s\n"

		target {
			name = "target_a" 
			address = "http://example.com"
			module = "http_2xx"
		}
		target {
			name = "target-b"
			address = "http://grafana.com"
			module = "http_2xx"
		}
		probe_timeout_offset = "0.5s"
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "", args.ConfigFile)
	var blackboxConfig blackbox_config.Config
	err = yaml.UnmarshalStrict([]byte(args.Config.Value), &blackboxConfig)
	require.NoError(t, err)
	require.Equal(t, blackboxConfig.Modules["http_2xx"].Prober, "http")
	require.Equal(t, blackboxConfig.Modules["http_2xx"].Timeout, 5*time.Second)
	require.Equal(t, 2, len(args.Targets))
	require.Equal(t, 500*time.Millisecond, args.ProbeTimeoutOffset)
	require.Contains(t, "target_a", args.Targets[0].Name)
	require.Contains(t, "http://example.com", args.Targets[0].Target)
	require.Contains(t, "http_2xx", args.Targets[0].Module)
	require.Contains(t, "target-b", args.Targets[1].Name)
	require.Contains(t, "http://grafana.com", args.Targets[1].Target)
	require.Contains(t, "http_2xx", args.Targets[1].Module)
}

func TestUnmarshalAlloyWithInvalidConfig(t *testing.T) {
	var tests = []struct {
		testname      string
		cfg           string
		expectedError string
	}{
		{
			"Invalid YAML",
			`
			config = "{ modules: { http_2xx: { prober: http, timeout: 5s }"

			target {
				name = "target_a"
				address = "http://example.com"
				module = "http_2xx"
			}
			`,
			`invalid blackbox_exporter config: yaml: line 1: did not find expected ',' or '}'`,
		},
		{
			"Invalid property",
			`
			config = "{ module: { http_2xx: { prober: http, timeout: 5s } } }"

			target {
				name = "target_a"
				address = "http://example.com"
				module = "http_2xx"
			}
			`,
			"invalid blackbox_exporter config: yaml: unmarshal errors:\n  line 1: field module not found in type config.plain",
		},
		{
			"Define config and config_file",
			`
			config_file = "config"
			config = "{ modules: { http_2xx: { prober: http, timeout: 5s } } }"

			target {
				name = "target-a"
				address = "http://example.com"
				module = "http_2xx"
			}
			`,
			`config and config_file are mutually exclusive`,
		},
		{
			"Define neither config nor config_file",
			`
			target {
				name = "target-a"
				address = "http://example.com"
				module = "http_2xx"
			}
			`,
			`config or config_file must be set`,
		},
		{
			"Specify label for target block instead of name attribute",
			`
			target "target_a" {
				address = "http://example.com"
				module = "http_2xx"
			}
			`,
			`2:4: block "target" does not support specifying labels`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var args Arguments
			require.EqualError(t, syntax.Unmarshal([]byte(tt.cfg), &args), tt.expectedError)
		})
	}
}

func TestConvertConfig(t *testing.T) {
	args := Arguments{
		ConfigFile:         "modules.yml",
		Targets:            TargetBlock{{Name: "target_a", Target: "http://example.com", Module: "http_2xx"}},
		ProbeTimeoutOffset: 1 * time.Second,
	}

	res := args.Convert()
	require.Equal(t, "modules.yml", res.BlackboxConfigFile)
	require.Equal(t, 1, len(res.BlackboxTargets))
	require.Contains(t, "target_a", res.BlackboxTargets[0].Name)
	require.Contains(t, "http://example.com", res.BlackboxTargets[0].Target)
	require.Contains(t, "http_2xx", res.BlackboxTargets[0].Module)
	require.Equal(t, 1.0, res.ProbeTimeoutOffset)
}

func TestConvertTargets(t *testing.T) {
	targets := TargetBlock{{
		Name:   "target_a",
		Target: "http://example.com",
		Module: "http_2xx",
	}}

	res := targets.Convert()
	require.Equal(t, 1, len(res))
	require.Contains(t, "target_a", res[0].Name)
	require.Contains(t, "http://example.com", res[0].Target)
	require.Contains(t, "http_2xx", res[0].Module)
}

func TestBuildBlackboxTargets(t *testing.T) {
	baseArgs := Arguments{
		ConfigFile:         "modules.yml",
		Targets:            TargetBlock{{Name: "target_a", Target: "http://example.com", Module: "http_2xx"}},
		ProbeTimeoutOffset: 1.0,
	}
	baseTarget := discovery.Target{
		model.SchemeLabel:                   "http",
		model.MetricsPathLabel:              "component/prometheus.exporter.blackbox.default/metrics",
		"instance":                          "prometheus.exporter.blackbox.default",
		"job":                               "integrations/blackbox",
		"__meta_agent_integration_name":     "blackbox",
		"__meta_agent_integration_instance": "prometheus.exporter.blackbox.default",
	}
	args := component.Arguments(baseArgs)
	targets := buildBlackboxTargets(baseTarget, args)
	require.Equal(t, 1, len(targets))
	require.Equal(t, "integrations/blackbox/target_a", targets[0]["job"])
	require.Equal(t, "http://example.com", targets[0]["__param_target"])
	require.Equal(t, "http_2xx", targets[0]["__param_module"])
}

func TestBuildBlackboxTargetsWithExtraLabels(t *testing.T) {
	baseArgs := Arguments{
		ConfigFile: "modules.yml",
		Targets: TargetBlock{{
			Name:   "target_a",
			Target: "http://example.com",
			Module: "http_2xx",
			Labels: map[string]string{
				"env": "test",
				"foo": "bar",
			},
		}},
		ProbeTimeoutOffset: 1.0,
	}
	baseTarget := discovery.Target{
		model.SchemeLabel:                   "http",
		model.MetricsPathLabel:              "component/prometheus.exporter.blackbox.default/metrics",
		"instance":                          "prometheus.exporter.blackbox.default",
		"job":                               "integrations/blackbox",
		"__meta_agent_integration_name":     "blackbox",
		"__meta_agent_integration_instance": "prometheus.exporter.blackbox.default",
	}
	args := component.Arguments(baseArgs)
	targets := buildBlackboxTargets(baseTarget, args)
	require.Equal(t, 1, len(targets))
	require.Equal(t, "integrations/blackbox/target_a", targets[0]["job"])
	require.Equal(t, "http://example.com", targets[0]["__param_target"])
	require.Equal(t, "http_2xx", targets[0]["__param_module"])

	require.Equal(t, "test", targets[0]["env"])
	require.Equal(t, "bar", targets[0]["foo"])

	// Check that the extra labels do not override existing labels
	baseArgs.Targets[0].Labels = map[string]string{
		"job":      "test",
		"instance": "test-instance",
	}
	args = component.Arguments(baseArgs)
	targets = buildBlackboxTargets(baseTarget, args)
	require.Equal(t, 1, len(targets))
	require.Equal(t, "integrations/blackbox/target_a", targets[0]["job"])
	require.Equal(t, "prometheus.exporter.blackbox.default", targets[0]["instance"])
}

// Test convert from TargetsList to []blackbox_exporter.BlackboxTarget
func TestConvertTargetsList(t *testing.T) {
	targets := TargetsList{
		{
			"name":        "target_a",
			"address":     "http://example.com",
			"module":      "http_2xx",
			"some_label1": "a",
			"some_label2": "b",
		},
	}

	res := targets.Convert()
	require.Equal(t, 1, len(res))
	require.Equal(t, "target_a", res[0].Name)
	require.Equal(t, "http://example.com", res[0].Target)
	require.Equal(t, "http_2xx", res[0].Module)
}

// Test convert from TargetsList to []blackbox_exporter.BlackboxTarget
func TestConvertTargetsListDifferentAddressLabel(t *testing.T) {
	targets := TargetsList{
		{
			"name":        "target_a",
			"__address__": "http://example.com",
			"module":      "http_2xx",
			"some_label1": "a",
			"some_label2": "b",
		},
	}

	res := targets.Convert()
	require.Equal(t, 1, len(res))
	require.Equal(t, "target_a", res[0].Name)
	require.Equal(t, "http://example.com", res[0].Target)
	require.Equal(t, "http_2xx", res[0].Module)
}

// Test convert from TargetsList to []BlackboxTarget
func TestConvertTargetsList2(t *testing.T) {
	targets := TargetsList{
		{
			"name":        "target_a",
			"address":     "http://example.com",
			"module":      "http_2xx",
			"some_label1": "a",
			"some_label2": "b",
		},
	}

	res := targets.convertInternal()
	require.Equal(t, 1, len(res))
	require.Equal(t, "target_a", res[0].Name)
	require.Equal(t, "http://example.com", res[0].Target)
	require.Equal(t, "http_2xx", res[0].Module)
	require.Equal(t, "a", res[0].Labels["some_label1"])
	require.Equal(t, "b", res[0].Labels["some_label2"])
}

// Test convert from TargetsList to []BlackboxTarget
func TestConvertTargetsList2DifferentAddressLabel(t *testing.T) {
	targets := TargetsList{
		{
			"name":        "target_a",
			"__address__": "http://example.com",
			"module":      "http_2xx",
			"some_label1": "a",
			"some_label2": "b",
		},
	}

	res := targets.convertInternal()
	require.Equal(t, 1, len(res))
	require.Equal(t, "target_a", res[0].Name)
	require.Equal(t, "http://example.com", res[0].Target)
	require.Equal(t, "http_2xx", res[0].Module)
	require.Equal(t, "a", res[0].Labels["some_label1"])
	require.Equal(t, "b", res[0].Labels["some_label2"])
}

func TestValidateTargetMissingName(t *testing.T) {
	targets := TargetsList{
		{
			"address":     "http://example.com",
			"module":      "http_2xx",
			"some_label1": "a",
			"some_label2": "b",
		},
	}
	args := Arguments{
		ConfigFile:  "modules.yml",
		TargetsList: targets,
	}
	require.ErrorContains(t, args.Validate(), "all targets must have a `name`")
}

func TestValidateTargetMissingAddress(t *testing.T) {
	targets := TargetsList{
		{
			"name":        "target_a",
			"module":      "http_2xx",
			"some_label1": "a",
			"some_label2": "b",
		},
	}
	args := Arguments{
		ConfigFile:  "modules.yml",
		TargetsList: targets,
	}
	require.ErrorContains(t, args.Validate(), "all targets must have an `address` or an `__address__` label")
}

func TestValidateTargetsMutualExclusivity(t *testing.T) {
	targets := TargetsList{
		{
			"name":    "target_a",
			"address": "http://example.com",
			"module":  "http_2xx",
		},
	}
	targetBlock := TargetBlock{{
		Name:   "network_switch_1",
		Target: "192.168.1.2",
		Module: "if_mib",
	}}
	args := Arguments{
		ConfigFile:  "modules.yml",
		TargetsList: targets,
		Targets:     targetBlock,
	}
	require.ErrorContains(t, args.Validate(), "the block `target` and the attribute `targets` are mutually exclusive")
}
