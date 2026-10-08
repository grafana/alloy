package benchdiff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommentBody(t *testing.T) {
	const (
		sha    = "0123456789abcdef"
		runURL = "https://github.com/grafana/alloy/actions/runs/1"
	)

	running := commentBody(false, "", sha, runURL)
	require.True(t, strings.HasPrefix(running, Marker))
	require.Contains(t, running, "⏳ Benchmarking `0123456789`")
	require.Contains(t, running, runURL)

	failed := commentBody(true, "", sha, runURL)
	require.True(t, strings.HasPrefix(failed, Marker))
	require.Contains(t, failed, "❌ The benchmark run for `0123456789` failed")

	done := commentBody(true, Marker+"\n## Benchmark report\n\nresults\n", sha, runURL)
	require.True(t, strings.HasPrefix(done, Marker))
	require.Contains(t, done, "results")
	require.Contains(t, done, "[Workflow run]("+runURL+") for `0123456789`")
}
