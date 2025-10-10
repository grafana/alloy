package stages

import (
	"bytes"
	"errors"
	"sort"

	json "github.com/json-iterator/go"

	"github.com/grafana/loki/v3/pkg/logqlmodel"
)

type Packed struct {
	Labels map[string]string `json:",inline"`
	Entry  string            `json:"_entry"`
}

// UnmarshalJSON populates a Packed struct where every key except the _entry key is added to the Labels field
func (w *Packed) UnmarshalJSON(data []byte) error {
	m := &map[string]interface{}{}
	err := json.Unmarshal(data, m)
	if err != nil {
		return err
	}
	w.Labels = map[string]string{}
	for k, v := range *m {
		// _entry key goes to the Entry field, everything else becomes a label
		if k == logqlmodel.PackedEntryKey {
			if s, ok := v.(string); ok {
				w.Entry = s
			} else {
				return errors.New("failed to unmarshal json, all values must be of type string")
			}
		} else {
			if s, ok := v.(string); ok {
				w.Labels[k] = s
			} else {
				return errors.New("failed to unmarshal json, all values must be of type string")
			}
		}
	}
	return nil
}

// MarshalJSON creates a Packed struct as JSON where the Labels are flattened into the top level of the object
func (w Packed) MarshalJSON() ([]byte, error) {
	// Marshal the entry to properly escape if it's json or contains quotes
	b, err := json.Marshal(w.Entry)
	if err != nil {
		return nil, err
	}

	// Creating a map and marshalling from a map results in a non deterministic ordering of the resulting json object
	// This is functionally ok but really annoying to humans and automated tests.
	// Instead we will build the json ourselves after sorting all the labels to get a consistent output
	keys := make([]string, 0, len(w.Labels))
	for k := range w.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer

	buf.WriteString("{")
	for i, k := range keys {
		if i != 0 {
			buf.WriteString(",")
		}
		// marshal key
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		// marshal value
		val, err := json.Marshal(w.Labels[k])
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	// Only add the comma if something exists in the buffer other than "{"
	if buf.Len() > 1 {
		buf.WriteString(",")
	}
	// Add the line entry
	buf.WriteString("\"" + logqlmodel.PackedEntryKey + "\":")
	buf.Write(b)

	buf.WriteString("}")
	return buf.Bytes(), nil
}

// PackConfig contains the configuration for a packStage
type PackConfig struct {
	Labels          []string `mapstrcuture:"labels"`
	IngestTimestamp *bool    `mapstructure:"ingest_timestamp"`
}
