package operator

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	var exampleRiverConfig = `
    forward_to = []
    namespaces = ["my-app"]
    selector {
        match_expression {
            key = "team"
            operator = "In"
            values = ["ops"]
        }
        match_labels = {
            team = "ops",
        }
    }
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}
