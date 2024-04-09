package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_normalizeVersion(t *testing.T) {
	tt := []struct {
		input  string
		expect string
	}{
		{"", "v0.0.0"},
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},
		{"1.2.3+SHA", "v1.2.3+SHA"},
		{"v1.2.3+SHA", "v1.2.3+SHA"},
		{"1.2.3-rc.1", "v1.2.3-rc.1"},
		{"v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"1.2.3-rc.1+SHA", "v1.2.3-rc.1+SHA"},
		{"v1.2.3-rc.1+SHA", "v1.2.3-rc.1+SHA"},
		{"not_semver", "not_semver"},
	}

	for _, tc := range tt {
		actual := normalizeVersion(tc.input)
		assert.Equal(t, tc.expect, actual,
			"Expected %q to normalize to %q, got %q",
			tc.input, tc.expect, actual,
		)
	}
}
