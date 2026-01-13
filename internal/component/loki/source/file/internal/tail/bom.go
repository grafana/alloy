package tail

import (
	"bytes"
	"io"
	"os"
)

// SkipBOMFile detects and skips a BOM at the beginning of the file.
// It returns the number of bytes skipped (0 if no BOM).
// The file offset is left positioned correctly for subsequent reads.
func skipBOM(f *os.File) int64 {
	// Read up to 4 bytes (longest BOM)
	var buf [4]byte
	n, err := f.Read(buf[:])
	if err != nil && n == 0 {
		return 0
	}

	bomLen := detectBOM(buf[:n])
	f.Seek(bomLen, io.SeekStart)
	return bomLen
}

func detectBOM(b []byte) int64 {
	switch {
	case bytes.HasPrefix(b, []byte{0x00, 0x00, 0xFE, 0xFF}):
		return 4 // UTF-32 BE
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE, 0x00, 0x00}):
		return 4 // UTF-32 LE
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return 3 // UTF-8
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return 2 // UTF-16 BE
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return 2 // UTF-16 LE
	default:
		return 0
	}
}

