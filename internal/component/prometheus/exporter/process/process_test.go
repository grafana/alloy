package process

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/ncabatoff/process-exporter/config"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfigUnmarshal(t *testing.T) {
	var exampleAlloyConfig = `
	matcher {
		name    = "alloy"
		comm    = ["alloy"]
		cmdline = ["*run*"]
	}
	track_children    = false
	track_threads     = false
	gather_smaps      = true
	recheck_on_scrape = true
	remove_empty_groups = true
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	require.False(t, args.Children)
	require.False(t, args.Threads)
	require.True(t, args.SMaps)
	require.True(t, args.Recheck)
	require.True(t, args.RemoveEmptyGroups)

	expected := []MatcherGroup{
		{
			Name:         "alloy",
			CommRules:    []string{"alloy"},
			CmdlineRules: []string{"*run*"},
		},
	}
	require.Equal(t, expected, args.ProcessExporter)
}

func TestAlloyConfigConvert(t *testing.T) {
	var exampleAlloyConfig = `
	matcher {
		name    = "static"
		comm    = ["alloy"]
		cmdline = ["*config.file*"]
	}
	track_children    = true
	track_threads     = true
	gather_smaps      = false
	recheck_on_scrape = false
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	require.True(t, args.Children)
	require.True(t, args.Threads)
	require.False(t, args.SMaps)
	require.False(t, args.Recheck)

	expected := []MatcherGroup{
		{
			Name:         "static",
			CommRules:    []string{"alloy"},
			CmdlineRules: []string{"*config.file*"},
		},
	}
	require.Equal(t, expected, args.ProcessExporter)

	c := args.Convert()
	require.True(t, c.Children)
	require.True(t, c.Threads)
	require.False(t, c.SMaps)
	require.False(t, c.Recheck)
	require.False(t, c.RemoveEmptyGroups)

	e := config.MatcherRules{
		{
			Name:         "static",
			CommRules:    []string{"alloy"},
			CmdlineRules: []string{"*config.file*"},
		},
	}
	require.Equal(t, e, c.ProcessExporter)
}
