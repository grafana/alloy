package common_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/stretchr/testify/require"
)

func TestOptionalSecret_Write(t *testing.T) {
	tt := []struct {
		name   string
		value  common.ConvertTargets
		expect string
	}{
		{
			name: "nil",
			value: common.ConvertTargets{
				Targets: nil,
			},
			expect: `[]`,
		},
		{
			name: "empty",
			value: common.ConvertTargets{
				Targets: []discovery.Target{{}},
			},
			expect: ``,
		},
		{
			name: "__address__ key",
			value: common.ConvertTargets{
				Targets: []discovery.Target{{"__address__": "testing"}},
			},
			expect: `[{
	__address__ = "testing",
}]`,
		},
		{
			name: "__address__ key label",
			value: common.ConvertTargets{
				Targets: []discovery.Target{{"__address__": "testing", "label": "value"}},
			},
			expect: `[{
	__address__ = "testing",
	label       = "value",
}]`,
		},
		{
			name: "multiple __address__ key label",
			value: common.ConvertTargets{
				Targets: []discovery.Target{
					{"__address__": "testing", "label": "value"},
					{"__address__": "testing2", "label": "value"},
				},
			},
			expect: `array.concat(
	[{
		__address__ = "testing",
		label       = "value",
	}],
	[{
		__address__ = "testing2",
		label       = "value",
	}],
)`,
		},
		{
			name: "__expr__ key",
			value: common.ConvertTargets{
				Targets: []discovery.Target{{"__expr__": "testing"}},
			},
			expect: `testing`,
		},
		{
			name: "multiple __expr__ key",
			value: common.ConvertTargets{
				Targets: []discovery.Target{{"__expr__": "testing"}, {"__expr__": "testing2"}},
			},
			expect: `array.concat(
	testing,
	testing2,
)`,
		},
		{
			name: "both key types",
			value: common.ConvertTargets{
				Targets: []discovery.Target{{"__address__": "testing", "label": "value"}, {"__expr__": "testing2"}},
			},
			expect: `array.concat(
	[{
		__address__ = "testing",
		label       = "value",
	}],
	testing2,
)`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			be := builder.NewExpr()
			be.SetValue(tc.value)
			require.Equal(t, tc.expect, string(be.Bytes()))
		})
	}
}
