package kubetail

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	tests := []struct {
		name       string
		windowSize int
		minEntries int
		input      []int64
		expected   time.Duration
	}{
		{
			name:       "empty, expect default",
			windowSize: 10,
			input:      []int64{},
			expected:   time.Duration(10),
		},
		{
			name:       "one sample, not enough, expect default",
			windowSize: 5,
			input:      []int64{10},
			expected:   time.Duration(10),
		},
		{
			name:       "partially full",
			windowSize: 10,
			input:      []int64{10, 20, 30, 40, 50},
			expected:   time.Duration(10),
		},
		{
			name:       "completely full",
			windowSize: 5,
			input:      []int64{10, 20, 30, 40, 50, 60},
			expected:   time.Duration(10),
		},
		{
			name:       "rollover simple",
			windowSize: 5,
			input:      []int64{10, 20, 30, 40, 50, 60},
			expected:   time.Duration(10),
		},
		{
			name:       "rollover complex: make sure first value is ignored",
			windowSize: 5,
			input:      []int64{0, 40, 50, 60, 70, 80, 90},
			expected:   time.Duration(10),
		},
		{
			name:       "complex",
			windowSize: 5,
			//                    40 +1  +4  +45  +5  = 95, 95/5 = 19
			input:    []int64{10, 50, 51, 55, 100, 105},
			expected: time.Duration(19),
		},
		{
			name:       "complex 2",
			windowSize: 10,
			//                  outside of window |
			//                    40 +1  +4  +45 +|5   +5   +90  +100 +150 +300 +5   +45  +50  +149 = 899
			input:    []int64{10, 50, 51, 55, 100, 105, 110, 200, 300, 450, 750, 755, 800, 850, 999},
			expected: time.Duration(89), // Integer math result is truncated not rounded.
		},
		{
			name:       "below min duration",
			windowSize: 5,
			input:      []int64{0, 1, 2, 3, 4, 5, 6},
			expected:   time.Duration(2),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := newRollingAverageCalculator(test.windowSize, test.minEntries, time.Duration(2), time.Duration(10))
			for _, v := range test.input {
				c.AddTimestamp(time.Unix(0, v))
			}
			avg := c.GetAverage()
			assert.Equal(t, test.expected, avg)
		})
	}
}
