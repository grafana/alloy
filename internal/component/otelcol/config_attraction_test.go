package otelcol_test

import (
	"testing"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/stretchr/testify/require"
)

func TestConvertAttrAction(t *testing.T) {
	inputActions := otelcol.AttrActionKeyValueSlice{
		{
			Action: "insert",
			Value:  123,
			Key:    "attribute1",
		},
		{
			Action: "delete",
			Key:    "attribute2",
		},
		{
			Action: "upsert",
			Value:  true,
			Key:    "attribute3",
		},
	}

	expectedActions := []interface{}{
		map[string]interface{}{
			"action":         "insert",
			"converted_type": "",
			"from_attribute": "",
			"from_context":   "",
			"key":            "attribute1",
			"pattern":        "",
			"value":          123,
		},
		map[string]interface{}{
			"action":         "delete",
			"converted_type": "",
			"from_attribute": "",
			"from_context":   "",
			"key":            "attribute2",
			"pattern":        "",
			"value":          interface{}(nil),
		},
		map[string]interface{}{
			"action":         "upsert",
			"converted_type": "",
			"from_attribute": "",
			"from_context":   "",
			"key":            "attribute3",
			"pattern":        "",
			"value":          true,
		},
	}

	result := inputActions.Convert()
	require.Equal(t, expectedActions, result)
}
