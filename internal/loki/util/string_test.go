package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSnakeCase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		input, expected string
	}{
		{
			name:     "simple",
			input:    "snakeCase",
			expected: "snake_case",
		},
		{
			name:     "mix",
			input:    "Snake_Case",
			expected: "snake_case", // should be snake__case??
		},
		{
			name:     "begin-with-underscore",
			input:    "_Snake_Case",
			expected: "_snake_case",
		},
		{
			name:     "end-with-underscore",
			input:    "Snake_Case_",
			expected: "snake_case_",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SnakeCase(c.input)
			assert.Equal(t, c.expected, got)
		})
	}
}
