package cri

import (
	"strings"
	"unicode"
)

type Flag int8

const (
	// FlagFull indicates a full log line.
	FlagFull Flag = iota
	// FlagPartial indicates a partial log line.
	FlagPartial
)

// String returns the CRI flag representation ("F" or "P").
func (f Flag) String() string {
	switch f {
	case FlagPartial:
		return "P"
	default:
		return "F"
	}
}

type Stream int8

const (
	// StreamStdOut indicates stdout log stream.
	StreamStdOut = iota
	// StreamStdErr indicates stderr log stream.
	StreamStdErr
)

// String returns the CRI stream representation ("stdout" or "stderr").
func (s Stream) String() string {
	switch s {
	case StreamStdErr:
		return "stderr"
	default:
		return "stdout"
	}
}

type Parsed struct {
	// Timestamp is the raw CRI timestamp field.
	Timestamp string
	// Stream is the CRI stream field (stdout/stderr).
	Stream Stream
	// Flag is the CRI flag field (F/P).
	Flag Flag
	// Content is the log content after the CRI header.
	Content string
}

// ParseCRI parses a CRI formatted log line, <TIMESTAMP> <STREAM> <FLAGS> <CONTENT>.
// To be compatible with previous implementation using regex we are a bit lenient
// with the format:
// 1. We don't validate that the timestamp is RFC 3339Nano.
// 2. FLAGS are optional and we assume 'F' if not set.
func ParseCRI(line string) (Parsed, bool) {
	var (
		timestamp string
		stream    Stream
		flag      Flag
		zero      Parsed
	)

	timestamp, line = parseTimestamp(line)
	line, ok := skipWhitespace(line)
	if !ok {
		return zero, false
	}

	stream, line, ok = parseStream(line)
	if !ok {
		return zero, false
	}

	line, ok = skipWhitespace(line)
	if !ok {
		return zero, false
	}

	// NOTE: because we only optionally require flag we also don't care if we have
	// a whitespace or not.
	flag, line = parseFlag(line)
	line, _ = skipWhitespace(line)
	return Parsed{
		Timestamp: timestamp,
		Stream:    stream,
		Flag:      flag,
		Content:   line,
	}, true
}

func parseTimestamp(line string) (string, string) {
	i := strings.IndexFunc(line, unicode.IsSpace)
	if i == -1 {
		return "", line
	}
	return line[0:i], line[i:]
}

func parseStream(line string) (Stream, string, bool) {
	if strings.HasPrefix(line, "stdout") {
		return StreamStdOut, line[len("stdout"):], true
	} else if strings.HasPrefix(line, "stderr") {
		return StreamStdErr, line[len("stderr"):], true
	}
	return StreamStdOut, line, false
}

func parseFlag(line string) (Flag, string) {
	if len(line) == 0 {
		return FlagFull, line
	}

	switch line[0] {
	case 'P':
		return FlagPartial, line[1:]
	case 'F':
		return FlagFull, line[1:]
	}

	return FlagFull, line
}

func skipWhitespace(line string) (string, bool) {
	if len(line) > 0 && line[0] == ' ' {
		return line[1:], true
	}
	return line, false
}
