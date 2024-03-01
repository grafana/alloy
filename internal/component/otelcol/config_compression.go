package otelcol

import (
	"encoding"
	"fmt"

	"go.opentelemetry.io/collector/config/configcompression"
)

// CompressionType represents a mechanism used to compress data.
type CompressionType string

// Supported values for compression
const (
	CompressionTypeGzip    CompressionType = "gzip"
	CompressionTypeZlib    CompressionType = "zlib"
	CompressionTypeDeflate CompressionType = "deflate"
	CompressionTypeSnappy  CompressionType = "snappy"
	CompressionTypeZstd    CompressionType = "zstd"
	CompressionTypeNone    CompressionType = "none"
	CompressionTypeEmpty   CompressionType = ""
)

var _ encoding.TextUnmarshaler = (*CompressionType)(nil)

// UnmarshalText converts a string into a CompressionType. Returns an error if
// the string is invalid.
func (ct *CompressionType) UnmarshalText(in []byte) error {
	switch typ := CompressionType(in); typ {
	case CompressionTypeGzip, CompressionTypeZlib, CompressionTypeDeflate,
		CompressionTypeSnappy, CompressionTypeZstd, CompressionTypeNone, CompressionTypeEmpty:

		*ct = typ
		return nil
	default:
		return fmt.Errorf("unrecognized compression type %q", typ)
	}
}

var compressionMappings = map[CompressionType]configcompression.CompressionType{
	CompressionTypeGzip:    configcompression.Gzip,
	CompressionTypeZlib:    configcompression.Zlib,
	CompressionTypeDeflate: configcompression.Deflate,
	CompressionTypeSnappy:  configcompression.Snappy,
	CompressionTypeZstd:    configcompression.Zstd,
	CompressionTypeNone:    configcompression.CompressionType("none"),
	CompressionTypeEmpty:   configcompression.CompressionType(""),
}

// Convert converts ct into the upstream type.
func (ct CompressionType) Convert() configcompression.CompressionType {
	upstream, ok := compressionMappings[ct]
	if !ok {
		// This line should never hit unless compressionMappings wasn't updated
		// when the list of valid options was extended.
		panic("missing entry in compressionMappings table for " + string(ct))
	}
	return upstream
}
