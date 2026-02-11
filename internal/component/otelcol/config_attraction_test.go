package otelcol_test

import (
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
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

	expectedActions := []any{
		map[string]any{
			"action":         "insert",
			"converted_type": "",
			"from_attribute": "",
			"from_context":   "",
			"key":            "attribute1",
			"pattern":        "",
			"value":          123,
		},
		map[string]any{
			"action":         "delete",
			"converted_type": "",
			"from_attribute": "",
			"from_context":   "",
			"key":            "attribute2",
			"pattern":        "",
			"value":          any(nil),
		},
		map[string]any{
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

func TestValidateAttrAction(t *testing.T) {
	inputActions := otelcol.AttrActionKeyValueSlice{
		{
			// ok - only key
			Action: "insert",
			Value:  123,
			Key:    "attribute1",
		},
		{
			// not ok - missing key
			Action:       "insert",
			Value:        123,
			RegexPattern: "pattern", // pattern is useless here
		},
		{
			// ok - only key
			Action: "delete",
			Key:    "key",
		},
		{
			// ok - only pattern
			Action:       "delete",
			RegexPattern: "pattern",
		},
		{
			// ok - both
			Action:       "delete",
			Key:          "key",
			RegexPattern: "pattern",
		},
		{
			// not ok - missing key and pattern
			Action: "delete",
		},
		{
			// ok - only pattern
			Action:       "hash",
			RegexPattern: "pattern",
		},
		{
			// ok - with uppercase
			Action:       "HaSH",
			RegexPattern: "pattern",
		},
	}

	expectedErrors := []string{
		"validation failed for action block number 2: the action insert requires the key argument to be set",
		"validation failed for action block number 6: the action delete requires at least the key argument or the pattern argument to be set",
	}
	require.EqualError(t, inputActions.Validate(), strings.Join(expectedErrors, "\n"))
}
