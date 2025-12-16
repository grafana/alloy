package syslogparser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
	"strconv"
	"unicode"

	"github.com/leodido/go-syslog/v4"
)

// IterStreamRaw returns an iterator to read syslog lines from a stream without contents parsing.
//
// Delimiter argument is used to determine line end for non-transparent framing.
func IterStreamRaw(r io.Reader, delimiter byte) iter.Seq2[*syslog.Base, error] {
	return func(yield func(*syslog.Base, error) bool) {
		buf := bufio.NewReaderSize(r, 1<<10)
		for {
			r, err := parseLineRaw(buf, delimiter)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					yield(nil, err)
				}

				return
			}

			if !yield(r, nil) {
				return
			}
		}
	}
}

// ReadLineRaw reads a single syslog line without syslog contents parsing.
//
// Delimiter argument is used to determine line end for non-transparent framing.
// Returns a nil value if line contents are empty.
//
// Meant to be used in UDP transport where a single syslog line is delivered per datagram.
func ReadLineRaw(r io.Reader, delimiter byte) (*syslog.Base, error) {
	buf := bufio.NewReaderSize(r, 1<<10)
	return parseLineRaw(buf, delimiter)
}

func parseLineRaw(buf *bufio.Reader, delimiter byte) (*syslog.Base, error) {
	b, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	// TODO: use bytebufferpool?
	_ = buf.UnreadByte()
	switch framingTypeFromFirstByte(b) {
	case framingTypeNonTransparent:
		buff, err := buf.ReadBytes(delimiter)
		if err != nil {
			// Ignore io.EOF if some data was returned
			if !errors.Is(err, io.EOF) || len(buff) == 0 {
				return nil, err
			}
		}

		if len(buff) == 0 {
			return nil, nil
		}

		// trim potential newline leftovers if called sequentially inside TCP conn.
		buff = bytes.TrimFunc(buff, unicode.IsSpace)
		return readLogLine(buff), nil

	case framingTypeOctetCounting:
		contentLength, err := readFrameLength(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read octet length header: %w", err)
		}

		// Skip space after octet length. Octet counting refers to length after a space.
		_, _ = buf.ReadByte()
		buff := make([]byte, contentLength)
		n, err := buf.Read(buff)
		if err != nil {
			return nil, fmt.Errorf("cannot read message: %w (length: %d)", err, contentLength)
		}

		if n == 0 {
			return nil, fmt.Errorf("empty buffer returned (expected: %d)", contentLength)
		}

		buff = buff[:n]
		return readLogLine(buff), nil
	default:
		return nil, fmt.Errorf("invalid or unsupported framing. first byte: %q", b)
	}
}

func readLogLine(line []byte) *syslog.Base {
	out := &syslog.Base{}
	line = readSeverity(line, out)

	msg := string(bytes.TrimSpace(line))
	out.Message = &msg
	return out
}

func readSeverity(line []byte, dst *syslog.Base) (next []byte) {
	// priority has to be in format '<0-9+>'
	if len(line) < 3 || line[0] != '<' {
		return line
	}

	buff := line[1:]
	priority := uint(0)
	for i, v := range buff {
		if v == '>' {
			if i == 0 || priority > 255 {
				return line
			}

			severity := uint8(priority % 8)
			dst.Severity = &severity
			dst.Priority = new(uint8)
			*dst.Priority = uint8(priority)

			buff = buff[i+1:]
			return buff
		}

		if !isDigit(v) {
			return line
		}

		priority *= 10
		priority += uint(v - '0')
	}

	return line
}

func readFrameLength(r *bufio.Reader) (flen int, err error) {
	// log lines with octet counted framing start with length.
	// Example: `114 <34>1 2025-01-03T14:07:15.003Z message...`
	part, err := r.ReadString(' ')
	if err != nil {
		return 0, fmt.Errorf("%w (read: %q)", err, part)
	}

	c, err := strconv.ParseInt(part, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse octet length from %q: %w", part, err)
	}

	return int(c), nil
}
