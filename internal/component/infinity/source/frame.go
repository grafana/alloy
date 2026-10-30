package source

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	xj "github.com/basgys/goxml2json"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/infinity-libs/lib/go/csvframer"
	"github.com/grafana/infinity-libs/lib/go/framesql"
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
			retErr = newLibError(reasonParse, fmt.Errorf("parser panic: %v", r))
		}
	}()

	frame, err := parseFrame(s, string(body))
	if err != nil {
		var pe *pollError
		if errors.As(err, &pe) {
			return nil, err
		}
		return nil, newLibError(reasonParse, err)
	}
	if frame == nil {
		frame = data.NewFrame(s.name)
	}
	toBoolColumns(frame, s.columns)
	frame, err = postProcess(s, frame)
	if err != nil {
		var pe *pollError
		if errors.As(err, &pe) {
			return nil, err
		}
		return nil, newLibError(reasonPostprocess, err)
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
		if err := checkCSVSize(body, opts); err != nil {
			return nil, err
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

// maxFrameCells limits rows times columns of one frame. The framers give
// every column a cell for every row. For JSON, gframer makes one column for
// each distinct key in any row, so a small body with many distinct keys can
// need many gigabytes. 2 million cells is far more than a normal API
// response needs, but one poll at this budget can still use a few hundred
// MB.
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
			err = tooManyCells()
			return false
		}
		return true
	})
	return err
}

// checkCSVSize reads body with the same reader settings as csvframer and
// counts the cells of the frame, before csvframer allocates them. Each
// record gets a cell for each header column, even a short record that
// relax_column_count allows.
func checkCSVSize(body string, opts csvframer.FramerOptions) error {
	r := csv.NewReader(strings.NewReader(body))
	r.LazyQuotes = true
	r.ReuseRecord = true
	if opts.Comment != "" {
		r.Comment = rune(opts.Comment[0])
	}
	if opts.Delimiter != "" {
		r.Comma = rune(opts.Delimiter[0])
	}
	if opts.RelaxColumnCount {
		r.FieldsPerRecord = -1
	}
	rows, cols := 0, -1
	for {
		record, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) && opts.SkipLinesWithError {
				continue
			}
			// csvframer reports the error, or would loop forever on an
			// error that does not move the reader forward.
			return newLibError(reasonParse, errors.Join(csvframer.ErrReadingCsvResponse, err))
		}
		if cols < 0 {
			cols = max(1, len(record))
			if !opts.NoHeaders {
				continue
			}
		}
		rows++
		if rows*cols > maxFrameCells {
			return tooManyCells()
		}
	}
}

func tooManyCells() error {
	return newPollError(reasonTooLarge, fmt.Errorf("the response makes a frame with more than %d cells (rows times columns)", maxFrameCells))
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
		frame, err = summarize(frame, s.summarize.expression, s.summarize.by, s.summarize.alias)
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
		return limitRows(frame, t.limit), nil
	case "filter":
		return firstFrame(transformations.FilterExpression([]*data.Frame{frame}, transformations.FilterExpressionOptions{Expression: t.expression}))
	case "computed_column":
		return transformations.GetFrameWithComputedColumns(frame, []transformations.ComputedColumn{{Selector: t.expression, Text: t.alias}})
	case "summarize":
		return summarize(frame, t.expression, t.by, t.alias)
	}
	return nil, fmt.Errorf("unknown transform %q", t.kind)
}

// limitRows keeps the first n rows of frame. It gives the same result as
// transformations.Limit, which deletes one row at a time and so takes time
// that grows with the square of the row count.
func limitRows(frame *data.Frame, n int) *data.Frame {
	if frame.Rows() <= n {
		return frame
	}
	// EmptyCopy would drop the field config, so copy each field here.
	out := &data.Frame{Name: frame.Name, RefID: frame.RefID, Meta: frame.Meta}
	for _, fld := range frame.Fields {
		f := data.NewFieldFromFieldType(fld.Type(), n)
		f.Name, f.Labels, f.Config = fld.Name, fld.Labels, fld.Config
		for i := range n {
			f.Set(i, fld.CopyAt(i))
		}
		out.Fields = append(out.Fields, f)
	}
	return out
}

// maxSummarizeWork limits groups times rows of one summarize_by step. The
// library scans all rows once for each group, so a column with a different
// value in each row makes the time grow with the square of the row count.
const maxSummarizeWork = 10_000_000

// summarize calls transformations.GetSummaryFrame, after it checks the
// work that summarize_by needs.
func summarize(frame *data.Frame, expression, by, alias string) (*data.Frame, error) {
	if strings.TrimSpace(by) != "" && strings.TrimSpace(expression) != "" && frame != nil {
		if groups := countGroups(frame, by); groups*frame.Rows() > maxSummarizeWork {
			return nil, newPollError(reasonTooLarge, fmt.Errorf("summarize_by makes more than %d group times row steps", maxSummarizeWork))
		}
	}
	return transformations.GetSummaryFrame(frame, expression, by, alias)
}

// countGroups counts the distinct values of the by field. It finds the
// field the same way as GetSummarizeByFrame. The count is an estimate,
// because the library compares values in a different form.
func countGroups(frame *data.Frame, by string) int {
	for _, fld := range frame.Fields {
		if fld.Name != by && framesql.SlugifyFieldName(fld.Name) != by {
			continue
		}
		seen := map[string]struct{}{}
		for i := range fld.Len() {
			v, ok := fld.ConcreteAt(i)
			if !ok {
				v = nil
			}
			seen[fmt.Sprint(v)] = struct{}{}
		}
		return len(seen)
	}
	return 0
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
		out = append(out, jsonframer.ColumnSelector{Selector: c.selector, Alias: c.alias, Type: framerType(c.typ), TimeFormat: c.timeFormat})
	}
	return out
}

func gframerColumns(cols []columnSpec) []gframer.ColumnSelector {
	out := make([]gframer.ColumnSelector, 0, len(cols))
	for _, c := range cols {
		out = append(out, gframer.ColumnSelector{Selector: c.selector, Alias: c.alias, Type: framerType(c.typ), TimeFormat: c.timeFormat})
	}
	return out
}

// framerType is the column type that the framer gets. gframer makes the
// text "false" in a boolean column null, so boolean columns are read as
// strings and toBoolColumns converts them.
func framerType(typ string) string {
	if typ == columnTypeBoolean {
		return "string"
	}
	return typ
}

// toBoolColumns converts each boolean column of frame from the strings that
// the framer made. "true" and "false" in any case become booleans. Other
// values become null, as gframer does.
func toBoolColumns(frame *data.Frame, cols []columnSpec) {
	for _, c := range cols {
		if c.typ != columnTypeBoolean {
			continue
		}
		name := c.alias
		if name == "" {
			name = c.selector
		}
		for i, fld := range frame.Fields {
			if fld.Name != name || fld.Type().NonNullableType() != data.FieldTypeString {
				continue
			}
			b := data.NewFieldFromFieldType(data.FieldTypeNullableBool, fld.Len())
			b.Name, b.Labels, b.Config = fld.Name, fld.Labels, fld.Config
			for row := range fld.Len() {
				v, ok := fld.ConcreteAt(row)
				if !ok {
					continue
				}
				switch strings.ToLower(v.(string)) {
				case "true":
					b.SetConcrete(row, true)
				case "false":
					b.SetConcrete(row, false)
				}
			}
			frame.Fields[i] = b
		}
	}
}
