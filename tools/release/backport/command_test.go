package backport

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/tools/release/internal/git"
)

func TestAppendOriginalAuthorTrailer(t *testing.T) {
	message := "fix: Backport change\n\n(cherry picked from commit abc1234)"
	author := git.CommitAuthor{Name: "Octo Cat", Email: "octocat@example.com"}

	got := appendOriginalAuthorTrailer(message, author)

	require.Equal(t, "fix: Backport change\n\n(cherry picked from commit abc1234)\n\nCo-authored-by: Octo Cat <octocat@example.com>", got)
}

func TestAppendOriginalAuthorTrailerUsesExactAuthorIdentity(t *testing.T) {
	message := "fix: Backport change\n\n(cherry picked from commit abc1234)"
	author := git.CommitAuthor{Name: "grafana-alloybot[bot]", Email: "bot@example.com"}

	got := appendOriginalAuthorTrailer(message, author)

	require.Equal(t, "fix: Backport change\n\n(cherry picked from commit abc1234)\n\nCo-authored-by: grafana-alloybot[bot] <bot@example.com>", got)
}
