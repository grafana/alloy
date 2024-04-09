package operator

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	var exampleAlloyConfig = `
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
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}
