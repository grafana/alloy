package source

import (
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/infinity-libs/lib/go/csvframer"
	"github.com/grafana/infinity-libs/lib/go/gframer"
	"github.com/grafana/infinity-libs/lib/go/jsonframer"
	"github.com/grafana/infinity-libs/lib/go/transformations"
	"github.com/grafana/infinity-libs/lib/go/xmlframer"
)

// This file is the only place that imports infinity-libs. Parsing and
// post-processing follow grafana-infinity-datasource pkg/infinity at commit
// 79f08dff4bce35f73cebd650325320f7b934f62c. Keep the step order the same as
// PostProcessFrame in that commit.

// buildFrame parses body and runs the post-processing steps.
func buildFrame(s querySpec, body []byte) (*data.Frame, error) {
	frame, err := parseFrame(s, string(body))
	if err != nil {
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
	case "json", "graphql":
		return jsonframer.ToFrame(body, jsonframer.FramerOptions{
			FramerType:   jsonFramerType(s.parser),
			FrameName:    s.name,
			RootSelector: s.rootSelector,
			Columns:      jsonColumns(s.columns),
		})
	case "csv", "tsv":
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
		if s.qtype == "tsv" {
			opts.Delimiter = "\t"
		}
		return csvframer.ToFrame(body, opts)
	case "xml", "html":
		return xmlframer.ToFrame(body, xmlframer.FramerOptions{
			FramerType:   string(jsonFramerType(s.parser)),
			FrameName:    s.name,
			RootSelector: s.rootSelector,
			Columns:      jsonColumns(s.columns),
		})
	}
	return nil, fmt.Errorf("unsupported type %q", s.qtype)
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
