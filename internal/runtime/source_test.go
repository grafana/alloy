package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/stretchr/testify/require"

	_ "github.com/grafana/alloy/internal/runtime/internal/testcomponents" // Include test components
)

func TestParseSource(t *testing.T) {
	content := `
		testcomponents.tick "ticker_a" {
			frequency = "1s"
		}

		testcomponents.passthrough "static" {
			input = "hello, world!"
		}
	`

	f, err := ParseSource(t.Name(), []byte(content))
	require.NoError(t, err)
	require.NotNil(t, f)

	require.Len(t, f.components, 2)
	require.Equal(t, "testcomponents.tick.ticker_a", getBlockID(f.components[0]))
	require.Equal(t, "testcomponents.passthrough.static", getBlockID(f.components[1]))
}

func TestParseSourceWithConfigBlock(t *testing.T) {
	content := `
        logging {
		    format = "json"
		}

		testcomponents.tick "ticker_with_config_block" {
			frequency = "1s"
		}
	`

	f, err := ParseSource(t.Name(), []byte(content))
	require.NoError(t, err)
	require.NotNil(t, f)

	require.Len(t, f.components, 1)
	require.Equal(t, "testcomponents.tick.ticker_with_config_block", getBlockID(f.components[0]))
	require.Len(t, f.configBlocks, 1)
	require.Equal(t, "logging", getBlockID(f.configBlocks[0]))
}

func TestParseSource_Defaults(t *testing.T) {
	f, err := ParseSource(t.Name(), []byte(``))
	require.NotNil(t, f)
	require.NoError(t, err)

	require.Len(t, f.components, 0)
}

func TestParseSources_DuplicateComponent(t *testing.T) {
	content := `
        logging {
		    format = "json"
		}

		testcomponents.tick "ticker_duplicate_component_1" {
			frequency = "1s"
		}
	`

	content2 := `
        logging {
		    format = "json"
		}

		testcomponents.tick "ticker_duplicate_component_1" {
			frequency = "1s"
		}
	`

	s, err := ParseSources(map[string][]byte{
		"t1": []byte(content),
		"t2": []byte(content2),
	})
	require.NoError(t, err)
	ctrl, err := New(testOptions(t))
	require.NoError(t, err)
	defer cleanUpController(t.Context(), ctrl)
	err = ctrl.LoadSource(s, nil, "")
	diagErrs, ok := err.(diag.Diagnostics)
	require.True(t, ok)
	require.Len(t, diagErrs, 2)
}

func TestParseSources_UniqueComponent(t *testing.T) {
	content := `
        logging {
		    format = "json"
		}

		testcomponents.tick "ticker_unique_component_1" {
			frequency = "1s"
		}
	`

	content2 := `
		testcomponents.tick "ticker_unique_component_2" {
			frequency = "1s"
		}
	`

	s, err := ParseSources(map[string][]byte{
		"t1": []byte(content),
		"t2": []byte(content2),
	})
	require.NoError(t, err)
	ctrl, err := New(testOptions(t))
	require.NoError(t, err)
	defer cleanUpController(t.Context(), ctrl)
	err = ctrl.LoadSource(s, nil, "")
	require.NoError(t, err)
}

func TestParseSources_SyntaxErrors(t *testing.T) {
	file1 := `
		testcomponents.tick "tick1" {
			frequency = "1s"
	`

	file2 := `
		testcomponents.tick "tick2" {
			frequency = "1s"
	`

	s, err := ParseSources(map[string][]byte{
		"1": []byte(file1),
		"2": []byte(file2),
	})

	require.Nil(t, s)
	require.Error(t, err)

	var diags diag.Diagnostics
	require.True(t, errors.As(err, &diags))
	require.Len(t, diags, 2)
	require.Equal(t, "1", diags[0].StartPos.Filename)
	require.Equal(t, "2", diags[1].StartPos.Filename)
}

func getBlockID(b *ast.BlockStmt) string {
	var parts []string
	parts = append(parts, b.Name...)
	if b.Label != "" {
		parts = append(parts, b.Label)
	}
	return strings.Join(parts, ".")
}
