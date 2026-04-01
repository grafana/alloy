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

type LogEntry struct {
	Timestamp string
	Line      string
	Metadata  LogEntryMetadata
}

type LogEntryMetadata struct {
	StructuredMetadata map[string]string `json:"structuredMetadata,omitempty"`
}

type LogData struct {
	Stream map[string]string `json:"stream"`
	Values []LogEntry        `json:"values"`
}

func (m *LogData) UnmarshalJSON(data []byte) error {
	var raw struct {
		Stream map[string]string    `json:"stream"`
		Values [][3]json.RawMessage `json:"values"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	m.Stream = raw.Stream
	m.Values = make([]LogEntry, 0, len(raw.Values))

	for _, value := range raw.Values {
		var entry LogEntry

		if err := json.Unmarshal(value[0], &entry.Timestamp); err != nil {
			return err
		}
		if err := json.Unmarshal(value[1], &entry.Line); err != nil {
			return err
		}

		if err := json.Unmarshal(value[2], &entry.Metadata); err != nil {
			return err
		}

		m.Values = append(m.Values, entry)
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
