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
