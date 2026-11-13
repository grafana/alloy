package source

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/grafana/alloy/syntax/alloytypes"
)

// querySpec is the validated internal form of a query block. It holds the
// config blocks that have no secrets as they are. The other fields hold
// values that the conversion makes, for example unwrapped secrets.
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
	columns      []Column
	steps        []transformStep
	csv          CSVOptions

	metrics MetricsBlock
	logs    LogsBlock
}

// transformStep is one post-processing step. The fixed fields of a query
// become steps too, so that one loop runs all of them.
type transformStep struct {
	label     string // starts the error message of the step
	transform Transform
}

// newQuerySpec converts a query block. The block must be validated first.
// The spec gets its own copy of each slice and block, so a later change to
// the config cannot change a poll that runs.
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
		graphqlVariables: q.URLOptions.BodyGraphQLVariables.Value,
		rootSelector:     q.RootSelector,
		columns:          slices.Clone(q.Columns),
		csv:              q.CSVOptions,
		// applyDefaults sets this line column only in a logs block that
		// the user writes.
		logs: LogsBlock{LineColumn: "body"},
	}
	if !hasHeader(q.URLOptions.Headers, "Accept") {
		s.accept = defaultAccept(q.Type)
	}
	// The fixed fields run before the transform list, in the order of
	// PostProcessFrame in the plugin. Each computed column is its own step,
	// so it sees the computed columns before it.
	for _, c := range q.ComputedColumns {
		s.steps = append(s.steps, transformStep{"computed columns", Transform{ComputedColumn: &ComputedColumnTransform{Expression: c.Selector, Alias: c.Text}}})
	}
	// An empty filter does not change a frame that is not nil, and
	// buildFrame never gives postProcess a nil frame. So it needs no step.
	if q.FilterExpression != "" {
		s.steps = append(s.steps, transformStep{"filter_expression", Transform{Filter: &FilterTransform{Expression: q.FilterExpression}}})
	}
	if q.SummarizeExpression != "" {
		alias := q.SummarizeAlias
		if alias == "" {
			// The plugin uses the same default alias. A summarize transform
			// has no default, so the library uses its expression.
			alias = "summary"
		}
		s.steps = append(s.steps, transformStep{"summarize", Transform{Summarize: &SummarizeTransform{Expression: q.SummarizeExpression, By: q.SummarizeBy, Alias: alias}}})
	}
	for i, t := range q.Transforms {
		s.steps = append(s.steps, transformStep{fmt.Sprintf("transform %d (%s)", i, t.kind()), t.clone()})
	}
	if q.Metrics != nil {
		s.metrics = *q.Metrics
	}
	if q.Logs != nil {
		s.logs = *q.Logs
		s.logs.LabelColumns = slices.Clone(q.Logs.LabelColumns)
		s.logs.StructuredMetadataColumns = slices.Clone(q.Logs.StructuredMetadataColumns)
	}
	return s
}

// clone returns t with its own copy of the block that t holds.
func (t Transform) clone() Transform {
	return Transform{Limit: clonePtr(t.Limit), Filter: clonePtr(t.Filter), Summarize: clonePtr(t.Summarize), ComputedColumn: clonePtr(t.ComputedColumn)}
}

// kind returns the name of the transform block for error messages.
func (t Transform) kind() string {
	switch {
	case t.Limit != nil:
		return "limit"
	case t.Filter != nil:
		return "filter"
	case t.Summarize != nil:
		return "summarize"
	case t.ComputedColumn != nil:
		return "computed_column"
	}
	return ""
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
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
