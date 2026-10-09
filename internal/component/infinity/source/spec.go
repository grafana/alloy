package source

import (
	"net/http"

	"github.com/grafana/alloy/syntax/alloytypes"
)

// querySpec is the validated internal form of a query block. It does not
// depend on Alloy syntax types, so the pipeline can be tested on its own.
type querySpec struct {
	name   string
	qtype  string
	parser string
	format string
	source string
	url    string
	data   string

	method           string
	params           map[string]string
	headers          map[string]string
	accept           string // default Accept header, empty if the user sets one
	bodyType         string
	body             string
	bodyContentType  string
	bodyForm         map[string]string
	graphqlQuery     string
	graphqlVariables string

	rootSelector string
	columns      []columnSpec
	computed     []computedSpec
	filter       string
	summarize    *summarizeSpec
	transforms   []transformSpec
	csv          csvSpec

	metrics metricsSpec
	logs    logsSpec
}

type columnSpec struct {
	selector, alias, typ, timeFormat string
}

type computedSpec struct {
	selector, text string
}

type summarizeSpec struct {
	expression, by, alias string
}

type transformSpec struct {
	kind       string // limit, filter, summarize or computed_column
	limit      int
	expression string
	by         string
	alias      string
}

type csvSpec struct {
	delimiter          string
	skipLinesWithError bool
	relaxColumnCount   bool
	columns            string
	comment            string
}

type metricsSpec struct {
	prefix      string
	seriesLimit int
}

type logsSpec struct {
	lineColumn                string
	labelColumns              []string
	structuredMetadataColumns []string
	entryLimit                int
}

// newQuerySpec converts a query block. The block must be validated first.
func newQuerySpec(q QueryBlock) querySpec {
	s := querySpec{
		name:             q.Name,
		qtype:            q.Type,
		parser:           q.Parser,
		format:           q.Format,
		source:           q.Source,
		url:              q.URL,
		data:             q.Data,
		method:           q.URLOptions.Method,
		params:           unwrapSecrets(q.URLOptions.Params),
		headers:          unwrapSecrets(q.URLOptions.Headers),
		bodyType:         q.URLOptions.BodyType,
		body:             q.URLOptions.Body.Value,
		bodyContentType:  q.URLOptions.BodyContentType,
		bodyForm:         unwrapSecrets(q.URLOptions.BodyForm),
		graphqlQuery:     q.URLOptions.BodyGraphQLQuery.Value,
		graphqlVariables: q.URLOptions.BodyGraphQLVariables,
		rootSelector:     q.RootSelector,
		filter:           q.FilterExpression,
		csv: csvSpec{
			delimiter:          q.CSVOptions.Delimiter,
			skipLinesWithError: q.CSVOptions.SkipLinesWithError,
			relaxColumnCount:   q.CSVOptions.RelaxColumnCount,
			columns:            q.CSVOptions.Columns,
			comment:            q.CSVOptions.Comment,
		},
		logs: logsSpec{lineColumn: "body"},
	}
	if !hasHeader(q.URLOptions.Headers, "Accept") {
		s.accept = defaultAccept(q.Type)
	}
	for _, c := range q.Columns {
		s.columns = append(s.columns, columnSpec{selector: c.Selector, alias: c.Text, typ: c.Type, timeFormat: c.TimestampFormat})
	}
	for _, c := range q.ComputedColumns {
		s.computed = append(s.computed, computedSpec{selector: c.Selector, text: c.Text})
	}
	if q.SummarizeExpression != "" {
		alias := q.SummarizeAlias
		if alias == "" {
			// The plugin uses the same default alias.
			alias = "summary"
		}
		s.summarize = &summarizeSpec{expression: q.SummarizeExpression, by: q.SummarizeBy, alias: alias}
	}
	for _, t := range q.Transforms {
		switch {
		case t.Limit != nil:
			s.transforms = append(s.transforms, transformSpec{kind: "limit", limit: t.Limit.Limit})
		case t.Filter != nil:
			s.transforms = append(s.transforms, transformSpec{kind: "filter", expression: t.Filter.Expression})
		case t.Summarize != nil:
			s.transforms = append(s.transforms, transformSpec{kind: "summarize", expression: t.Summarize.Expression, by: t.Summarize.By, alias: t.Summarize.Alias})
		case t.ComputedColumn != nil:
			s.transforms = append(s.transforms, transformSpec{kind: "computed_column", expression: t.ComputedColumn.Expression, alias: t.ComputedColumn.Alias})
		}
	}
	if q.Metrics != nil {
		s.metrics = metricsSpec{prefix: q.Metrics.Prefix, seriesLimit: q.Metrics.SeriesLimit}
	}
	if q.Logs != nil {
		s.logs = logsSpec{
			lineColumn:                q.Logs.LineColumn,
			labelColumns:              q.Logs.LabelColumns,
			structuredMetadataColumns: q.Logs.StructuredMetadataColumns,
			entryLimit:                q.Logs.EntryLimit,
		}
	}
	return s
}

// defaultAccept returns the Accept header that the plugin sends for each
// type.
func defaultAccept(qtype string) string {
	switch qtype {
	case typeCSV, typeTSV:
		return "text/csv"
	case "xml", "html":
		return "text/xml;q=0.9,text/plain"
	}
	return "application/json;q=0.9,text/plain"
}

func hasHeader[V any](headers map[string]V, name string) bool {
	for k := range headers {
		if http.CanonicalHeaderKey(k) == name {
			return true
		}
	}
	return false
}

func unwrapSecrets(in map[string]alloytypes.Secret) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}
