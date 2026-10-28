package source

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/infinity-libs/lib/go/transformations"
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

// TestBuildFrameRecoversPanic uses an input that makes gframer panic on a
// type conversion. The poll must fail with reason parse, not crash Alloy.
func TestBuildFrameRecoversPanic(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
	}`)
	require.NoError(t, err)

	_, err = buildFrame(s, []byte(`[{"a":1},{"a":"x"}]`))
	require.Error(t, err)
	require.Equal(t, reasonParse, reasonOf(err))
	require.ErrorContains(t, err, "parser panic")
}

// distinctKeyRows returns a JSON array of n objects, each with its own key.
// gframer would allocate n*n cells for it.
func distinctKeyRows(n int) []byte {
	var b strings.Builder
	b.WriteByte('[')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"k%d":1}`, i)
	}
	b.WriteByte(']')
	return []byte(b.String())
}

func TestBuildFrameRejectsHugeFrame(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
	}`)
	require.NoError(t, err)
	body := distinctKeyRows(50_000)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	_, err = buildFrame(s, body)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)

	require.Error(t, err)
	require.Equal(t, reasonTooLarge, reasonOf(err))
	require.Less(t, elapsed, time.Second)
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(100<<20), "the check must fail before the frame is allocated")
}

func TestBuildFrameSizeCountsColumnBlocks(t *testing.T) {
	// With column blocks, the frame has only those columns, so many
	// distinct keys are fine.
	s, err := specFromConfig(`query "q" {
		url = "http://x"
		column {
			selector = "k1"
		}
	}`)
	require.NoError(t, err)
	_, err = buildFrame(s, distinctKeyRows(5_000))
	require.NoError(t, err)
}

// distinctChildXML returns n repeated item elements, each with its own
// child name. The XML parser turns them into rows with distinct keys.
func distinctChildXML(n int) []byte {
	var b strings.Builder
	b.WriteString("<root>")
	for i := range n {
		fmt.Fprintf(&b, "<item><k%d>1</k%d></item>", i, i)
	}
	b.WriteString("</root>")
	return []byte(b.String())
}

func TestBuildFrameRejectsHugeXMLFrame(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url           = "http://x"
		type          = "xml"
		root_selector = "root.item"
	}`)
	require.NoError(t, err)

	start := time.Now()
	_, err = buildFrame(s, distinctChildXML(5_000))
	require.Error(t, err)
	require.Equal(t, reasonTooLarge, reasonOf(err))
	require.Less(t, time.Since(start), time.Second)
}

// TestBuildFrameBudget checks the size of the budget: 1,500 rows with 1,500
// distinct keys need 2.25 million cells.
func TestBuildFrameBudget(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
	}`)
	require.NoError(t, err)
	_, err = buildFrame(s, distinctKeyRows(1_500))
	require.Equal(t, reasonTooLarge, reasonOf(err))
}

// wideCSV returns a header with cols columns, one full row, and rows short
// rows of one field each. relax_column_count accepts the short rows, but
// each one still gets a cell for every header column.
func wideCSV(cols, rows int) []byte {
	var b strings.Builder
	for i := range cols {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "c%d", i)
	}
	b.WriteByte('\n')
	b.WriteString(strings.TrimSuffix(strings.Repeat("1,", cols), ","))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("1\n", rows))
	return []byte(b.String())
}

func TestBuildFrameRejectsHugeCSVFrame(t *testing.T) {
	for _, qtype := range []string{"csv", "tsv"} {
		t.Run(qtype, func(t *testing.T) {
			s, err := specFromConfig(fmt.Sprintf(`query "q" {
				url  = "http://x"
				type = %q
				csv_options {
					relax_column_count = true
				}
			}`, qtype))
			require.NoError(t, err)
			body := wideCSV(2_000, 20_000)
			if qtype == typeTSV {
				body = []byte(strings.ReplaceAll(string(body), ",", "\t"))
			}

			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err = buildFrame(s, body)
			runtime.ReadMemStats(&after)

			require.Equal(t, reasonTooLarge, reasonOf(err))
			require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(100<<20), "the check must fail before the frame is allocated")
		})
	}
}

// TestBuildFrameCSVBudget checks that a CSV frame under the budget parses,
// and that skipped bad lines do not count.
func TestBuildFrameCSVBudget(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url  = "http://x"
		type = "csv"
		csv_options {
			skip_lines_with_error = true
		}
	}`)
	require.NoError(t, err)
	// 2,000 rows of 1,000 columns is the budget. The short rows have the
	// wrong field count, so the reader skips them.
	body := string(wideCSV(1_000, 50_000))
	body += strings.Repeat(strings.TrimSuffix(strings.Repeat("1,", 1_000), ",")+"\n", 1_999)
	f, err := buildFrame(s, []byte(body))
	require.NoError(t, err)
	require.Equal(t, 2_000, f.Rows())
}

// TestBuildFrameCSVBadDelimiterDoesNotSpin checks the backstop for a
// delimiter that validation rejects. encoding/csv fails each read without
// moving forward, and csvframer would loop forever when it skips bad lines.
func TestBuildFrameCSVBadDelimiterDoesNotSpin(t *testing.T) {
	s := querySpec{name: "q", qtype: typeCSV, csv: csvSpec{delimiter: `"`, skipLinesWithError: true}}
	done := make(chan error, 1)
	go func() {
		_, err := buildFrame(s, []byte("a,b\n1,2\n"))
		done <- err
	}()
	select {
	case err := <-done:
		require.Equal(t, reasonParse, reasonOf(err))
	case <-time.After(5 * time.Second):
		t.Fatal("buildFrame did not return")
	}
}

// TestLimitRowsMatchesLibrary checks that limitRows gives the same frame as
// transformations.Limit.
func TestLimitRowsMatchesLibrary(t *testing.T) {
	newFrame := func() *data.Frame {
		f := data.NewFrame("q",
			data.NewField("s", data.Labels{"a": "b"}, []*string{}),
			data.NewField("v", nil, []*float64{}),
			data.NewField("b", nil, []bool{}),
		)
		f.Fields[1].SetConfig(&data.FieldConfig{Unit: "ms"})
		for i := range 50 {
			f.AppendRow(ptr(fmt.Sprint(i)), ptr(float64(i)), i%2 == 0)
		}
		return f
	}
	for _, n := range []int{1, 10, 50, 100} {
		want, err := transformations.Limit([]*data.Frame{newFrame()}, transformations.LimitOptions{LimitField: n})
		require.NoError(t, err)
		// The JSON form holds the schema and the values. Equal would also
		// compare the capacity of the slices.
		wantJSON, err := want[0].MarshalJSON()
		require.NoError(t, err)
		gotJSON, err := limitRows(newFrame(), n).MarshalJSON()
		require.NoError(t, err)
		require.JSONEq(t, string(wantJSON), string(gotJSON), "limit %d", n)
	}
}

func TestBuildFrameLimitIsLinear(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://x"
		transform.limit {
			limit = 10
		}
	}`)
	require.NoError(t, err)
	// The library takes about 3 seconds for 200,000 rows.
	body := "[" + strings.TrimSuffix(strings.Repeat("1,", 200_000), ",") + "]"

	start := time.Now()
	f, err := buildFrame(s, []byte(body))
	require.NoError(t, err)
	require.Equal(t, 10, f.Rows())
	require.Less(t, time.Since(start), 2*time.Second)
}

// groupRows returns n rows with groups distinct values of k.
func groupRows(n, groups int) []byte {
	var b strings.Builder
	b.WriteByte('[')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"k":"g%d","v":1}`, i%groups)
	}
	b.WriteByte(']')
	return []byte(b.String())
}

func TestBuildFrameSummarizeByBudget(t *testing.T) {
	for _, where := range []string{"query", "transform"} {
		t.Run(where, func(t *testing.T) {
			cfg := `query "q" {
				url                  = "http://x"
				summarize_expression = "sum(v)"
				summarize_by         = "k"
			}`
			if where == "transform" {
				cfg = `query "q" {
					url = "http://x"
					transform.summarize {
						expression = "sum(v)"
						by         = "k"
					}
				}`
			}
			s, err := specFromConfig(cfg)
			require.NoError(t, err)

			// The library takes about 12 seconds for 20,000 groups of one row.
			start := time.Now()
			_, err = buildFrame(s, groupRows(20_000, 20_000))
			require.Equal(t, reasonTooLarge, reasonOf(err))
			require.Less(t, time.Since(start), 2*time.Second)

			f, err := buildFrame(s, groupRows(1_000, 10))
			require.NoError(t, err)
			require.Equal(t, 10, f.Rows())
		})
	}
}

// TestBuildFrameBooleanColumn checks that "false" in a boolean column is
// false, not null. gframer alone makes it null.
func TestBuildFrameBooleanColumn(t *testing.T) {
	tests := map[string]struct {
		cfg, body string
	}{
		"csv": {`query "q" {
			url  = "http://x"
			type = "csv"
			column {
				selector = "ok"
				text     = "healthy"
				type     = "boolean"
			}
		}`, "ok\ntrue\nfalse\nTRUE\nFalse\nyes\n\n"},
		"json strings": {`query "q" {
			url = "http://x"
			column {
				selector = "ok"
				text     = "healthy"
				type     = "boolean"
			}
		}`, `[{"ok":"true"},{"ok":"false"},{"ok":"TRUE"},{"ok":"False"},{"ok":"yes"},{"ok":null}]`},
		"json booleans": {`query "q" {
			url = "http://x"
			column {
				selector = "ok"
				text     = "healthy"
				type     = "boolean"
			}
		}`, `[{"ok":true},{"ok":false},{"ok":true},{"ok":false},{"ok":1},{}]`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s, err := specFromConfig(tt.cfg)
			require.NoError(t, err)
			f, err := buildFrame(s, []byte(tt.body))
			require.NoError(t, err)
			got, ok := fieldValues(f, "healthy")
			require.True(t, ok)
			// Values other than true and false are null, as in gframer.
			want := []any{true, false, true, false, nil}
			require.Equal(t, want, got[:len(want)])
		})
	}
}
