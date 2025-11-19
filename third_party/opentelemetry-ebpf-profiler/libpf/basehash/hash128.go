// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basehash // import "go.opentelemetry.io/ebpf-profiler/libpf/basehash"

import (
	"encoding"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Hash128 represents a uint128 using two uint64s.
//
// hi represents the most significant 64 bits and lo represents the least
// significant 64 bits.
type Hash128 struct { //nolint:recvcheck
	hi uint64
	lo uint64
}

func (h Hash128) ToUUIDString() string {
	// The following can't fail: we are guaranteed to get a slice of the correct length.
	id, _ := uuid.FromBytes(h.Bytes())
	return id.String()
}

// Base64 returns the base64 FileID representation for inclusion into JSON payloads.
func (h Hash128) Base64() string {
	return base64.RawURLEncoding.EncodeToString(h.Bytes())
}

func New128(hi, lo uint64) Hash128 {
	return Hash128{hi, lo}
}

// New128FromBytes returns a Hash128 given by the bytes in b.
func New128FromBytes(b []byte) (Hash128, error) {
	if len(b) != 16 {
		return Hash128{}, fmt.Errorf("invalid length for bytes: %d", len(b))
	}
	h := Hash128{}
	h.hi = binary.BigEndian.Uint64(b[0:8])
	h.lo = binary.BigEndian.Uint64(b[8:16])
	return h, nil
}

// New128FromString returns a Hash128 given by the characters in s.
func New128FromString(s string) (Hash128, error) {
	// The Hash128 Format prefixes the string with "0x" for some given formatter.
	s = strings.TrimPrefix(s, "0x")
	// In the UUID representation the hash string contains "-".
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return Hash128{}, fmt.Errorf("invalid length for string '%s': %d", s, len(s))
	}
	hi, err := strconv.ParseUint(s[0:16], 16, 64)
	if err != nil {
		return Hash128{}, err
	}
	lo, err := strconv.ParseUint(s[16:32], 16, 64)
	if err != nil {
		return Hash128{}, err
	}
	return New128(hi, lo), nil
}

// Less reports whether h is less than other.
//
// The order defined here must be the same as the one used in "SELECT ... FOR UPDATE" queries,
// otherwise DB deadlocks may occur on concurrent updates, hence the casts to int64.
func (h Hash128) Less(other Hash128) bool {
	return int64(h.hi) < int64(other.hi) ||
		(h.hi == other.hi && int64(h.lo) < int64(other.lo))
}

func (h Hash128) Equal(other Hash128) bool {
	return h.hi == other.hi && h.lo == other.lo
}

func (h Hash128) IsZero() bool {
	return h.hi == 0 && h.lo == 0
}

// Compare returns an integer comparing two hashes lexicographically.
// The result will be 0 if h == other, -1 if h < other, and +1 if h > other.
func (h Hash128) Compare(other Hash128) int {
	if int64(h.hi) < int64(other.hi) {
		return -1
	}
	if int64(h.hi) > int64(other.hi) {
		return 1
	}
	if int64(h.lo) < int64(other.lo) {
		return -1
	}
	if int64(h.lo) > int64(other.lo) {
		return 1
	}
	return 0
}

// copyBytes copies the byte slice representation of a Hash128 into b.
func (h Hash128) copyBytes(b []byte) []byte {
	binary.BigEndian.PutUint64(b[0:8], h.hi)
	binary.BigEndian.PutUint64(b[8:16], h.lo)
	return b
}

// Bytes returns a byte slice representation of a Hash128.
func (h Hash128) Bytes() []byte {
	return h.copyBytes(make([]byte, 16))
}

// Format implements fmt.Formatter.
//
// It accepts the formats 'd' (decimal), 'v' (value), 'x'
// (lowercase hexadecimal), and 'X' (uppercase hexadecimal).
//
// Also supported is a subset of the package fmt's format
// flags, including '#' for leading zero in hexadecimal.
//
// For any unsupported format, the value will be serialized
// using the gob codec.
func (h Hash128) Format(s fmt.State, ch rune) {
	if s.Flag('#') {
		if ch == 'x' || ch == 'v' {
			_, _ = s.Write([]byte("0x"))
			_, _ = s.Write([]byte(uint64ToLowerHex(h.hi)))
			buf := make([]byte, 16)
			putUint64AsLowerHex(h.lo, buf)
			_, _ = s.Write(buf)
			return
		}

		if ch == 'X' {
			_, _ = s.Write([]byte("0x"))
			_, _ = s.Write([]byte(uint64ToUpperHex(h.hi)))
			buf := make([]byte, 16)
			putUint64AsUpperHex(h.lo, buf)
			_, _ = s.Write(buf)
			return
		}
	}

	if ch == 'x' {
		_, _ = s.Write([]byte(uint64ToLowerHex(h.hi)))
		buf := make([]byte, 16)
		putUint64AsLowerHex(h.lo, buf)
		_, _ = s.Write(buf)
		return
	}

	if ch == 'X' {
		_, _ = s.Write([]byte(uint64ToUpperHex(h.hi)))
		buf := make([]byte, 16)
		putUint64AsUpperHex(h.lo, buf)
		_, _ = s.Write(buf)
		return
	}

	if ch == 'd' || ch == 'v' {
		fmt.Fprintf(s, "{%d %d}", h.hi, h.lo)
		return
	}

	fmt.Fprintf(s, "{%s %s}", uint64ToGob(h.hi, ch), uint64ToGob(h.lo, ch))
}

func (h Hash128) StringNoQuotes() string {
	return string(marshalIdentifier(h.hi, h.lo))
}

func (h Hash128) Words() (hi, lo uint64) {
	return h.hi, h.lo
}

func (h Hash128) MarshalJSON() ([]byte, error) {
	return marshalQuotedIdentifier(h.hi, h.lo), nil
}

func (h *Hash128) UnmarshalJSON(b []byte) error {
	if len(b) != 34 {
		return fmt.Errorf("invalid length for bytes: %d", len(b))
	}
	hash128, err := New128FromString(string(b)[1:33])
	if err != nil {
		return err
	}
	h.hi = hash128.hi
	h.lo = hash128.lo
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface, so we can
// marshal (from JSON) a map using a Hash128 as a key
func (h Hash128) MarshalText() ([]byte, error) {
	// Implements the encoding.TextMarshaler interface, so we can
	// marshal (from JSON) a map using a Hash128 as a key
	return marshalIdentifier(h.hi, h.lo), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface, so we can
// unmarshal (from JSON) a map using a Hash128 as a key
func (h *Hash128) UnmarshalText(text []byte) error {
	// Implements the encoding.TextUnmarshaler interface, so we can
	// unmarshal (from JSON) a map using a Hash128 as a key
	hash128, err := New128FromString(string(text))
	if err != nil {
		return err
	}
	h.hi = hash128.hi
	h.lo = hash128.lo
	return nil
}

// Hi returns the high 64 bits
func (h Hash128) Hi() uint64 {
	return h.hi
}

// Lo returns the low 64 bits
func (h Hash128) Lo() uint64 {
	return h.lo
}

// PutBytes16 writes the 16 bytes into the provided array pointer.
func (h Hash128) PutBytes16(b *[16]byte) {
	// The following can't fail since the length is 16 bytes
	_ = h.copyBytes(b[0:16])
}

// Compile-time interface checks
var _ fmt.Formatter = (*Hash128)(nil)

var _ encoding.TextUnmarshaler = (*Hash128)(nil)
var _ encoding.TextMarshaler = (*Hash128)(nil)

var _ json.Marshaler = (*Hash128)(nil)
var _ json.Unmarshaler = (*Hash128)(nil)
