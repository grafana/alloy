package gce

import (
	"testing"
	"time"

	"github.com/prometheus/prometheus/discovery/gce"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestUnmarshalAlloy(t *testing.T) {
	var alloyConfig = `
	project = "project"
	zone = "zone"
	filter = "filter"
	refresh_interval = "60s"
	port = 80
	tag_separator = ","
`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)
}

func TestUnmarshalAlloyInvalid(t *testing.T) {
	var alloyConfig = `
	filter = "filter"
	refresh_interval = "60s"
	port = 80
	tag_separator = ","
`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)

	// Validate that project and zone are required.
	require.Error(t, err)
}

func TestConvert(t *testing.T) {
	args := Arguments{
		Project:         "project",
		Zone:            "zone",
		Filter:          "filter",
		RefreshInterval: 10 * time.Second,
		Port:            81,
		TagSeparator:    ",",
	}

	sdConfig := args.Convert().(*gce.SDConfig)
	require.Equal(t, args.Project, sdConfig.Project)
	require.Equal(t, args.Zone, sdConfig.Zone)
	require.Equal(t, args.Filter, sdConfig.Filter)
	require.Equal(t, args.RefreshInterval, time.Duration(sdConfig.RefreshInterval))
	require.Equal(t, args.Port, sdConfig.Port)
	require.Equal(t, args.TagSeparator, sdConfig.TagSeparator)
}
