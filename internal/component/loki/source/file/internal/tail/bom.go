package tail

import (
	"bufio"
	"bytes"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// BOM byte sequences
var (
	bomUTF8Bytes    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16BEBytes = []byte{0xFE, 0xFF}
	bomUTF16LEBytes = []byte{0xFF, 0xFE}
	bomUTF32BEBytes = []byte{0x00, 0x00, 0xFE, 0xFF}
	bomUTF32LEBytes = []byte{0xFF, 0xFE, 0x00, 0x00}
)

type BOM int

const (
	bomUNKNOWN BOM = iota
	bomUTF8
	bomUTF16BE
	bomUTF16LE
	bomUTF32BE
	bomUTF32LE
)

// detectBOM tries to detect a BOM from reader. It is important that the reader
// and underlying file are positioned at the beginning of the file
// when calling this function, as it peeks at the first bytes to detect the BOM.
func detectBOM(br *bufio.Reader, offset int64) (int64, BOM) {
	// Peek up to 4 bytes (longest BOM)
	buf, err := br.Peek(4)
	if err != nil {
		return offset, bomUNKNOWN
	}

	var bom BOM
	switch {
	case bytes.HasPrefix(buf, bomUTF8Bytes):
		bom = bomUTF8
		offset = max(3, offset)
	case bytes.HasPrefix(buf, bomUTF16BEBytes):
		bom = bomUTF16BE
		offset = max(2, offset)
	case bytes.HasPrefix(buf, bomUTF16LEBytes):
		bom = bomUTF16LE
		offset = max(2, offset)
	case bytes.HasPrefix(buf, bomUTF32BEBytes):
		bom = bomUTF32BE
		offset = max(4, offset)
	case bytes.HasPrefix(buf, bomUTF32LEBytes):
		bom = bomUTF32LE
		offset = max(4, offset)
	}

	return offset, bom
}

// resolveEncodingFromBOM takes the detected BOM and the original encoding,
// and returns the resolved encoding.
func resolveEncodingFromBOM(bom BOM, enc encoding.Encoding) encoding.Encoding {
	if bom == bomUNKNOWN {
		return enc
	}

	switch bom {
	case bomUTF8:
		return encoding.Nop
	case bomUTF16BE:
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case bomUTF16LE:
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	default:
		// Other BOMs don't affect encoding selection
		return enc
	}
}
