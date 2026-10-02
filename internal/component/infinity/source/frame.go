package source

import (
	"errors"
	"fmt"
	"strings"

	xj "github.com/basgys/goxml2json"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/infinity-libs/lib/go/csvframer"
	"github.com/grafana/infinity-libs/lib/go/gframer"
	"github.com/grafana/infinity-libs/lib/go/jsonframer"
	"github.com/grafana/infinity-libs/lib/go/transformations"
	"github.com/tidwall/gjson"
)

// This file is the only place that imports infinity-libs. Parsing and
// post-processing follow grafana-infinity-datasource pkg/infinity at commit
// 79f08dff4bce35f73cebd650325320f7b934f62c. Keep the step order the same as
// PostProcessFrame in that commit.

// buildFrame parses body and runs the post-processing steps.
func buildFrame(s querySpec, body []byte) (_ *data.Frame, retErr error) {
	// infinity-libs can panic on some input, for example mixed value types
	// in one column. A bad response must fail the poll, not crash Alloy.
	defer func() {
		if r := recover(); r != nil {
			retErr = newPollError(reasonParse, fmt.Errorf("parser panic: %v", r))
		}
	}()

	frame, err := parseFrame(s, string(body))
	if err != nil {
		var pe *pollError
		if errors.As(err, &pe) {
			return nil, err
		}
		return nil, newPollError(reasonParse, err)
	}
	if frame == nil {
		frame = data.NewFrame(s.name)
	}
	frame, err = postProcess(s, frame)
	if err != nil {
		return nil, newPollError(reasonPostprocess, err)
	}
	if frame == nil {
		frame = data.NewFrame(s.name)
	}
	return frame, nil
}

func parseFrame(s querySpec, body string) (*data.Frame, error) {
	switch s.qtype {
	case "json", typeGraphQL:
		// This is the same check that ToFrame runs first.
		if !gjson.Valid(body) {
			return nil, errors.New("invalid json response received")
		}
		return jsonToFrame(s, body)
	case typeCSV, typeTSV:
		opts := csvframer.FramerOptions{
			FrameName:          s.name,
			Columns:            gframerColumns(s.columns),
			Comment:            s.csv.comment,
			Delimiter:          s.csv.delimiter,
			SkipLinesWithError: s.csv.skipLinesWithError,
			RelaxColumnCount:   s.csv.relaxColumnCount,
		}
		switch s.csv.columns {
		case "":
		case "-", "none":
			opts.NoHeaders = true
		default:
			body = s.csv.columns + "\n" + body
		}
		if s.qtype == typeTSV {
			opts.Delimiter = "\t"
		}
		return csvframer.ToFrame(body, opts)
	case "xml", "html":
		// xmlframer only converts XML to JSON and calls jsonframer. Do the
		// same here, so XML frames get the same size check as JSON.
		j, err := xj.Convert(strings.NewReader(body))
		if err != nil {
			return nil, errors.Join(errors.New("error converting xml to grafana data frame"), err)
		}
		return jsonToFrame(s, j.String())
	}
	return nil, fmt.Errorf("unsupported type %q", s.qtype)
}

// jsonToFrame applies the root selector, checks the frame size and builds
// the frame. ToFrame gets no selector, so the selector runs only once.
func jsonToFrame(s querySpec, body string) (*data.Frame, error) {
	selected, err := jsonframer.GetRootData(body, s.rootSelector, jsonFramerType(s.parser))
	if err != nil {
		return nil, err
	}
	if err := checkFrameSize(selected, len(s.columns)); err != nil {
		return nil, err
	}
	return jsonframer.ToFrame(selected, jsonframer.FramerOptions{
		FramerType: jsonFramerType(s.parser),
		FrameName:  s.name,
		Columns:    jsonColumns(s.columns),
	})
}

// maxFrameCells limits rows times columns of one JSON, GraphQL, XML or
// HTML frame. gframer makes one column for each distinct key in any row,
// and gives every column a cell for every row. A small body with many
// distinct keys can so need many gigabytes. 2 million cells is far more
// than a normal API response needs, but one poll at this budget can still
// use a few hundred MB.
const maxFrameCells = 2_000_000

// checkFrameSize estimates the cells gframer allocates for the JSON in
// selected, before it allocates them. numColumns is the number of column
// blocks; when it is not 0, the frame has only those columns.
func checkFrameSize(selected string, numColumns int) error {
	r := gjson.Parse(selected)
	if !r.IsArray() {
		// One object becomes one row, so the body already bounds it.
		return nil
	}
	rows, cols := 0, numColumns
	keys := map[string]struct{}{}
	var err error
	r.ForEach(func(_, row gjson.Result) bool {
		rows++
		if numColumns == 0 && row.IsObject() {
			row.ForEach(func(k, _ gjson.Result) bool {
				keys[k.Str] = struct{}{}
				return true
			})
			cols = max(1, len(keys))
		}
		if cols == 0 {
			cols = 1
		}
		if rows*cols > maxFrameCells {
			err = newPollError(reasonTooLarge, fmt.Errorf("the response makes a frame with more than %d cells (rows times columns)", maxFrameCells))
			return false
		}
		return true
	})
	return err
}

func postProcess(s querySpec, frame *data.Frame) (*data.Frame, error) {
	computed := make([]transformations.ComputedColumn, 0, len(s.computed))
	for _, c := range s.computed {
		computed = append(computed, transformations.ComputedColumn{Selector: c.selector, Text: c.text})
	}
	frame, err := transformations.GetFrameWithComputedColumns(frame, computed)
	if err != nil {
		return nil, fmt.Errorf("computed columns: %w", err)
	}
	frame, err = transformations.ApplyFilter(frame, s.filter)
	if err != nil {
		return nil, fmt.Errorf("filter_expression: %w", err)
	}
	if s.summarize != nil {
		frame, err = transformations.GetSummaryFrame(frame, s.summarize.expression, s.summarize.by, s.summarize.alias)
		if err != nil {
			return nil, fmt.Errorf("summarize: %w", err)
		}
	}
	for i, t := range s.transforms {
		frame, err = applyTransform(frame, t)
		if err != nil {
			return nil, fmt.Errorf("transform %d (%s): %w", i, t.kind, err)
		}
	}
	return frame, nil
}

// applyTransform calls the same library functions as the plugin's
// ApplyTransformation, but on one frame only.
func applyTransform(frame *data.Frame, t transformSpec) (*data.Frame, error) {
	switch t.kind {
	case "limit":
		return firstFrame(transformations.Limit([]*data.Frame{frame}, transformations.LimitOptions{LimitField: t.limit}))
	case "filter":
		return firstFrame(transformations.FilterExpression([]*data.Frame{frame}, transformations.FilterExpressionOptions{Expression: t.expression}))
	case "computed_column":
		return transformations.GetFrameWithComputedColumns(frame, []transformations.ComputedColumn{{Selector: t.expression, Text: t.alias}})
	case "summarize":
		return transformations.GetSummaryFrame(frame, t.expression, t.by, t.alias)
	}
	return nil, fmt.Errorf("unknown transform %q", t.kind)
}

func firstFrame(frames []*data.Frame, err error) (*data.Frame, error) {
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return data.NewFrame(""), nil
	}
	return frames[0], nil
}

func jsonFramerType(parser string) jsonframer.FramerType {
	if parser == "jq-backend" {
		return jsonframer.FramerTypeJQ
	}
	return jsonframer.FramerTypeGJSON
}

func jsonColumns(cols []columnSpec) []jsonframer.ColumnSelector {
	out := make([]jsonframer.ColumnSelector, 0, len(cols))
	for _, c := range cols {
		out = append(out, jsonframer.ColumnSelector{Selector: c.selector, Alias: c.alias, Type: c.typ, TimeFormat: c.timeFormat})
	}
	return out
}

func gframerColumns(cols []columnSpec) []gframer.ColumnSelector {
	out := make([]gframer.ColumnSelector, 0, len(cols))
	for _, c := range cols {
		out = append(out, gframer.ColumnSelector{Selector: c.selector, Alias: c.alias, Type: c.typ, TimeFormat: c.timeFormat})
	}
	return out
}
