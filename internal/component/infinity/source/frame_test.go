package source

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

// fieldValues returns the concrete values of the named field, with nil for
// null values.
func fieldValues(f *data.Frame, name string) ([]any, bool) {
	for _, fld := range f.Fields {
		if fld.Name != name {
			continue
		}
		out := make([]any, fld.Len())
		for i := range out {
			if v, ok := fld.ConcreteAt(i); ok {
				out[i] = v
			}
		}
		return out, true
	}
	return nil, false
}

func TestBuildFrameJSON(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url           = "http://x"
		root_selector = "items"
		column {
			selector = "name"
			text     = "service"
			type     = "string"
		}
		column {
			selector = "stats.latency"
			text     = "latency"
			type     = "number"
		}
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte(`{"items":[{"name":"api","stats":{"latency":12}},{"name":"db","stats":{"latency":40}}]}`))
	require.NoError(t, err)
	services, ok := fieldValues(f, "service")
	require.True(t, ok)
	require.Equal(t, []any{"api", "db"}, services)
	latency, ok := fieldValues(f, "latency")
	require.True(t, ok)
	require.Equal(t, []any{12.0, 40.0}, latency)
}

func TestBuildFrameJQ(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url           = "http://x"
		parser        = "jq-backend"
		root_selector = ".items | map({n: .name})"
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte(`{"items":[{"name":"a"},{"name":"b"}]}`))
	require.NoError(t, err)
	names, ok := fieldValues(f, "n")
	require.True(t, ok)
	require.Equal(t, []any{"a", "b"}, names)
}

func TestBuildFrameCSV(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		type = "csv"
		url  = "http://x"
		column {
			selector = "v"
			text     = "v"
			type     = "number"
		}
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte("k,v\na,1\nb,2\n"))
	require.NoError(t, err)
	vals, ok := fieldValues(f, "v")
	require.True(t, ok)
	require.Equal(t, []any{1.0, 2.0}, vals)
}

func TestBuildFrameCSVHeaderOverride(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		type = "csv"
		url  = "http://x"
		csv_options { columns = "k,v" }
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte("a,1\nb,2\n"))
	require.NoError(t, err)
	keys, ok := fieldValues(f, "k")
	require.True(t, ok)
	require.Equal(t, []any{"a", "b"}, keys)
}

func TestBuildFrameTSV(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		type = "tsv"
		url  = "http://x"
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte("k\tv\na\t1\n"))
	require.NoError(t, err)
	keys, ok := fieldValues(f, "k")
	require.True(t, ok)
	require.Equal(t, []any{"a"}, keys)
}

func TestBuildFrameXML(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		type          = "xml"
		url           = "http://x"
		root_selector = "root.item"
		column {
			selector = "name"
			text     = "name"
			type     = "string"
		}
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte(`<root><item><name>a</name></item><item><name>b</name></item></root>`))
	require.NoError(t, err)
	names, ok := fieldValues(f, "name")
	require.True(t, ok)
	require.Equal(t, []any{"a", "b"}, names)
}

func TestBuildFrameInvalidJSON(t *testing.T) {
	s, err := specFromConfig(`query "q" { url = "http://x" }`)
	require.NoError(t, err)

	_, err = buildFrame(s, []byte(`{not json`))
	require.Equal(t, reasonParse, reasonOf(err))
}

// TestPostProcessOrder locks the step order. Each step depends on the one
// before it, so a different order gives a different result or an error.
func TestPostProcessOrder(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
		column {
			selector = "a"
			text     = "a"
			type     = "number"
		}
		computed_column {
			selector = "a * 10"
			text     = "b"
		}
		filter_expression    = "b > 10"
		summarize_expression = "count(a)"
		summarize_alias      = "n"
		transform.computed_column {
			expression = "n + 1"
			alias      = "m"
		}
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte(`[{"a":1},{"a":2},{"a":3}]`))
	require.NoError(t, err)
	n, ok := fieldValues(f, "n")
	require.True(t, ok)
	require.Equal(t, []any{2.0}, n)
	m, ok := fieldValues(f, "m")
	require.True(t, ok)
	require.Equal(t, []any{3.0}, m)
}

func TestTransformLimitAndFilter(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
		column {
			selector = "a"
			text     = "a"
			type     = "number"
		}
		transform.filter {
			expression = "a > 1"
		}
		transform.limit {
			limit = 1
		}
	}`)
	require.NoError(t, err)

	f, err := buildFrame(s, []byte(`[{"a":1},{"a":2},{"a":3}]`))
	require.NoError(t, err)
	a, ok := fieldValues(f, "a")
	require.True(t, ok)
	require.Equal(t, []any{2.0}, a)
}

func TestPostProcessError(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url               = "http://x"
		filter_expression = "a >"
	}`)
	require.NoError(t, err)

	_, err = buildFrame(s, []byte(`[{"a":1}]`))
	require.Equal(t, reasonPostprocess, reasonOf(err))
}
