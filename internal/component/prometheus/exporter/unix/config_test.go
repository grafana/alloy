package unix

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestArguments_Qdisc(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  string
		include string
		exclude string
	}{
		{name: "omitted"},
		{name: "empty", config: `qdisc {}`},
		{name: "include", config: `qdisc { device_include = "^(eth|bond).*" }`, include: "^(eth|bond).*"},
		{name: "exclude", config: `qdisc { device_exclude = "^(veth|cali).*" }`, exclude: "^(veth|cali).*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.config), &args))
			require.Equal(t, tc.include, args.Qdisc.DeviceInclude)
			require.Equal(t, tc.exclude, args.Qdisc.DeviceExclude)
			converted := args.Convert()
			require.Equal(t, tc.include, converted.QdiscDeviceInclude)
			require.Equal(t, tc.exclude, converted.QdiscDeviceExclude)
			require.NotContains(t, converted.EnableCollectors, "qdisc")
		})
	}
}
