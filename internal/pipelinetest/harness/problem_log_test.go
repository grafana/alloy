package harness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProblemLog(t *testing.T) {
	p := newProblemLog(2)
	write := func(s string) {
		_, err := p.Write([]byte(s))
		require.NoError(t, err)
	}

	write("ts=2026-01-01T00:00:00Z level=info msg=started\n")
	// level=error inside a value is not the line's level.
	write("ts=2026-01-01T00:00:00Z level=info entry=\"ts=x level=error msg=y\"\n")
	write("ts=2026-01-01T00:00:01Z level=error msg=\"stat failed\" path=a.log\n")
	// The same error again, at a later time, is a duplicate.
	write("ts=2026-01-01T00:00:02Z level=error msg=\"stat failed\" path=a.log\n")
	// A line can arrive in two writes.
	write("ts=2026-01-01T00:00:03Z level=warn ")
	write("msg=slow\n")
	write("ts=2026-01-01T00:00:04Z level=error msg=third\n")
	write("ts=2026-01-01T00:00:05Z level=error msg=fourth\n")

	require.Equal(t, []string{
		`level=error msg="stat failed" path=a.log`,
		`level=warn msg=slow`,
		`... and 2 more`,
	}, p.lines())
}
