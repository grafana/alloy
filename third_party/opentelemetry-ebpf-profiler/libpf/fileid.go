// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"

	sha256 "github.com/minio/sha256-simd"
	"go.opentelemetry.io/ebpf-profiler/libpf/basehash"
)

// FileID is used for unique identifiers for files
type FileID struct {
	basehash.Hash128
}

// UnknownKernelFileID is used as 128-bit FileID when the host agent isn't able to derive a FileID
// for a kernel frame.
var UnknownKernelFileID = NewFileID(math.MaxUint64-2, math.MaxUint64-2)

func NewFileID(hi, lo uint64) FileID {
	return FileID{basehash.New128(hi, lo)}
}

// FileIDFromBytes parses a byte slice into the internal data representation for a file ID.
func FileIDFromBytes(b []byte) (FileID, error) {
	// We need to check for nil since byte slice fields in protobuf messages can be optional.
	// Until improved message validation and deserialization is added, this check will prevent
	// panics.
	if b == nil {
		return FileID{}, nil
	}
	h, err := basehash.New128FromBytes(b)
	if err != nil {
		return FileID{}, err
	}
	return FileID{h}, nil
}

// FileIDFromString parses a hexadecimal notation of a file ID into the internal data
// representation.
func FileIDFromString(s string) (FileID, error) {
	hash128, err := basehash.New128FromString(s)
	if err != nil {
		return FileID{}, err
	}
	return FileID{hash128}, nil
}

// FileIDFromBase64 converts a base64url encoded file ID into its binary representation.
// We store binary fields as keywords as base64 URL encoded strings.
// But when retrieving binary fields, ES sends them as base64 STD encoded strings.
func FileIDFromBase64(s string) (FileID, error) {
	data, err := base64.RawURLEncoding.DecodeString(s) // allows - and _ in input
	if err != nil {
		// ES uses StdEncoding when marshaling binary fields
		data, err = base64.RawStdEncoding.DecodeString(s) // allows + and / in input
		if err != nil {
			return FileID{}, fmt.Errorf("failed to decode to fileID %s: %v", s, err)
		}
	}
	if len(data) != 16 {
		return FileID{}, fmt.Errorf("unexpected input size (expected 16 bytes): %d",
			len(data))
	}
	return FileIDFromBytes(data)
}

// Hash32 returns a 32 bits hash of the input.
// It's main purpose is to be used as key for caching.
func (f FileID) Hash32() uint32 {
	return uint32(f.Hi())
}

func (f FileID) Equal(other FileID) bool {
	return f.Hash128.Equal(other.Hash128)
}

func (f FileID) Less(other FileID) bool {
	return f.Hash128.Less(other.Hash128)
}

// Compare returns an integer comparing two hashes lexicographically.
// The result will be 0 if f == other, -1 if f < other, and +1 if f > other.
func (f FileID) Compare(other FileID) int {
	return f.Hash128.Compare(other.Hash128)
}

// Swapped creates a new FileID with swapped high and low part. This function is its own inverse,
// so it can be used for the opposite operation. This is mostly used to connect Linux kernel
// module and its debug file build IDs. This provides 2 properties:
//   - FileIDs must be different between kernel files and their debug files.
//   - A kernel FileID (debug and non-debug) must only depend on its GNU BuildID (see
//     FileIDFromKernelBuildID), and can always be computed in the Host Agent or during indexing
//     without external information.
func (f FileID) Swapped() FileID {
	// Reverse high and low.
	return NewFileID(f.Lo(), f.Hi())
}

// Compile-time interface checks
var _ encoding.TextUnmarshaler = (*FileID)(nil)
var _ encoding.TextMarshaler = (*FileID)(nil)

// FileIDFromExecutableReader hashes portions of the contents of the reader in order to
// generate a system-independent identifier. The file is expected to be an executable
// file (ELF or PE) where the header and footer has enough data to make the file unique.
//
// *** WARNING ***
// ANY CHANGE IN BEHAVIOR CAN EASILY BREAK OUR INFRASTRUCTURE, POSSIBLY MAKING THE ENTIRETY
// OF THE DEBUG INDEX OR FRAME METADATA WORTHLESS (BREAKING BACKWARDS COMPATIBILITY).
func FileIDFromExecutableReader(reader io.ReadSeeker) (FileID, error) {
	h := sha256.New()

	// Hash algorithm: SHA256 of the following:
	// 1) 4 KiB header:
	//    ELF: should cover the program headers, and usually the GNU Build ID (if present)
	//         plus other sections.
	//    PE/dotnet: section headers, and typically also the build GUID.
	// 2) 4 KiB trailer: ELF: in practice, should cover the ELF section headers, as well as the
	//    contents of the debug link and other sections.
	// 3) File length (8 bytes, big-endian). Just for paranoia: ELF files can be appended to
	//    without restrictions, so it feels a bit too easy to produce valid ELF files that would
	//    produce identical hashes using only 1) and 2).

	// 1) Hash header
	_, err := io.Copy(h, io.LimitReader(reader, 4096))
	if err != nil {
		return FileID{}, fmt.Errorf("failed to hash file header: %v", err)
	}

	var size int64
	size, err = reader.Seek(0, io.SeekEnd)
	if err != nil {
		return FileID{}, fmt.Errorf("failed to seek end of file: %v", err)
	}

	// 2) Hash trailer
	// This will double-hash some data if the file is < 8192 bytes large. Oh well - better keep
	// it simple since the logic is customer-facing.
	tailBytes := min(size, 4096)
	_, err = reader.Seek(-tailBytes, io.SeekEnd)
	if err != nil {
		return FileID{}, fmt.Errorf("failed to seek file trailer: %v", err)
	}

	_, err = io.Copy(h, reader)
	if err != nil {
		return FileID{}, fmt.Errorf("failed to hash file trailer: %v", err)
	}

	// 3) Hash length
	lengthArray := make([]byte, 8)
	binary.BigEndian.PutUint64(lengthArray, uint64(size))
	_, err = io.Copy(h, bytes.NewReader(lengthArray))
	if err != nil {
		return FileID{}, fmt.Errorf("failed to hash file length: %v", err)
	}

	return FileIDFromBytes(h.Sum(nil)[0:16])
}

// FileIDFromExecutableFile opens an executable file and calculates the FileID for it.
// The caller is responsible pre-validate it as an executable file and that the algorithm
// described in FileIDFromExecutableReader is suitable.
func FileIDFromExecutableFile(fileName string) (FileID, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return FileID{}, err
	}
	defer f.Close()

	return FileIDFromExecutableReader(f)
}

// FileIDFromKernelBuildID returns the FileID of a kernel image or module, which consists
// of a hash of its GNU BuildID in hex string form.
// The hashing step is to ensure that the FileID remains an opaque concept to the end user.
func FileIDFromKernelBuildID(buildID string) FileID {
	h := fnv.New128a()
	_, _ = h.Write([]byte(buildID))
	// Cannot fail, ignore error.
	fileID, _ := FileIDFromBytes(h.Sum(nil))
	return fileID
}
