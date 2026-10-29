package convert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLabel(t *testing.T) {
	tt := []struct{ in, want string }{
		{"http.method", "http_method"},
		{"1abc", "key_1abc"},
		{"_private", "_private"},
		{"a__b", "a__b"},
		{"!!!", "___"},
		{"", ""},
	}
	for _, tc := range tt {
		require.Equal(t, tc.want, normalizeLabel(tc.in), tc.in)
	}
}
