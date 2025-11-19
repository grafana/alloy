// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package modulestore // import "go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// ID is used to uniquely identify a module in a `Store`.
type ID struct {
	// hash stores the SHA256 sum of the module. It's distinct from the typical file ID  we use
	// everywhere else because the file ID is a partial hash, allowing for collisions. To also
	// allow testing cases with colliding file IDs, the coredump tests use a more traditional
	// checksum.
	hash [32]byte
}

// String implements the `fmt.Stringer` interface
func (id *ID) String() string {
	return hex.EncodeToString(id.hash[:])
}

// IDFromString parses a string into an ID.
func IDFromString(s string) (ID, error) {
	if len(s) != 64 {
		return ID{}, fmt.Errorf("length %d doesn't match expected value (64)", len(s))
	}

	slice, err := hex.DecodeString(s)
	if err != nil {
		return ID{}, fmt.Errorf("failed to parse id: %w", err)
	}

	var id ID
	copy(id.hash[:], slice)

	return id, nil
}

// MarshalJSON encodes the ID into JSON.
func (id *ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

// UnmarshalJSON decodes JSON into an ID.
func (id *ID) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	parsed, err := IDFromString(v)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// calculateModuleID calculates the module ID for the given reader.
func calculateModuleID(reader io.Reader) (ID, error) {
	buf := make([]byte, 16*1024)
	hasher := sha256.New()
	for {
		n, err := reader.Read(buf)
		if n == 0 {
			break
		}

		hasher.Write(buf[:n])

		if err != nil {
			if err == io.EOF {
				break
			}
			return ID{}, fmt.Errorf("failed to read chunk: %w", err)
		}
	}

	var id ID
	copy(id.hash[:], hasher.Sum(nil))

	return id, nil
}
