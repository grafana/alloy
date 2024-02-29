package internal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

var (
	markerHeaderV1 = []byte{'0', '1'}
)

// EncodeMarkerV1 encodes the segment number, from whom we need to create a marker, in the marker file format,
// which in v1 includes the segment number and a trailing CRC code of the first 10 bytes.
func EncodeMarkerV1(segment uint64) ([]byte, error) {
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

	return bs, nil
}

// DecodeMarkerV1 decodes the segment number from a segment marker, encoded with EncodeMarkerV1.
func DecodeMarkerV1(bs []byte) (uint64, error) {
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
