package gce

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalRiver(t *testing.T) {
	var riverConfig = `
	project = "project"
	zone = "zone"
	filter = "filter"
	refresh_interval = "60s"
	port = 80
	tag_separator = ","
`

	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)
}

func TestUnmarshalRiverInvalid(t *testing.T) {
	var riverConfig = `
	filter = "filter"
	refresh_interval = "60s"
	port = 80
	tag_separator = ","
`

	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)

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

	sdConfig := args.Convert()
	require.Equal(t, args.Project, sdConfig.Project)
	require.Equal(t, args.Zone, sdConfig.Zone)
	require.Equal(t, args.Filter, sdConfig.Filter)
	require.Equal(t, args.RefreshInterval, time.Duration(sdConfig.RefreshInterval))
	require.Equal(t, args.Port, sdConfig.Port)
	require.Equal(t, args.TagSeparator, sdConfig.TagSeparator)
}
