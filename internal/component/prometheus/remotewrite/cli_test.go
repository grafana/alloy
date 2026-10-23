package remotewrite

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTargetTableIsASCII checks that the WAL stats table keeps the plain
// ASCII borders it had before tablewriter v1.
func TestTargetTableIsASCII(t *testing.T) {
	var buf bytes.Buffer
	table := newTargetTable(&buf)
	require.NoError(t, table.Append("job", "instance", "1", "2"))
	require.NoError(t, table.Render())

	out := buf.String()
	require.Contains(t, out, "+-")
	require.Contains(t, out, "| job")
	for _, r := range out {
		require.Less(t, r, rune(128), "the table must use only ASCII: %q", out)
	}
}
