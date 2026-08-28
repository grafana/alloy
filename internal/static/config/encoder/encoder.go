package encoder

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/dimchansky/utfbom"
	"golang.org/x/text/encoding"
	uni "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
)

// EnsureUTF8 will convert from the most common encodings to UTF8.
// If useStrictUTF8 is enabled then if the file is not already utf8 then an error will be returned.
func EnsureUTF8(config []byte, useStrictUTF8 bool) ([]byte, error) {
	buffer := bytes.NewBuffer(config)
	src, enc := utfbom.Skip(buffer)
	var converted []byte
	skippedBytes, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	var encoder encoding.Encoding
	switch enc {
	case utfbom.UTF16BigEndian:
		encoder = uni.UTF16(uni.BigEndian, uni.IgnoreBOM)
	case utfbom.UTF16LittleEndian:
		encoder = uni.UTF16(uni.LittleEndian, uni.IgnoreBOM)
	case utfbom.UTF32BigEndian:
		encoder = utf32.UTF32(utf32.BigEndian, utf32.IgnoreBOM)
	case utfbom.UTF32LittleEndian:
		encoder = utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM)
	case utfbom.UTF8: // This only checks utf8 bom
		return config, nil
	default:
		// If its utf8 valid then return.
		if utf8.Valid(config) {
			return config, nil
		}
		return nil, fmt.Errorf("unknown encoding for config")
	}
	if useStrictUTF8 {
		return nil, fmt.Errorf("configuration is encoded with %s but must be utf8", enc.String())
	}
	decoder := encoder.NewDecoder()
	converted, err = decoder.Bytes(skippedBytes)
	return converted, err
}
