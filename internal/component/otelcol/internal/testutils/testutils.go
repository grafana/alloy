package testutils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// compareConfigsAsJSON compares two configs by marshaling them to JSON.
// It is useful in situations where Go config structs contain internal state.
// Several components have been introducing this.
// For example: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/40933
func CompareConfigsAsJSON(t *testing.T, actual, expected any) {
	actualJSON, err := json.Marshal(actual)
	require.NoError(t, err)

	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	require.JSONEq(t, string(expectedJSON), string(actualJSON))
}
