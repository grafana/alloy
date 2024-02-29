package process

import (
	"testing"

	"github.com/grafana/river"
	"github.com/ncabatoff/process-exporter/config"
	"github.com/stretchr/testify/require"
)

func TestRiverConfigUnmarshal(t *testing.T) {
	var exampleRiverConfig = `
	matcher {
		name    = "flow"
		comm    = ["grafana-agent"]
		cmdline = ["*run*"]
	}
	track_children    = false
	track_threads     = false
	gather_smaps      = true
	recheck_on_scrape = true
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	require.False(t, args.Children)
	require.False(t, args.Threads)
	require.True(t, args.SMaps)
	require.True(t, args.Recheck)

	expected := []MatcherGroup{
		{
			Name:         "flow",
			CommRules:    []string{"grafana-agent"},
			CmdlineRules: []string{"*run*"},
		},
	}
	require.Equal(t, expected, args.ProcessExporter)
}

func TestRiverConfigConvert(t *testing.T) {
	var exampleRiverConfig = `
	matcher {
		name    = "static"
		comm    = ["grafana-agent"]
		cmdline = ["*config.file*"]
	}
	track_children    = true
	track_threads     = true
	gather_smaps      = false
	recheck_on_scrape = false
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	require.True(t, args.Children)
	require.True(t, args.Threads)
	require.False(t, args.SMaps)
	require.False(t, args.Recheck)

	expected := []MatcherGroup{
		{
			Name:         "static",
			CommRules:    []string{"grafana-agent"},
			CmdlineRules: []string{"*config.file*"},
		},
	}
	require.Equal(t, expected, args.ProcessExporter)

	c := args.Convert()
	require.True(t, c.Children)
	require.True(t, c.Threads)
	require.False(t, c.SMaps)
	require.False(t, c.Recheck)

	e := config.MatcherRules{
		{
			Name:         "static",
			CommRules:    []string{"grafana-agent"},
			CmdlineRules: []string{"*config.file*"},
		},
	}
	require.Equal(t, e, c.ProcessExporter)
}
