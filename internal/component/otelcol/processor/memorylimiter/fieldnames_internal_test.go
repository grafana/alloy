package memorylimiter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// Rounding down in Validate should keep upstream's limit_mib rules unreachable, so
// nothing else exercises this backstop.
func TestAlloyFieldNames(t *testing.T) {
	require.NoError(t, alloyFieldNames(nil))

	for _, tc := range []struct{ in, want string }{
		{"'limit_mib' or 'limit_percentage' must be greater than zero", "'limit' or 'limit_percentage' must be greater than zero"},
		{"'spike_limit_mib' must be smaller than 'limit_mib'", "'spike_limit' must be smaller than 'limit'"},
		{"'check_interval' must be greater than zero", "'check_interval' must be greater than zero"},
	} {
		require.EqualError(t, alloyFieldNames(errors.New(tc.in)), tc.want)
	}
}
