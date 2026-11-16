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

func TestReader(t *testing.T) {
	t.Run("releases large pending buffer after line", func(t *testing.T) {
		large := strings.Repeat("a", maxRetainedBufSize+1) + "\n"

		r, err := newReader(logging.NewSlogNop(), strings.NewReader(large), 0, 0, encoding.Nop, "", false)
		require.NoError(t, err)

		line, err := r.next()
		require.NoError(t, err)
		require.Len(t, line, maxRetainedBufSize+1)
		require.LessOrEqual(t, cap(r.pending), maxRetainedBufSize)
	})

	t.Run("releases large pending buffer on flush", func(t *testing.T) {
		large := strings.Repeat("a", maxRetainedBufSize+1)

		r, err := newReader(logging.NewSlogNop(), strings.NewReader(large), 0, 0, encoding.Nop, "", false)
		require.NoError(t, err)

		_, err = r.next()
		require.ErrorIs(t, err, io.EOF)
		require.Greater(t, cap(r.pending), maxRetainedBufSize)

		line, err := r.flush()
		require.NoError(t, err)
		require.Len(t, line, maxRetainedBufSize+1)
		require.LessOrEqual(t, cap(r.pending), maxRetainedBufSize)
	})

	t.Run("max line size with CRLF split across buffer boundary", func(t *testing.T) {
		// The line is sized so the buffer fills right after the CR.
		line := strings.Repeat("a", defaultBufSize-1)

		r, err := newReader(logging.NewSlogNop(), strings.NewReader(line+"\r\n"), 0, len(line), encoding.Nop, "", false)
		require.NoError(t, err)

		got, err := r.next()
		require.NoError(t, err)
		require.Equal(t, line, got)

		_, err = r.next()
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("max line size does not split utf16 code units", func(t *testing.T) {
		enc := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
		content, err := enc.NewEncoder().String("aaaaa\n")
		require.NoError(t, err)

		// An odd max line size lands inside a 2-byte UTF-16 code unit.
		r, err := newReader(logging.NewSlogNop(), strings.NewReader(content), 0, 5, enc, "", false)
		require.NoError(t, err)

		var lines []string
		for {
			line, err := r.next()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			lines = append(lines, line)
		}

		require.Equal(t, "aaaaa", strings.Join(lines, ""))
	})
}

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

			r, err := newReader(logging.NewSlogNop(), reader, 0, 0, tt.enc, "", false)
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
