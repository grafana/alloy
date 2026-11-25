package alloycli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithSystemFallback(t *testing.T) {
	noLimit := func() (uint64, error) { return 0, errors.New("no cgroup limit") }
	cgroupLimit := func() (uint64, error) { return 1234, nil }

	for _, tc := range []struct {
		name        string
		base        func() (uint64, error)
		experiments string
		wantLimit   uint64 // 0 means an error is expected
	}{
		{"no experiment, no limit", noLimit, "", 0},
		{"other experiment, no limit", noLimit, "other", 0},
		{"system experiment, no limit", noLimit, "other,system", 1},
		{"system experiment, cgroup limit wins", cgroupLimit, "system", 1234},
		{"no experiment, cgroup limit", cgroupLimit, "", 1234},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := withSystemFallback(tc.base, tc.experiments)()
			if tc.wantLimit == 0 {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.wantLimit == 1 {
				require.NotZero(t, got) // The system memory size varies per host.
				return
			}
			require.Equal(t, tc.wantLimit, got)
		})
	}
}
