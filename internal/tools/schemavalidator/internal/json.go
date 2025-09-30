package internal

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

func JSONFromYAML(src []byte) ([]byte, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(src), &data); err != nil {
		return nil, err
	}
	return json.MarshalIndent(data, "", "  ")
}
