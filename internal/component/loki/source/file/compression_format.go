package file

import (
	"encoding"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
)

type CompressionFormat string

var (
	_ encoding.TextMarshaler   = CompressionFormat("")
	_ encoding.TextUnmarshaler = (*CompressionFormat)(nil)
)

func (ut CompressionFormat) String() string {
	return string(ut)
}

// MarshalText implements encoding.TextMarshaler.
func (ut CompressionFormat) MarshalText() (text []byte, err error) {
	return []byte(ut.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ut *CompressionFormat) UnmarshalText(text []byte) error {
	s := string(text)
	_, ok := supportedCompressedFormats()[s]
	if !ok {
		return fmt.Errorf(
			"unsupported compression format: %q - please use one of %q",
			s,
			strings.Join(maps.Keys(supportedCompressedFormats()), ", "),
		)
	}
	*ut = CompressionFormat(s)
	return nil
}
