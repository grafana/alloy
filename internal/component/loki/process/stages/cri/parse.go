package cri

import (
	"strings"
	"unicode"
)

type Flag int8

const (
	FlagFull Flag = iota + 1
	FlagPartial
)

func (f Flag) String() string {
	switch f {
	case FlagFull:
		return "F"
	case FlagPartial:
		return "P"
	default:
		return ""
	}
}

type Stream int8

const (
	StreamStdOut Stream = iota + 1
	StreamStdErr
	StreamUnknown
)

func (s Stream) String() string {
	switch s {
	case StreamStdOut:
		return "stdout"
	case StreamStdErr:
		return "stderr"
	default:
		return ""
	}
}

// FIXME: avoid unnessisary allocation.

type Parsed struct {
	// Maybe borrowed string??
	Timestamp string
	Stream    Stream
	Flag      Flag

	// Maybe borrowed string??
	Content string
}

func (p Parsed) Valid() bool {
	return p.Stream != StreamUnknown
}

func ParseCRI(line string) Parsed {
	var parsed Parsed

	parsed.Timestamp, line = parseTimestamp(line)
	parsed.Stream, line = parseStream(line)
	parsed.Flag, line = parseFlag(line)
	parsed.Content = line

	return parsed
}

func parseTimestamp(line string) (string, string) {
	i := strings.IndexFunc(line, unicode.IsSpace)
	if i == -1 {
		return "", line
	}
	return line[0:i], skipWhitespace(line[i:])
}

func parseStream(line string) (Stream, string) {
	stream := StreamUnknown

	// Optimize this!!
	if strings.HasPrefix(line, "stdout") {
		stream, line = StreamStdOut, line[len("stdout"):]
	} else if strings.HasPrefix(line, "stderr") {
		stream, line = StreamStdErr, line[len("stderr"):]
	}

	return stream, skipWhitespace(line)
}

func parseFlag(line string) (Flag, string) {
	if len(line) == 0 {
		return FlagFull, line
	}

	var (
		b    = line[0]
		flag = FlagFull
	)

	switch b {
	case 'P':
		flag = FlagPartial
		line = line[1:]
	case 'F':
		line = line[1:]
	}

	return flag, skipWhitespace(line)
}

func skipWhitespace(line string) string {
	// Iterate over runes starting at byte index i
	for i, r := range line {
		if !unicode.IsSpace(r) {
			return line[i:]
		}
	}

	// Only whitespace until end
	return line
}
