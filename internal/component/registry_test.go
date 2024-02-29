package component

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseComponentName(t *testing.T) {
	tt := []struct {
		check       string
		expectValid bool
	}{
		{check: "", expectValid: false},
		{check: " ", expectValid: false},
		{check: "test", expectValid: true},
		{check: "foo.bar", expectValid: true},
		{check: "foo.bar.", expectValid: false},
		{check: "foo.bar. ", expectValid: false},
		{check: "small.LARGE", expectValid: true},
		{check: "a_b_c_012345", expectValid: true},
	}

	for _, tc := range tt {
		t.Run(tc.check, func(t *testing.T) {
			_, err := parseComponentName(tc.check)
			if tc.expectValid {
				require.NoError(t, err, "expected component name to be valid")
			} else {
				require.Error(t, err, "expected component name to not be valid")
			}
		})
	}
}

func Test_validatePrefixMatch(t *testing.T) {
	existing := map[string]parsedName{
		"remote.http":     {"remote", "http"},
		"test":            {"test"},
		"three.part.name": {"three", "part", "name"},
	}

	tt := []struct {
		check       string
		expectValid bool
	}{
		{check: "remote.s3", expectValid: true},
		{check: "remote", expectValid: false},
		{check: "test2", expectValid: true},
		{check: "test.new", expectValid: false},
		{check: "remote.something.else", expectValid: true},
		{check: "three.part", expectValid: false},
		{check: "three", expectValid: false},
		{check: "three.two", expectValid: true},
		{check: "three.part.other", expectValid: true},
	}

	for _, tc := range tt {
		t.Run(tc.check, func(t *testing.T) {
			parsed, err := parseComponentName(tc.check)
			require.NoError(t, err)

			err = validatePrefixMatch(parsed, existing)
			if tc.expectValid {
				require.NoError(t, err, "expected component to be accepted")
			} else {
				require.Error(t, err, "expected component to not be accepted")
			}
		})
	}
}
