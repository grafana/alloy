package common

import (
	"encoding/json"
)

// Query response types
type LogResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string    `json:"resultType"`
		Result     []LogData `json:"result"`
	} `json:"data"`
}

type LogData struct {
	Stream map[string]string `json:"stream"`
	Values []LogEntry        `json:"values"`
}

type LogEntry struct {
	Timestamp string
	Line      string
	Metadata  LogEntryMetadata
}

type LogEntryMetadata struct {
	StructuredMetadata map[string]string
}

func (e LogEntry) MarshalJSON() ([]byte, error) {
	if len(e.Metadata.StructuredMetadata) == 0 {
		return json.Marshal([2]any{e.Timestamp, e.Line})
	}

	return json.Marshal([3]any{e.Timestamp, e.Line, e.Metadata.StructuredMetadata})
}

func (e *LogEntry) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if err := json.Unmarshal(raw[0], &e.Timestamp); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &e.Line); err != nil {
		return err
	}

	if len(raw) == 2 {
		return nil
	}

	// Query responses use a object in the third array element, for example:
	// ["<ts>", "<line>", {"structuredMetadata": {...}}]
	// See https://github.com/grafana/loki/blob/5f06abe88d1a5cb7df12cfd2e07808c9d3238312/pkg/loghttp/entry.go#L63-L73
	// Because we always use this to unmashal query response we don't have to handle the flat map case.
	var metadata struct {
		StructuredMetadata map[string]string `json:"structuredMetadata"`
	}
	if err := json.Unmarshal(raw[2], &metadata); err != nil {
		return err
	}

	if len(metadata.StructuredMetadata) > 0 {
		e.Metadata.StructuredMetadata = metadata.StructuredMetadata
	}
	return nil
}

func (m *LogResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

// Push request types
type PushRequest struct {
	Streams []LogData `json:"streams"`
}

type LogSeriesResponse struct {
	Status string              `json:"status"`
	Data   []map[string]string `json:"data"`
}

func (m *LogSeriesResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

