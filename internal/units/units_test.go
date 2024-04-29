package units_test

import (
	"errors"
	"strings"
	"testing"

	oldunits "github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	text  string
	bytes units.Bytes
}{
	{"0", 0},
	{"512", 512},

	{"1kB", 1 * units.Kilobyte},
	{"2MB", 2 * units.Megabyte},
	{"3GB", 3 * units.Gigabyte},
	{"4TB", 4 * units.Terabyte},
	{"5PB", 5 * units.Petabyte},
	{"6EB", 6 * units.Exabyte},

	{"7KiB", 7 * units.Kibibyte},
	{"8MiB", 8 * units.Mebibyte},
	{"9GiB", 9 * units.Gibibyte},
	{"10TiB", 10 * units.Tebibyte},
	{"11PiB", 11 * units.Pebibyte},
	{"7EiB", 7 * units.Exbibyte}, // 7EiB is the highest we can go without overflowing int64
}

// Fuzz_UnmarshalText ensures that nothing panics when calling UnmarshalText.
func Fuzz_UnmarshalText(f *testing.F) {
	f.Add("0")
	f.Add("512")
	f.Add("7KiB")
	f.Add("8MiB")
	f.Add("9GiB")
	f.Add("10TiB")
	f.Add("11PiB")
	f.Add("7EiB")
	f.Add("-3MiB")
	f.Add("3MiB4KiB5B")

	f.Fuzz(func(t *testing.T, text string) {
		var bytes units.Bytes
		_ = bytes.UnmarshalText([]byte(text))
	})
}

// Fuzz_String ensures that nothing panics when calling String.
func Fuzz_String(f *testing.F) {
	for _, tc := range testCases {
		f.Add(int64(tc.bytes))
	}

	f.Fuzz(func(t *testing.T, bytes int64) {
		_ = units.Bytes(bytes).String()
	})
}

// Fuzz_Compare ensures that anything parseable by oldunits.UnmarshalText can be parsed by units.UnmarshalText.
func Fuzz_Compare(f *testing.F) {
	for _, tc := range testCases {
		f.Add(tc.text)
	}

	// Ensure that spaces are not permitted in oldunits, since they're not
	// permitted here either.
	f.Add("4 MiB")
	f.Add("4MiB 2KiB")

	f.Fuzz(func(t *testing.T, text string) {
		if strings.Contains(text, ".") {
			t.Skip()
		}

		var oldType oldunits.Base2Bytes
		if err := oldType.UnmarshalText([]byte(text)); err != nil {
			t.Skip()
		}

		var newType units.Bytes
		err := newType.UnmarshalText([]byte(text))
		if errors.Is(err, units.ErrOverflow) {
			t.Skip()
		}

		require.NoError(t, err, "%q should not have failed to parse", text)
	})
}

func TestBytes_UnmarshalText(t *testing.T) {
	for _, tc := range testCases {
		expect := tc.bytes

		var actual units.Bytes
		if assert.NoError(t, actual.UnmarshalText([]byte(tc.text)), "Parsing %q should not have failed", tc.text) {
			assert.Equal(t, tc.bytes, actual, "Parsing %q should have resulted in %d, but got %d", tc.text, expect, actual)
		}
	}

	t.Run("Multiple units", func(t *testing.T) {
		in := "4MB3kB100B"
		expect := (4 * units.Megabyte) + (3 * units.Kilobyte) + (100 * units.Byte)

		var actual units.Bytes
		require.NoError(t, actual.UnmarshalText([]byte(in)), "%q should not have failed to parse", in)
		require.Equal(t, expect, actual, "%q did not parse correctly", in)
	})

	t.Run("KB and kB are equivalent", func(t *testing.T) {
		strings := []string{"15KB", "15kB"}
		expect := 15 * units.Kilobyte

		for _, str := range strings {
			var actual units.Bytes
			if assert.NoError(t, actual.UnmarshalText([]byte(str)), "Parsing %q should not have failed", str) {
				assert.Equal(t, expect, actual, "Parsing %q should have resulted in %d, but got %d", str, expect, actual)
			}
		}
	})

	t.Run("Leading positive sign", func(t *testing.T) {
		for _, tc := range testCases {
			text := "+" + tc.text
			expect := tc.bytes

			var actual units.Bytes
			if assert.NoError(t, actual.UnmarshalText([]byte(text)), "Parsing %q should not have failed", text) {
				assert.Equal(t, tc.bytes, actual, "Parsing %q should have resulted in %d, but got %d", text, expect, actual)
			}
		}
	})

	t.Run("Leading negative sign", func(t *testing.T) {
		for _, tc := range testCases {
			text := "-" + tc.text
			expect := tc.bytes

			var actual units.Bytes
			if assert.NoError(t, actual.UnmarshalText([]byte(text)), "Parsing %q should not have failed", text) {
				assert.Equal(t, -tc.bytes, actual, "Parsing %q should have resulted in %d, but got %d", text, -expect, actual)
			}
		}
	})

	t.Run("Detect overflows", func(t *testing.T) {
		var actual units.Bytes
		err := actual.UnmarshalText([]byte("10000EB"))
		require.ErrorIs(t, err, units.ErrOverflow, "Parsing 10000EB should have resulted in an overflow error")
	})

	t.Run("Detect overflows of sequence", func(t *testing.T) {
		var actual units.Bytes
		err := actual.UnmarshalText([]byte("7EB5EB"))
		require.ErrorIs(t, err, units.ErrOverflow, "Parsing 7EB5EB should have resulted in an overflow error")
	})
}

func splitNumberSuffix(in string) (string, string) {
	suffixOffSet := len(in)

Loop:
	for i := 0; i < len(in); i++ {
		switch {
		case i == 0 && (in[i] == '-' || in[i] == '+'):
			suffixOffSet = i + 1
		case '0' <= in[i] && in[i] <= '9':
			suffixOffSet = i + 1
		default:
			break Loop
		}
	}

	return in[:suffixOffSet], in[suffixOffSet:]
}

func TestBytes_String(t *testing.T) {
	for _, tc := range testCases {
		expect := tc.text
		actual := tc.bytes.String()
		assert.Equal(t, expect, actual, "String() should have returned %q, but got %q", expect, actual)
	}

	t.Run("Negative numbers", func(t *testing.T) {
		for _, tc := range testCases {
			// Ignore 0, as -0 gets simplified to 0.
			if tc.text == "0" {
				continue
			}

			expect := "-" + tc.text
			actual := (-tc.bytes).String()
			assert.Equal(t, expect, actual, "String() should have returned %q, but got %q", expect, actual)
		}
	})
}
