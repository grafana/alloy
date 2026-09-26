package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/units"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// Arguments configures the infinity.source component.
type Arguments struct {
	Interval        time.Duration           `alloy:"interval,attr,optional"`
	Timeout         time.Duration           `alloy:"timeout,attr,optional"`
	MaxResponseSize units.Base2Bytes        `alloy:"max_response_size,attr,optional"`
	Client          config.HTTPClientConfig `alloy:"client,block,optional"`
	Clustering      cluster.ComponentBlock  `alloy:"clustering,block,optional"`
	Queries         []QueryBlock            `alloy:"query,block"`
	ForwardTo       ForwardTo               `alloy:"forward_to,block,optional"`
	Output          Output                  `alloy:"output,block,optional"`
}

// ForwardTo holds the Alloy-native receivers.
type ForwardTo struct {
	Metrics []storage.Appendable `alloy:"metrics,attr,optional"`
	Logs    []loki.LogsReceiver  `alloy:"logs,attr,optional"`
}

// Output holds the OTel consumers.
type Output struct {
	Metrics []otelcol.Consumer `alloy:"metrics,attr,optional"`
	Logs    []otelcol.Consumer `alloy:"logs,attr,optional"`
}

// QueryBlock is one Infinity query.
type QueryBlock struct {
	Name                string           `alloy:",label"`
	Type                string           `alloy:"type,attr,optional"`
	Parser              string           `alloy:"parser,attr,optional"`
	Format              string           `alloy:"format,attr,optional"`
	Source              string           `alloy:"source,attr,optional"`
	URL                 string           `alloy:"url,attr,optional"`
	Data                string           `alloy:"data,attr,optional"`
	RootSelector        string           `alloy:"root_selector,attr,optional"`
	FilterExpression    string           `alloy:"filter_expression,attr,optional"`
	SummarizeExpression string           `alloy:"summarize_expression,attr,optional"`
	SummarizeBy         string           `alloy:"summarize_by,attr,optional"`
	SummarizeAlias      string           `alloy:"summarize_alias,attr,optional"`
	CSVOptions          CSVOptions       `alloy:"csv_options,block,optional"`
	URLOptions          URLOptions       `alloy:"url_options,block,optional"`
	Columns             []Column         `alloy:"column,block,optional"`
	ComputedColumns     []ComputedColumn `alloy:"computed_column,block,optional"`
	Transforms          []Transform      `alloy:"transform,enum,optional"`
	Metrics             *MetricsBlock    `alloy:"metrics,block,optional"`
	Logs                *LogsBlock       `alloy:"logs,block,optional"`
}

// CSVOptions holds the options that csvframer supports.
type CSVOptions struct {
	Delimiter          string `alloy:"delimiter,attr,optional"`
	SkipLinesWithError bool   `alloy:"skip_lines_with_error,attr,optional"`
	RelaxColumnCount   bool   `alloy:"relax_column_count,attr,optional"`
	Columns            string `alloy:"columns,attr,optional"`
	Comment            string `alloy:"comment,attr,optional"`
}

// URLOptions configures the HTTP request.
type URLOptions struct {
	Method               string                       `alloy:"method,attr,optional"`
	Headers              map[string]alloytypes.Secret `alloy:"headers,attr,optional"`
	Params               map[string]alloytypes.Secret `alloy:"params,attr,optional"`
	BodyType             string                       `alloy:"body_type,attr,optional"`
	Body                 alloytypes.OptionalSecret    `alloy:"body,attr,optional"`
	BodyContentType      string                       `alloy:"body_content_type,attr,optional"`
	BodyForm             map[string]alloytypes.Secret `alloy:"body_form,attr,optional"`
	BodyGraphQLQuery     alloytypes.OptionalSecret    `alloy:"body_graphql_query,attr,optional"`
	BodyGraphQLVariables string                       `alloy:"body_graphql_variables,attr,optional"`
}

// Column selects one field from the response.
type Column struct {
	Selector        string `alloy:"selector,attr"`
	Text            string `alloy:"text,attr,optional"`
	Type            string `alloy:"type,attr,optional"`
	TimestampFormat string `alloy:"timestamp_format,attr,optional"`
}

// ComputedColumn adds a field from an expression.
type ComputedColumn struct {
	Selector string `alloy:"selector,attr"`
	Text     string `alloy:"text,attr"`
}

// Transform is one element of the ordered transform list. The enum decoder
// sets exactly one field for each element.
type Transform struct {
	Limit          *LimitTransform          `alloy:"limit,block,optional"`
	Filter         *FilterTransform         `alloy:"filter,block,optional"`
	Summarize      *SummarizeTransform      `alloy:"summarize,block,optional"`
	ComputedColumn *ComputedColumnTransform `alloy:"computed_column,block,optional"`
}

type LimitTransform struct {
	Limit int `alloy:"limit,attr"`
}

type FilterTransform struct {
	Expression string `alloy:"expression,attr"`
}

type SummarizeTransform struct {
	Expression string `alloy:"expression,attr"`
	By         string `alloy:"by,attr,optional"`
	Alias      string `alloy:"alias,attr,optional"`
}

type ComputedColumnTransform struct {
	Expression string `alloy:"expression,attr"`
	Alias      string `alloy:"alias,attr"`
}

// MetricsBlock holds settings for format "table".
type MetricsBlock struct {
	Prefix      string `alloy:"prefix,attr,optional"`
	SeriesLimit int    `alloy:"series_limit,attr,optional"`
}

// LogsBlock holds settings for format "logs".
type LogsBlock struct {
	LineColumn                string   `alloy:"line_column,attr,optional"`
	LabelColumns              []string `alloy:"label_columns,attr,optional"`
	StructuredMetadataColumns []string `alloy:"structured_metadata_columns,attr,optional"`
	EntryLimit                int      `alloy:"entry_limit,attr,optional"`
}

// DefaultArguments holds the component defaults.
var DefaultArguments = Arguments{
	Interval:        60 * time.Second,
	Timeout:         10 * time.Second,
	MaxResponseSize: 10 * units.MiB,
	Client:          config.DefaultHTTPClientConfig,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Query format and type values that appear in more than one place.
const (
	formatTable = "table"
	formatLogs  = "logs"
	sourceURL   = "url"
	typeGraphQL = "graphql"
	typeCSV     = "csv"
	typeTSV     = "tsv"
	bodyTypeRaw = "raw"
)

var (
	supportedTypes       = []string{"json", typeCSV, typeTSV, "xml", "html", typeGraphQL}
	supportedParsers     = []string{"backend", "jq-backend"}
	supportedFormats     = []string{formatTable, formatLogs}
	supportedSources     = []string{sourceURL, "inline"}
	supportedMethods     = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	supportedBodyTypes   = []string{bodyTypeRaw, "form-data", "x-www-form-urlencoded", typeGraphQL}
	supportedColumnTypes = []string{"string", "number", "boolean", "timestamp", "timestamp_epoch", "timestamp_epoch_s"}

	metricPrefixRE = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
)

// Validate implements syntax.Validator. It also applies query defaults.
// They run here and not in a Defaulter, because some of them depend on
// type, for example graphql uses POST.
func (a *Arguments) Validate() error {
	var errs []error
	if a.Interval <= 0 {
		errs = append(errs, errors.New("interval must be greater than 0"))
	}
	if a.Timeout <= 0 {
		errs = append(errs, errors.New("timeout must be greater than 0"))
	}
	if a.Timeout > a.Interval {
		errs = append(errs, errors.New("timeout must be less than or equal to interval"))
	}
	if a.MaxResponseSize <= 0 {
		errs = append(errs, errors.New("max_response_size must be greater than 0"))
	}
	if err := a.Client.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("client: %w", err))
	}
	if len(a.Queries) == 0 {
		errs = append(errs, errors.New("at least one query block is required"))
	}

	clientHeaders := map[string]struct{}{}
	if a.Client.HTTPHeaders != nil {
		for k := range a.Client.HTTPHeaders.Headers {
			clientHeaders[http.CanonicalHeaderKey(k)] = struct{}{}
		}
	}

	seen := map[string]struct{}{}
	for i := range a.Queries {
		q := &a.Queries[i]
		// Check this before applyDefaults sets method and body_type.
		urlOptionsSet := !q.URLOptions.isZero()
		q.applyDefaults()
		if _, dup := seen[q.Name]; dup {
			errs = append(errs, fmt.Errorf("duplicate query name %q", q.Name))
		}
		seen[q.Name] = struct{}{}
		if err := q.validate(clientHeaders, urlOptionsSet); err != nil {
			errs = append(errs, fmt.Errorf("query %q: %w", q.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (q *QueryBlock) applyDefaults() {
	if q.Type == "" {
		q.Type = "json"
	}
	if q.Parser == "" {
		q.Parser = "backend"
	}
	if q.Format == "" {
		q.Format = formatTable
	}
	if q.Source == "" {
		q.Source = sourceURL
	}
	q.URLOptions.Method = strings.ToUpper(strings.TrimSpace(q.URLOptions.Method))
	if q.URLOptions.Method == "" {
		q.URLOptions.Method = http.MethodGet
		if q.Type == typeGraphQL {
			q.URLOptions.Method = http.MethodPost
		}
	}
	if q.URLOptions.BodyType == "" {
		q.URLOptions.BodyType = bodyTypeRaw
		if q.Type == typeGraphQL {
			q.URLOptions.BodyType = typeGraphQL
		}
	}
	if q.Logs != nil && q.Logs.LineColumn == "" {
		q.Logs.LineColumn = "body"
	}
}

func (q *QueryBlock) validate(clientHeaders map[string]struct{}, urlOptionsSet bool) error {
	var errs []error

	if !slices.Contains(supportedTypes, q.Type) {
		switch q.Type {
		case "uql", "groq", "google-sheets":
			errs = append(errs, fmt.Errorf("type %q is not supported in Alloy", q.Type))
		default:
			errs = append(errs, fmt.Errorf("unknown type %q, must be one of %s", q.Type, strings.Join(supportedTypes, ", ")))
		}
	}
	if !slices.Contains(supportedParsers, q.Parser) {
		switch q.Parser {
		case "simple", "uql", "groq":
			errs = append(errs, fmt.Errorf("parser %q is frontend-only in Infinity and is not supported in Alloy", q.Parser))
		default:
			errs = append(errs, fmt.Errorf("unknown parser %q, must be one of %s", q.Parser, strings.Join(supportedParsers, ", ")))
		}
	}
	if !slices.Contains(supportedFormats, q.Format) {
		if q.Format == "timeseries" {
			errs = append(errs, errors.New(`format "timeseries" is not supported, use "table" for current-state metrics`))
		} else {
			errs = append(errs, fmt.Errorf("unknown format %q, must be one of %s", q.Format, strings.Join(supportedFormats, ", ")))
		}
	}

	switch q.Source {
	case sourceURL:
		if q.Data != "" {
			errs = append(errs, errors.New(`data must be empty when source is "url"`))
		}
		u, err := url.Parse(q.URL)
		if q.URL == "" || err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			errs = append(errs, errors.New("url must be an absolute http or https URL"))
		} else {
			existing := u.Query()
			for k := range q.URLOptions.Params {
				if existing.Has(k) {
					errs = append(errs, fmt.Errorf("url_options.params key %q is also in the url query", k))
				}
			}
		}
	case "inline":
		if q.Data == "" {
			errs = append(errs, errors.New(`data is required when source is "inline"`))
		}
		if q.URL != "" {
			errs = append(errs, errors.New(`url must be empty when source is "inline"`))
		}
		if urlOptionsSet {
			errs = append(errs, errors.New(`url_options must be empty when source is "inline"`))
		}
	case "azure-blob", "reference", "expression", "random-walk":
		errs = append(errs, fmt.Errorf("source %q is not supported in Alloy", q.Source))
	default:
		errs = append(errs, fmt.Errorf("unknown source %q, must be one of %s", q.Source, strings.Join(supportedSources, ", ")))
	}

	if q.Metrics != nil {
		if q.Format != formatTable {
			errs = append(errs, errors.New(`metrics block requires format "table"`))
		}
		if q.Metrics.SeriesLimit < 0 {
			errs = append(errs, errors.New("metrics.series_limit must not be negative"))
		}
		if q.Metrics.Prefix != "" && !metricPrefixRE.MatchString(q.Metrics.Prefix) {
			errs = append(errs, fmt.Errorf("metrics.prefix %q is not a valid metric name prefix", q.Metrics.Prefix))
		}
	}
	if q.Logs != nil {
		if q.Format != formatLogs {
			errs = append(errs, errors.New(`logs block requires format "logs"`))
		}
		if q.Logs.EntryLimit < 0 {
			errs = append(errs, errors.New("logs.entry_limit must not be negative"))
		}
		for _, c := range q.Logs.LabelColumns {
			if slices.Contains(q.Logs.StructuredMetadataColumns, c) {
				errs = append(errs, fmt.Errorf("column %q is in both label_columns and structured_metadata_columns", c))
			}
		}
	}

	for _, c := range q.Columns {
		if c.Type != "" && !slices.Contains(supportedColumnTypes, c.Type) {
			errs = append(errs, fmt.Errorf("column %q: unknown type %q", c.Selector, c.Type))
		}
		if c.TimestampFormat != "" && c.Type != "timestamp" {
			errs = append(errs, fmt.Errorf("column %q: timestamp_format requires type \"timestamp\"", c.Selector))
		}
	}
	for i, t := range q.Transforms {
		if t.kinds() != 1 {
			errs = append(errs, fmt.Errorf("transform %d must set exactly one kind", i))
		}
		// The library changes a limit of 0 or less to 10 without an error.
		if t.Limit != nil && t.Limit.Limit <= 0 {
			errs = append(errs, fmt.Errorf("transform %d: limit must be greater than 0", i))
		}
	}
	if q.SummarizeExpression == "" && (q.SummarizeBy != "" || q.SummarizeAlias != "") {
		errs = append(errs, errors.New("summarize_by and summarize_alias require summarize_expression"))
	}
	if q.CSVOptions != (CSVOptions{}) && q.Type != typeCSV && q.Type != typeTSV {
		errs = append(errs, errors.New(`csv_options requires type "csv" or "tsv"`))
	}

	errs = append(errs, q.URLOptions.validate(q.Type, q.Source, clientHeaders)...)
	return errors.Join(errs...)
}

func (t Transform) kinds() int {
	n := 0
	for _, set := range []bool{t.Limit != nil, t.Filter != nil, t.Summarize != nil, t.ComputedColumn != nil} {
		if set {
			n++
		}
	}
	return n
}

func (o *URLOptions) isZero() bool {
	return o.Method == "" && len(o.Headers) == 0 && len(o.Params) == 0 && o.BodyType == "" &&
		o.Body.Value == "" && o.BodyContentType == "" && len(o.BodyForm) == 0 &&
		o.BodyGraphQLQuery.Value == "" && o.BodyGraphQLVariables == ""
}

func (o *URLOptions) validate(queryType, source string, clientHeaders map[string]struct{}) []error {
	var errs []error
	if !slices.Contains(supportedMethods, o.Method) {
		errs = append(errs, fmt.Errorf("unknown method %q, must be one of %s", o.Method, strings.Join(supportedMethods, ", ")))
	}
	if !slices.Contains(supportedBodyTypes, o.BodyType) {
		errs = append(errs, fmt.Errorf("unknown body_type %q, must be one of %s", o.BodyType, strings.Join(supportedBodyTypes, ", ")))
	}
	hasBody := o.Body.Value != "" || len(o.BodyForm) > 0 || o.BodyGraphQLQuery.Value != ""
	if o.Method == http.MethodGet && hasBody {
		errs = append(errs, errors.New("a request body is not allowed with method GET"))
	}
	if queryType == typeGraphQL {
		if o.Method != http.MethodPost {
			errs = append(errs, errors.New(`type "graphql" requires method "POST"`))
		}
		if o.BodyType != typeGraphQL {
			errs = append(errs, errors.New(`type "graphql" requires body_type "graphql"`))
		}
		// An inline source sends no request, so it needs no query.
		if source == sourceURL && o.BodyGraphQLQuery.Value == "" {
			errs = append(errs, errors.New(`type "graphql" requires body_graphql_query`))
		}
	}
	if o.BodyType != bodyTypeRaw && (o.Body.Value != "" || o.BodyContentType != "") {
		errs = append(errs, errors.New(`body and body_content_type require body_type "raw"`))
	}
	if o.BodyType != "form-data" && o.BodyType != "x-www-form-urlencoded" && len(o.BodyForm) > 0 {
		errs = append(errs, errors.New(`body_form requires body_type "form-data" or "x-www-form-urlencoded"`))
	}
	if o.BodyType != typeGraphQL && (o.BodyGraphQLQuery.Value != "" || o.BodyGraphQLVariables != "") {
		errs = append(errs, errors.New(`body_graphql_query and body_graphql_variables require body_type "graphql"`))
	}
	if o.BodyGraphQLVariables != "" && !json.Valid([]byte(o.BodyGraphQLVariables)) {
		errs = append(errs, errors.New("body_graphql_variables must be valid JSON"))
	}
	canonical := map[string]string{}
	for _, k := range slices.Sorted(maps.Keys(o.Headers)) {
		ck := http.CanonicalHeaderKey(k)
		if prev, dup := canonical[ck]; dup {
			errs = append(errs, fmt.Errorf("url_options.headers keys %q and %q are the same header", prev, k))
		}
		canonical[ck] = k
		if _, ok := clientHeaders[ck]; ok {
			errs = append(errs, fmt.Errorf("url_options.headers key %q is also in client.http_headers", k))
		}
	}
	return errs
}

// validateOutputs checks that each query has an output for its signal.
// It is separate from Validate because config text in unit tests cannot hold
// capsule values. New and Update call it, so it still fails at config load.
func validateOutputs(a Arguments) error {
	hasMetrics := len(a.ForwardTo.Metrics) > 0 || len(a.Output.Metrics) > 0
	hasLogs := len(a.ForwardTo.Logs) > 0 || len(a.Output.Logs) > 0
	var errs []error
	for _, q := range a.Queries {
		if q.Format == formatTable && !hasMetrics {
			errs = append(errs, fmt.Errorf(`query %q: format "table" needs forward_to.metrics or output.metrics`, q.Name))
		}
		if q.Format == formatLogs && !hasLogs {
			errs = append(errs, fmt.Errorf(`query %q: format "logs" needs forward_to.logs or output.logs`, q.Name))
		}
	}
	return errors.Join(errs...)
}
