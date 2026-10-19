package otelcol_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
)

type upLimit struct {
	Max int `mapstructure:"max"`
}

type upItem struct {
	Name  string  `mapstructure:"name"`
	Limit upLimit `mapstructure:"limit"`
}

type upList struct {
	Items []upItem `mapstructure:"items"`
}

type upListFactory struct{}

func (upListFactory) CreateDefaultConfig() otelcomponent.Config { return &upList{} }

type alLimit struct {
	Max int `alloy:"max,attr,optional"`
}

func (l *alLimit) SetToDefault() { l.Max = 5 }

type alItem struct {
	Name  string  `alloy:"name,attr"`
	Limit alLimit `alloy:"limit,block,optional"`
}

type alList struct {
	Items []alItem `alloy:"item,block,optional"`
}

func TestConvertViaConfmap_BlocksInLists(t *testing.T) {
	var args alList
	require.NoError(t, syntax.Unmarshal([]byte(`
		item { name = "unwritten" }
		item {
			name = "defaults"
			limit {}
		}
		item {
			name = "set"
			limit { max = 7 }
		}`), &args))

	got, err := otelcol.ConvertViaConfmap(args, upListFactory{}, otelcol.WithConvertRename("item", "items", nil))
	require.NoError(t, err)
	require.Equal(t, &upList{Items: []upItem{
		{Name: "unwritten"},
		// A block written with only Alloy defaults keeps them.
		{Name: "defaults", Limit: upLimit{Max: 5}},
		{Name: "set", Limit: upLimit{Max: 7}},
	}}, got)
}
