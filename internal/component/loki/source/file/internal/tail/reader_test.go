package tail

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/alecthomas/units"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/grafana/alloy/internal/runtime/logging"
)

var benchText string

func BenchmarkReader(b *testing.B) {
	type testCase struct {
		name     string
		lineSize int
		enc      encoding.Encoding
	}

	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	utf16be := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)

	tests := []testCase{
		{name: "utf8 100B", enc: encoding.Nop, lineSize: 100},
		{name: "utf8 5KB", enc: encoding.Nop, lineSize: int(5 * units.KiB)},
		{name: "utf8 1MB", enc: encoding.Nop, lineSize: int(1 * units.MiB)},
		{name: "utf8 8MB", enc: encoding.Nop, lineSize: int(8 * units.MiB)},
		{name: "utf16le 100B", enc: utf16le, lineSize: 100},
		{name: "utf16le 5KB", enc: utf16le, lineSize: int(5 * units.KiB)},
		{name: "utf16le 1MB", enc: utf16le, lineSize: int(1 * units.MiB)},
		{name: "utf16le 8MB", enc: utf16le, lineSize: int(8 * units.MiB)},
		{name: "utf16be 100B", enc: utf16be, lineSize: 100},
		{name: "utf16be 5KB", enc: utf16be, lineSize: int(5 * units.KiB)},
		{name: "utf16be 1MB", enc: utf16be, lineSize: int(1 * units.MiB)},
		{name: "utf16be 8MB", enc: utf16be, lineSize: int(8 * units.MiB)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			line := strings.Repeat("a", tt.lineSize-1) + "\n"
			content, err := tt.enc.NewEncoder().String(line)
			require.NoError(b, err)
			reader := strings.NewReader(content)

			r, err := newReader(logging.NewSlogNop(), reader, 0, tt.enc, "", false)
			require.NoError(b, err)

			b.ReportAllocs()
			for b.Loop() {
				for {
					benchText, err = r.next()
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(b, err)
				}
				require.NoError(b, r.reset(reader, 0))
			}
		})
	}
}
