package common

import "encoding/json"

type LogResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string    `json:"resultType"`
		Result     []LogData `json:"result"`
	} `json:"data"`
}

type LogData struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

func (m *LogResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}
