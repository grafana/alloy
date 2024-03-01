package snmp

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/river"

	"github.com/prometheus/common/model"
	"github.com/prometheus/snmp_exporter/config"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalRiver(t *testing.T) {
	riverCfg := `
		config_file = "modules.yml"
		target "network_switch_1" {
			address = "192.168.1.2"
			module = "if_mib"
			walk_params = "public"
			auth = "public_v2"
		}
		target "network_router_2" {
			address = "192.168.1.3"
			module = "mikrotik"
			walk_params = "private"
		}
		walk_param "private" {
			retries = 1
		}
		walk_param "public" {
			retries = 2
		}		
`
	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "modules.yml", args.ConfigFile)
	require.Equal(t, 2, len(args.Targets))

	require.Contains(t, "network_switch_1", args.Targets[0].Name)
	require.Contains(t, "192.168.1.2", args.Targets[0].Target)
	require.Contains(t, "if_mib", args.Targets[0].Module)
	require.Contains(t, "public", args.Targets[0].WalkParams)
	require.Contains(t, "public_v2", args.Targets[0].Auth)

	require.Contains(t, "network_router_2", args.Targets[1].Name)
	require.Contains(t, "192.168.1.3", args.Targets[1].Target)
	require.Contains(t, "mikrotik", args.Targets[1].Module)
	require.Contains(t, "private", args.Targets[1].WalkParams)
	require.Empty(t, args.Targets[1].Auth)

	require.Equal(t, 2, len(args.WalkParams))

	require.Contains(t, "private", args.WalkParams[0].Name)
	require.Equal(t, 1, args.WalkParams[0].Retries)
	require.Contains(t, "public", args.WalkParams[1].Name)
	require.Equal(t, 2, args.WalkParams[1].Retries)
}

func TestConvertConfig(t *testing.T) {
	args := Arguments{
		ConfigFile: "modules.yml",
		Targets:    TargetBlock{{Name: "network_switch_1", Target: "192.168.1.2", Module: "if_mib"}},
		WalkParams: WalkParams{{Name: "public", Retries: 2}},
	}

	res := args.Convert()
	require.Equal(t, "modules.yml", res.SnmpConfigFile)
	require.Equal(t, 1, len(res.SnmpTargets))
	require.Equal(t, "network_switch_1", res.SnmpTargets[0].Name)
}

func TestConvertConfigWithInlineConfig(t *testing.T) {
	args := Arguments{
		ConfigStruct: config.Config{Modules: map[string]*config.Module{"if_mib": {Walk: []string{"1.3.6.1.2.1.2"}}}},
		Targets:      TargetBlock{{Name: "network_switch_1", Target: "192.168.1.2", Module: "if_mib"}},
		WalkParams:   WalkParams{{Name: "public", Retries: 2}},
	}

	res := args.Convert()
	require.Equal(t, 1, len(res.SnmpTargets))
	require.Equal(t, "network_switch_1", res.SnmpTargets[0].Name)
	require.Equal(t, 1, len(res.SnmpConfig.Modules))
}

func TestConvertTargets(t *testing.T) {
	targets := TargetBlock{{
		Name:   "network_switch_1",
		Target: "192.168.1.2",
		Module: "if_mib",
		Auth:   "public_v2",
	}}

	res := targets.Convert()
	require.Equal(t, 1, len(res))
	require.Equal(t, "network_switch_1", res[0].Name)
	require.Equal(t, "192.168.1.2", res[0].Target)
	require.Equal(t, "if_mib", res[0].Module)
	require.Equal(t, "public_v2", res[0].Auth)
}

func TestConvertWalkParams(t *testing.T) {
	retries := 3
	walkParams := WalkParams{{
		Name:                    "public",
		MaxRepetitions:          uint32(10),
		Retries:                 retries,
		Timeout:                 time.Duration(5),
		UseUnconnectedUDPSocket: true,
	}}

	res := walkParams.Convert()
	require.Equal(t, 1, len(res))
	require.Equal(t, uint32(10), res["public"].MaxRepetitions)
	require.Equal(t, &retries, res["public"].Retries)
	require.Equal(t, time.Duration(5), res["public"].Timeout)
	require.Equal(t, true, res["public"].UseUnconnectedUDPSocket)
}

func TestBuildSNMPTargets(t *testing.T) {
	baseArgs := Arguments{
		ConfigFile: "modules.yml",
		Targets: TargetBlock{{Name: "network_switch_1", Target: "192.168.1.2", Module: "if_mib",
			WalkParams: "public", Auth: "public_v2"}},
		WalkParams: WalkParams{{Name: "public", Retries: 2}},
	}
	baseTarget := discovery.Target{
		model.SchemeLabel:                   "http",
		model.MetricsPathLabel:              "component/prometheus.exporter.snmp.default/metrics",
		"instance":                          "prometheus.exporter.snmp.default",
		"job":                               "integrations/snmp",
		"__meta_agent_integration_name":     "snmp",
		"__meta_agent_integration_instance": "prometheus.exporter.snmp.default",
	}
	args := component.Arguments(baseArgs)
	targets := buildSNMPTargets(baseTarget, args)
	require.Equal(t, 1, len(targets))
	require.Equal(t, "integrations/snmp/network_switch_1", targets[0]["job"])
	require.Equal(t, "192.168.1.2", targets[0]["__param_target"])
	require.Equal(t, "if_mib", targets[0]["__param_module"])
	require.Equal(t, "public", targets[0]["__param_walk_params"])
	require.Equal(t, "public_v2", targets[0]["__param_auth"])
}

func TestUnmarshalRiverWithInlineConfig(t *testing.T) {
	riverCfg := `
		config = "{ modules: {if_mib: {walk: [1.3.6.1.2.1.2], get: [1.3.6.1.2.1.1.3.0], metrics: [{name: sysUpTime, oid: 1.3.6.1.2.1.1.3, type: gauge}]}}, auths: { public_v1: { community: public, security_level: noAuthNoPriv, auth_protocol: MD5, priv_protocol: DES, version: 1 } } }"

		target "network_switch_1" {
			address = "192.168.1.2"
			module = "if_mib"
			walk_params = "public"
			auth = "public_v1"
		}
		target "network_router_2" {
			address = "192.168.1.3"
			module = "if_mib"
			walk_params = "private"
		}
`
	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "", args.ConfigFile)
	require.Equal(t, args.ConfigStruct.Modules["if_mib"].Walk, []string{"1.3.6.1.2.1.2"})
	require.Equal(t, args.ConfigStruct.Modules["if_mib"].Get, []string{"1.3.6.1.2.1.1.3.0"})
	require.Equal(t, args.ConfigStruct.Modules["if_mib"].Metrics[0].Name, "sysUpTime")
	require.Equal(t, args.ConfigStruct.Modules["if_mib"].Metrics[0].Oid, "1.3.6.1.2.1.1.3")
	require.Equal(t, args.ConfigStruct.Modules["if_mib"].Metrics[0].Type, "gauge")

	require.Equal(t, args.ConfigStruct.Auths["public_v1"].Community, config.Secret("public"))
	require.Equal(t, args.ConfigStruct.Auths["public_v1"].SecurityLevel, "noAuthNoPriv")
	require.Equal(t, args.ConfigStruct.Auths["public_v1"].AuthProtocol, "MD5")
	require.Equal(t, args.ConfigStruct.Auths["public_v1"].PrivProtocol, "DES")
	require.Equal(t, args.ConfigStruct.Auths["public_v1"].Version, 1)

	require.Equal(t, 2, len(args.Targets))
	require.Contains(t, "network_switch_1", args.Targets[0].Name)
	require.Contains(t, "192.168.1.2", args.Targets[0].Target)
	require.Contains(t, "if_mib", args.Targets[0].Module)
	require.Contains(t, "public", args.Targets[0].WalkParams)
	require.Contains(t, "public_v1", args.Targets[0].Auth)

	require.Contains(t, "network_router_2", args.Targets[1].Name)
	require.Contains(t, "192.168.1.3", args.Targets[1].Target)
	require.Contains(t, "if_mib", args.Targets[1].Module)
	require.Contains(t, "private", args.Targets[1].WalkParams)
	require.Contains(t, "public_v2", args.Targets[1].Auth)
}
func TestUnmarshalRiverWithInvalidInlineConfig(t *testing.T) {
	var tests = []struct {
		testname      string
		cfg           string
		expectedError string
	}{
		{
			"Invalid YAML",
			`
			config = "{ auth: { public_v1: { community: public, version: 1 } }"


			target "network_switch_1" {
				address = "192.168.1.2"
				module = "if_mib"
				walk_params = "public"
				auth = "public_v2"
			}
			`,
			`invalid snmp_exporter config: yaml: line 1: did not find expected ',' or '}'`,
		},
		{
			"Invalid property",
			`
			config = "{ auths: { public_v1: { community: public, versions: 1 } } }"

			target "network_switch_1" {
				address = "192.168.1.2"
				module = "if_mib"
				walk_params = "public"
				auth = "public_v2"
			}
			`,
			"invalid snmp_exporter config: yaml: unmarshal errors:\n  line 1: field versions not found in type config.plain",
		},
		{
			"Define config and config_file",
			`
			config_file = "config"
			config = "{ auths: { public_v1: { community: public, version: 1 } } }"

			target "network_switch_1" {
				address = "192.168.1.2"
				module = "if_mib"
				walk_params = "public"
				auth = "public_v2"
			}
			`,
			`config and config_file are mutually exclusive`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var args Arguments
			require.EqualError(t, river.Unmarshal([]byte(tt.cfg), &args), tt.expectedError)
		})
	}
}
