// This file was copied from v0.129.0 of opentelemetry-collector-contrib/internal/coreinternal/textutils

package textutils

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"
)

var encodingOverrides = map[string]encoding.Encoding{
	"utf-16":    unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"utf16":     unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"utf-8":     unicode.UTF8,
	"utf8":      unicode.UTF8,
	"utf-8-raw": UTF8Raw,
	"utf8-raw":  UTF8Raw,
	"ascii":     unicode.UTF8,
	"us-ascii":  unicode.UTF8,
	"nop":       encoding.Nop,
	"":          unicode.UTF8,
}

func LookupEncoding(enc string) (encoding.Encoding, error) {
	if e, ok := encodingOverrides[strings.ToLower(enc)]; ok {
		return e, nil
	}
	e, err := ianaindex.IANA.Encoding(enc)
	if err != nil {
		return nil, fmt.Errorf("unsupported encoding '%s'", enc)
	}
	if e == nil {
		return nil, fmt.Errorf("no charmap defined for encoding '%s'", enc)
	}
	return e, nil
}
