package tail

import (
	"bytes"
	"io"
	"os"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// BOM byte sequences
var (
	bomUTF32BE = []byte{0x00, 0x00, 0xFE, 0xFF}
	bomUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF16LE = []byte{0xFF, 0xFE}
)

// skipBOM detects and skips a BOM at the beginning of the file.
// It takes the current offset and returns the final offset
// and the BOM bytes that were detected.
// The file is positioned at the start to read the BOM, then
// seeks to the final offset for subsequent reads.
func skipBOM(f *os.File, offset int64) (int64, []byte) {
	// Make sure we are reading from the start of the file.
	f.Seek(0, io.SeekStart)

	// Read up to 4 bytes (longest BOM)
	var buf [4]byte
	n, err := f.Read(buf[:])
	if err != nil && n == 0 {
		return offset, nil
	}

	bomLen := detectBom(buf[:n])

	// If a BOM was detected and its length is greater than or equal to the
	// provided offset, use the BOM length as the offset. Otherwise, use the
	// provided offset (which may be beyond the BOM).
	if bomLen >= offset {
		offset = bomLen
	}

	f.Seek(offset, io.SeekStart)
	return offset, buf[:bomLen]
}

// detectBom detects a BOM in the given bytes and returns the length
// of the BOM (0 if no BOM was detected).
func detectBom(b []byte) int64 {
	switch {
	case bytes.HasPrefix(b, bomUTF8):
		return 3
	case bytes.HasPrefix(b, bomUTF16BE):
		return 2
	case bytes.HasPrefix(b, bomUTF16LE):
		return 2
	case bytes.HasPrefix(b, bomUTF32BE):
		return 4
	case bytes.HasPrefix(b, bomUTF32LE):
		return 4
	default:
		return 0
	}
}

// resolveEncodingFromBOM takes the BOM bytes and the original encoding,
// and returns the resolved encoding. If a UTF-16 BOM is detected, it returns
// an encoding with the correct endianness and IgnoreBOM mode.
// Otherwise, it returns the original encoding.
func resolveEncodingFromBOM(bomBytes []byte, originalEnc encoding.Encoding) encoding.Encoding {
	if len(bomBytes) == 0 {
		return originalEnc
	}

	switch {
	case bytes.HasPrefix(bomBytes, bomUTF8):
		// UTF-8 BOM detected - return encoding
		return encoding.Nop
	case bytes.HasPrefix(bomBytes, bomUTF16BE):
		// UTF-16 BE BOM detected - return encoding with IgnoreBOM since we skip it
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case bytes.HasPrefix(bomBytes, bomUTF16LE):
		// UTF-16 LE BOM detected - return encoding with IgnoreBOM since we skip it
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	default:
		// Other BOMs (UTF-8, UTF-32) don't affect encoding selection
		return originalEnc
	}
}
