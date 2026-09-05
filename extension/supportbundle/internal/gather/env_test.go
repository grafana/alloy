package gather

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvGathererAllowlist(t *testing.T) {
	// A default-allowlist variable and a config-defined extra are captured.
	// A variable that is neither is not.
	t.Setenv("GOMAXPROCS", "7")
	t.Setenv("SUPPORTBUNDLE_EXTRA", "extra-value")
	t.Setenv("SUPPORTBUNDLE_UNLISTED", "should-not-appear")

	g := Environment{Extra: []string{"SUPPORTBUNDLE_EXTRA"}}
	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "environment.txt")
	content := string(m["environment.txt"])
	require.Contains(t, content, "GOMAXPROCS=7")
	require.Contains(t, content, "SUPPORTBUNDLE_EXTRA=extra-value")
	require.NotContains(t, content, "SUPPORTBUNDLE_UNLISTED")
}

func TestMergeEnvNames(t *testing.T) {
	got := mergeEnvNames([]string{"B", "A"}, []string{"A", "C", ""})
	require.Equal(t, []string{"A", "B", "C"}, got)
}
