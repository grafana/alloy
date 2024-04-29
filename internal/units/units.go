// Package units provides functionality for parsing and displaying multiples of
// bytes.
package units

import (
	"encoding"
	"errors"
	"fmt"
	"strconv"
)

var (
	// ErrInvalidSyntax is returned when a byte string cannot be parsed.
	ErrInvalidSyntax = errors.New("invalid syntax")

	// ErrOverflow is returned when a byte string is too large to be represented.
	ErrOverflow = errors.New("byte size overflows int64")
)

type Bytes int64

var (
	_ encoding.TextUnmarshaler = (*Bytes)(nil)
	_ encoding.TextMarshaler   = Bytes(0)
)

const (
	Byte Bytes = 1

	Kilobyte = 1000 * Byte
	Megabyte = 1000 * Kilobyte
	Gigabyte = 1000 * Megabyte
	Terabyte = 1000 * Gigabyte
	Petabyte = 1000 * Terabyte
	Exabyte  = 1000 * Petabyte

	Kibibyte = 1024 * Byte
	Mebibyte = 1024 * Kibibyte
	Gibibyte = 1024 * Mebibyte
	Tebibyte = 1024 * Gibibyte
	Pebibyte = 1024 * Tebibyte
	Exbibyte = 1024 * Pebibyte
)

var unitMap = map[string]Bytes{
	"":   Byte,
	"b":  Byte,
	"B":  Byte,
	"kB": Kilobyte,
	"KB": Kilobyte,
	"MB": Megabyte,
	"GB": Gigabyte,
	"TB": Terabyte,
	"PB": Petabyte,
	"EB": Exabyte,

	"KiB": Kibibyte,
	"MiB": Mebibyte,
	"GiB": Gibibyte,
	"TiB": Tebibyte,
	"PiB": Pebibyte,
	"EiB": Exbibyte,
}

// UnmarshalText parses a byte size from a string. Byte sizes are represented
// as sequences of number and unit pairs with no whitespace. Units are
// represented either as IEC units (KiB, MiB, GiB, etc) or metric units (kB or
// KB, MB, GB, etc).
//
// Multiple sequences of byte sizes can be provided in a single string, such as
// "4MB2KB". The sum of across all sequences is returned.
func (b *Bytes) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return ErrInvalidSyntax
	}

	// Byte offset while scanning through input.
	s := newScanner(string(text))

	// Parse optional leading sign.
	sign := 1
	switch s.Peek() {
	case '-':
		sign = -1
		s.Scan() // Advance scanner
	case '+':
		sign = 1 // This is redundant, but added for clarity.
		s.Scan() // Advance scanner
	}

	var sum Bytes

	for s.Next() {
		// Find digit components.
		numberText, err := scanNumberString(s)
		if err != nil {
			return err
		}
		number, err := strconv.ParseInt(numberText, 10, 64)
		if err != nil {
			return ErrInvalidSyntax
		}

		unit, ok := unitMap[scanUnitString(s)]
		if !ok {
			return ErrInvalidSyntax
		}

		newBytes := Bytes(number * int64(unit))
		if newBytes/unit != Bytes(number) {
			return ErrOverflow
		} else if sum+newBytes < sum {
			return ErrOverflow
		}

		sum += newBytes
	}

	*b = Bytes(sign) * sum
	return nil
}

func scanNumberString(s *scanner) (string, error) {
	var str string

	for s.Next() {
		ch := s.Peek()

		if '0' <= ch && ch <= '9' {
			str += string(ch)
			_ = s.Scan() // Advance the scanner.
			continue
		}

		break
	}

	if len(str) == 0 {
		return "", ErrInvalidSyntax
	}
	return str, nil
}

func scanUnitString(s *scanner) string {
	var str string

	// Scan until a non-number character.
	for s.Next() {
		ch := s.Peek()
		if ch < '0' || ch > '9' {
			str += string(ch)
			_ = s.Scan() // Advance the scanner.
			continue
		}

		break
	}

	return str
}

// MarshalText returns the string representation of b. See [Bytes.String] for
// more information.
func (b Bytes) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// String returns a string representing the bytes in human-readable form. Bytes
// are returned in the highest possible unit that retains accuracy. If b is a
// multiple of 1024, the IEC binary prefixes are used (KiB, MiB, GiB, etc).
// Otherwise, the SI decimal prefixes are used (kB, MB, GB, etc).
//
// Byte sizes are always displayed as whole numbers, and are represented in the
// highest possible prefix that preserves precision. For example, 1024 bytes
// would be represented as "1KiB", while 1025 bytes would be represented as
// "1025".
func (b Bytes) String() string {
	if b == 0 {
		return "0"
	}

	var metricSuffixes = []string{"", "kB", "MB", "GB", "TB", "PB", "EB"}
	var iecSuffixes = []string{"", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

	var suffixOffset int

	isMetric := b%1000 == 0

	switch {
	case isMetric:
		for b%1000 == 0 && suffixOffset <= len(metricSuffixes)-1 {
			// Divide by 1000, increase suffix offset.
			b /= 1000
			suffixOffset++
		}

		if suffixOffset == 0 {
			return fmt.Sprintf("%d", b)
		}
		return fmt.Sprintf("%d%s", b, metricSuffixes[suffixOffset])

	default:
		for b%1024 == 0 && suffixOffset < len(iecSuffixes)-1 {
			// Divide by 1024, increase suffix offset.
			b /= 1024
			suffixOffset++
		}

		if suffixOffset == 0 {
			return fmt.Sprintf("%d", b)
		}
		return fmt.Sprintf("%d%s", b, iecSuffixes[suffixOffset])
	}
}
