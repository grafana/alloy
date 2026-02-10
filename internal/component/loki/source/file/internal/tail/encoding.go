package tail

import (
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
)

func getEncoding(enc string) (encoding.Encoding, error) {
	if enc == "" {
		return encoding.Nop, nil
	}

	return ianaindex.IANA.Encoding(enc)
}

func encodedNewline(e *encoding.Encoder) ([]byte, error) {
	return e.Bytes([]byte{'\n'})
}

func encodedCarriageReturn(e *encoding.Encoder) ([]byte, error) {
	return e.Bytes([]byte{'\r'})
}
