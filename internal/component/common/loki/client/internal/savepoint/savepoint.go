package savepoint

import (
	"encoding/json"
	"fmt"
)

const (
	// currentVersion is the version written to new savepoint files. Bump it only
	// for changes that an older Alloy cannot read.
	currentVersion = 1

	// noSegment is reported for an endpoint that has no saved segment.
	noSegment = -1
)

// entry is the savepoint of a single endpoint. It is an object rather than a
// bare segment number so that it can gain fields without a format change.
type entry struct {
	Segment int `json:"segment"`
}

// contents is the on-disk layout of the savepoint file.
type contents struct {
	Version   int              `json:"version"`
	Endpoints map[string]entry `json:"endpoints"`
}

func encode(entries map[string]entry) ([]byte, error) {
	return json.Marshal(contents{
		Version:   currentVersion,
		Endpoints: entries,
	})
}

func decode(bs []byte) (map[string]entry, error) {
	var c contents
	if err := json.Unmarshal(bs, &c); err != nil {
		return nil, err
	}

	if c.Version != currentVersion {
		return nil, fmt.Errorf("unsupported savepoint version %d", c.Version)
	}

	if c.Endpoints == nil {
		c.Endpoints = make(map[string]entry)
	}

	return c.Endpoints, nil
}
