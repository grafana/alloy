package subset

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssert(t *testing.T) {
	tt := []struct {
		name           string
		source, target string
		expect         string
	}{
		// Plain values
		{
			name:   "values match",
			source: `true`,
			target: `true`,
			expect: "",
		},
		{
			name:   "values mismatch",
			source: `true`,
			target: `false`,
			expect: "true != false",
		},
		{
			name:   "type mismatch",
			source: `true`,
			target: `5`,
			expect: "type mismatch: bool != int",
		},

		// Arrays
		{
			name:   "arrays match",
			source: `[1, 2, 3]`,
			target: `[1, 2, 3]`,
			expect: "",
		},
		{
			name:   "arrays mismatch",
			source: `[1, 2, 3]`,
			target: `[1, 2, 4]`,
			expect: "element 2: 3 != 4",
		},
		{
			name:   "array element type mismatch",
			source: `[1, 2, 3]`,
			target: `[1, 2, true]`,
			expect: "element 2: type mismatch: int != bool",
		},

		// Maps
		{
			name:   "maps match",
			source: `{"hello": "world"}`,
			target: `{"hello": "world"}`,
			expect: "",
		},
		{
			name:   "maps mismatch",
			source: `{"hello": "world", "year": 2000}`,
			target: `{"hello": "world", "year": 2001}`,
			expect: "year: 2000 != 2001",
		},
		{
			name:   "maps subset",
			source: `{"hello": "world"}`,
			target: `{"hello": "world", "year": 2001}`,
			expect: "",
		},
		{
			name:   "maps type mismatch",
			source: `{"hello": "world", "year": 2000}`,
			target: `{"hello": "world", "year": "yes"}`,
			expect: "year: type mismatch: int != string",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := YAMLAssert([]byte(tc.source), []byte(tc.target))
			if tc.expect == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expect)
			}
		})
	}
}
