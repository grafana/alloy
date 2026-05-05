package stages

import (
	"encoding"
	"fmt"
)

type SourceType string

const (
	SourceTypeLine               SourceType = "line"
	SourceTypeLabel              SourceType = "label"
	SourceTypeStructuredMetadata SourceType = "structured_metadata"
	SourceTypeExtractedMap       SourceType = "extracted"
)

var (
	_ encoding.TextMarshaler   = SourceType("")
	_ encoding.TextUnmarshaler = (*SourceType)(nil)
)

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *SourceType) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case string(SourceTypeLine), string(SourceTypeLabel), string(SourceTypeStructuredMetadata), string(SourceTypeExtractedMap):
		*t = SourceType(str)
	default:
		return fmt.Errorf("unknown source_type: %s", str)
	}

	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (t SourceType) MarshalText() (text []byte, err error) {
	return []byte(t), nil
}
