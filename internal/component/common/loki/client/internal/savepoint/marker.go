package savepoint

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"github.com/natefinch/atomic"
)

var (
	markerHeaderV1 = []byte{'0', '1'}

	markerFolderName = "remote"
	markerFileName   = "segment_marker"
)

// encodeMarker encodes the segment number, from whom we need to create a marker, in the marker file format,
// which in v1 includes the segment number and a trailing CRC code of the first 10 bytes.
func encodeMarker(segment uint64) []byte {
	// marker format v1
	// marker [ 0 , 1 ] - HEADER, which is used to track version
	// marker [ 2 , 9 ] - encoded uint64 which is the content of the marker, the last "consumed" segment
	// marker [ 10, 13 ] - CRC32 of the first 10 bytes of the marker, using IEEE polynomial
	bs := make([]byte, 14)
	// write header with marker format version
	bs[0] = markerHeaderV1[0]
	bs[1] = markerHeaderV1[1]
	// write actual marked segment number
	binary.BigEndian.PutUint64(bs[2:10], segment)
	// checksum is the IEEE CRC32 checksum of the first 10 bytes of the marker record
	checksum := crc32.ChecksumIEEE(bs[0:10])
	binary.BigEndian.PutUint32(bs[10:], checksum)

	return bs
}

// decodeMarker decodes the segment number from a segment marker.
func decodeMarker(bs []byte) (uint64, error) {
	// first check that read byte stream has expected length
	if len(bs) != 14 {
		return 0, fmt.Errorf("bad length %d", len(bs))
	}

	// check CRC first
	expectedCrc := crc32.ChecksumIEEE(bs[0:10])
	gotCrc := binary.BigEndian.Uint32(bs[len(bs)-4:])
	if expectedCrc != gotCrc {
		return 0, fmt.Errorf("corrupted WAL marker")
	}

	// check expected version header
	header := bs[:2]
	if !(header[0] == markerHeaderV1[0] && header[1] == markerHeaderV1[1]) {
		return 0, fmt.Errorf("wrong WAL marker header")
	}

	// lastly, decode marked segment number
	return binary.BigEndian.Uint64(bs[2:10]), nil
}

// MigrateLegacyMarker seeds the savepoints of keys in dir from a marker file
// written before savepoints existed, so that every endpoint resumes where the
// shared marker left off. It must run before the savepoint file is loaded.
// It does nothing if dir already holds a savepoint file, or if there is no
// marker file to migrate.
func MigrateLegacyMarker(dir string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	savepointPath := filepath.Join(dir, folderName, fileName)
	_, err := os.Stat(savepointPath)
	if err == nil {
		// IF we already have a savepoint file we don't do any migration
		return nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error reading savepoint file %q: %w", savepointPath, err)
	}

	markerPath := filepath.Join(dir, markerFolderName, markerFileName)
	bs, err := os.ReadFile(markerPath)

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading WAL marker %q: %w", markerPath, err)
	}

	segment, err := decodeMarker(bs)
	if err != nil {
		return fmt.Errorf("error decoding WAL marker %q: %w", markerPath, err)
	}

	entries := make(map[string]entry, len(keys))
	for _, key := range keys {
		entries[key] = entry{Segment: int(segment)}
	}

	contents, err := encode(entries)
	if err != nil {
		return fmt.Errorf("error encoding savepoints: %w", err)
	}

	folder := filepath.Join(dir, folderName)
	if err := os.MkdirAll(folder, folderMode); err != nil {
		return fmt.Errorf("error creating savepoint folder %q: %w", folder, err)
	}

	if err := atomic.WriteFile(savepointPath, bytes.NewReader(contents)); err != nil {
		return fmt.Errorf("error writing savepoint file %q: %w", savepointPath, err)
	}

	return nil
}
