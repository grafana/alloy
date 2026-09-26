package source

import (
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/grafana/alloy/syntax"
)

func parse(cfg string) (Arguments, error) {
	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	return args, err
}

const minimalQuery = `query "q" { url = "http://example.com/api" }`

func TestDefaults(t *testing.T) {
	args, err := parse(minimalQuery)
	require.NoError(t, err)

	require.Equal(t, 60*time.Second, args.Interval)
	require.Equal(t, 10*time.Second, args.Timeout)
	require.Equal(t, 10*units.MiB, args.MaxResponseSize)
	require.Len(t, args.Queries, 1)

	q := args.Queries[0]
	require.Equal(t, "q", q.Name)
	require.Equal(t, "json", q.Type)
	require.Equal(t, "backend", q.Parser)
	require.Equal(t, "table", q.Format)
	require.Equal(t, "url", q.Source)
	require.Equal(t, "GET", q.URLOptions.Method)
	require.Equal(t, "raw", q.URLOptions.BodyType)
}

func TestGraphQLDefaults(t *testing.T) {
	args, err := parse(`query "q" {
		type = "graphql"
		url  = "http://example.com/graphql"
		url_options { body_graphql_query = "{ viewer { login } }" }
	}`)
	require.NoError(t, err)
	require.Equal(t, "POST", args.Queries[0].URLOptions.Method)
	require.Equal(t, "graphql", args.Queries[0].URLOptions.BodyType)
}

func TestLogsDefaults(t *testing.T) {
	args, err := parse(`query "q" {
		url    = "http://example.com/api"
		format = "logs"
		logs {}
	}`)
	require.NoError(t, err)
	require.Equal(t, "body", args.Queries[0].Logs.LineColumn)
}

func TestTransformEnumKeepsOrder(t *testing.T) {
	args, err := parse(`query "q" {
		url = "http://example.com/api"
		transform.limit { limit = 5 }
		transform.filter { expression = "a > 1" }
	}`)
	require.NoError(t, err)
	tr := args.Queries[0].Transforms
	require.Len(t, tr, 2)
	require.NotNil(t, tr[0].Limit)
	require.Equal(t, 5, tr[0].Limit.Limit)
	require.NotNil(t, tr[1].Filter)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     string
		wantErr string
	}{
		{"valid minimal", minimalQuery, ""},
		{"no query", `interval = "10s"`, "query"},
		{"duplicate names", minimalQuery + "\n" + minimalQuery, `duplicate query name "q"`},
		{"zero interval", `interval = "0s"` + "\n" + minimalQuery, "interval must be greater than 0"},
		{"timeout above interval", `interval = "5s"` + "\n" + `timeout = "6s"` + "\n" + minimalQuery, "timeout must be less than or equal to interval"},
		{"zero max size", `max_response_size = "0B"` + "\n" + minimalQuery, "max_response_size must be greater than 0"},
		{"uql type", `query "q" {
	type = "uql"
	url = "http://x"
}`, `type "uql" is not supported in Alloy`},
		{"unknown type", `query "q" {
	type = "yaml"
	url = "http://x"
}`, `unknown type "yaml"`},
		{"simple parser", `query "q" {
	parser = "simple"
	url = "http://x"
}`, `parser "simple" is frontend-only`},
		{"timeseries format", `query "q" {
	format = "timeseries"
	url = "http://x"
}`, `use "table" for current-state metrics`},
		{"unknown format", `query "q" {
	format = "trace"
	url = "http://x"
}`, `unknown format "trace"`},
		{"azure source", `query "q" { source = "azure-blob" }`, `source "azure-blob" is not supported in Alloy`},
		{"missing url", `query "q" { }`, "url must be an absolute http or https URL"},
		{"relative url", `query "q" { url = "/api" }`, "url must be an absolute http or https URL"},
		{"ftp url", `query "q" { url = "ftp://example.com" }`, "url must be an absolute http or https URL"},
		{"data with url source", `query "q" {
	url = "http://x"
	data = "[]"
}`, `data must be empty when source is "url"`},
		{"inline without data", `query "q" { source = "inline" }`, `data is required when source is "inline"`},
		{"metrics with logs format", `query "q" {
	url = "http://x"
	format = "logs"
	metrics {}
}`, `metrics block requires format "table"`},
		{"logs with table format", `query "q" {
	url = "http://x"
	logs {}
}`, `logs block requires format "logs"`},
		{"negative series limit", `query "q" {
	url = "http://x"
	metrics { series_limit = -1 }
}`, "metrics.series_limit must not be negative"},
		{"negative entry limit", `query "q" {
	url = "http://x"
	format = "logs"
	logs { entry_limit = -1 }
}`, "logs.entry_limit must not be negative"},
		{"bad prefix", `query "q" {
	url = "http://x"
	metrics { prefix = "9bad" }
}`, `metrics.prefix "9bad" is not a valid metric name prefix`},
		{"label and metadata overlap", `query "q" {
	url = "http://x"
	format = "logs"
	logs {
		label_columns = ["a"]
		structured_metadata_columns = ["a"]
	}
}`, `column "a" is in both label_columns and structured_metadata_columns`},
		{"unknown column type", `query "q" {
	url = "http://x"
	column {
		selector = "a"
		type = "date"
	}
}`, `column "a": unknown type "date"`},
		{"timestamp format on string", `query "q" {
	url = "http://x"
	column {
		selector = "a"
		type = "string"
		timestamp_format = "2006"
	}
}`, `column "a": timestamp_format requires type "timestamp"`},
		{"unknown method", `query "q" {
	url = "http://x"
	url_options { method = "HEAD" }
}`, `unknown method "HEAD"`},
		{"unknown body type", `query "q" {
	url = "http://x"
	url_options {
		method = "POST"
		body_type = "xml"
	}
}`, `unknown body_type "xml"`},
		{"body with GET", `query "q" {
	url = "http://x"
	url_options { body = "x" }
}`, "a request body is not allowed with method GET"},
		{"graphql with GET", `query "q" {
	type = "graphql"
	url = "http://x"
	url_options { method = "GET" }
}`, `type "graphql" requires method "POST"`},
		{"graphql with raw body", `query "q" {
	type = "graphql"
	url = "http://x"
	url_options { body_type = "raw" }
}`, `type "graphql" requires body_type "graphql"`},
		{"bad graphql variables", `query "q" {
	type = "graphql"
	url = "http://x"
	url_options { body_graphql_variables = "{" }
}`, "body_graphql_variables must be valid JSON"},
		{"param also in url", `query "q" {
	url = "http://x/?page=1"
	url_options { params = { "page" = "2" } }
}`, `url_options.params key "page" is also in the url query`},
		{"header also in client", `client { http_headers = { "X-Key" = ["a"] } }
query "q" {
	url = "http://x"
	url_options { headers = { "x-key" = "b" } }
}`, `url_options.headers key "x-key" is also in client.http_headers`},
		{"zero transform limit", `query "q" {
	url = "http://x"
	transform.limit {
		limit = 0
	}
}`, "transform 0: limit must be greater than 0"},
		{"summarize by without expression", `query "q" {
	url = "http://x"
	summarize_by = "a"
}`, "summarize_by and summarize_alias require summarize_expression"},
		{"summarize alias without expression", `query "q" {
	url = "http://x"
	summarize_alias = "total"
}`, "summarize_by and summarize_alias require summarize_expression"},
		{"raw body with form body type", `query "q" {
	url = "http://x"
	url_options {
		method = "POST"
		body_type = "form-data"
		body = "x"
	}
}`, `body and body_content_type require body_type "raw"`},
		{"body content type with graphql body type", `query "q" {
	type = "graphql"
	url = "http://x"
	url_options {
		body_graphql_query = "{ a }"
		body_content_type = "text/plain"
	}
}`, `body and body_content_type require body_type "raw"`},
		{"form body with raw body type", `query "q" {
	url = "http://x"
	url_options {
		method = "POST"
		body_form = { "a" = "b" }
	}
}`, `body_form requires body_type "form-data" or "x-www-form-urlencoded"`},
		{"graphql query with raw body type", `query "q" {
	url = "http://x"
	url_options {
		method = "POST"
		body_graphql_query = "{ a }"
	}
}`, `body_graphql_query and body_graphql_variables require body_type "graphql"`},
		{"graphql variables with raw body type", `query "q" {
	url = "http://x"
	url_options {
		method = "POST"
		body_graphql_variables = "{}"
	}
}`, `body_graphql_query and body_graphql_variables require body_type "graphql"`},
		{"graphql without query", `query "q" {
	type = "graphql"
	url = "http://x"
}`, `type "graphql" requires body_graphql_query`},
		{"headers equal after canonicalization", `query "q" {
	url = "http://x"
	url_options {
		headers = { "x-key" = "a", "X-Key" = "b" }
	}
}`, `url_options.headers keys "X-Key" and "x-key" are the same header`},
		{"csv options with json type", `query "q" {
	url = "http://x"
	csv_options {
		delimiter = ";"
	}
}`, `csv_options requires type "csv" or "tsv"`},
		{"url with inline source", `query "q" {
	source = "inline"
	data = "[]"
	url = "http://x"
}`, `url must be empty when source is "inline"`},
		{"url options with inline source", `query "q" {
	source = "inline"
	data = "[]"
	url_options {
		method = "GET"
	}
}`, `url_options must be empty when source is "inline"`},
		{"valid inline graphql", `query "q" {
	type = "graphql"
	source = "inline"
	data = "{}"
}`, ""},
		{"valid form body", `query "q" {
	url = "http://x"
	url_options {
		method = "POST"
		body_type = "x-www-form-urlencoded"
		body_form = { "a" = "b" }
	}
}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.cfg)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateOutputs(t *testing.T) {
	table, err := parse(minimalQuery)
	require.NoError(t, err)
	logs, err := parse(`query "q" {
		url    = "http://example.com/api"
		format = "logs"
	}`)
	require.NoError(t, err)

	require.ErrorContains(t, validateOutputs(table), `query "q": format "table" needs forward_to.metrics or output.metrics`)
	require.ErrorContains(t, validateOutputs(logs), `query "q": format "logs" needs forward_to.logs or output.logs`)

	table.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{}}
	require.NoError(t, validateOutputs(table))

	logs.ForwardTo.Logs = []loki.LogsReceiver{loki.NewLogsReceiver()}
	require.NoError(t, validateOutputs(logs))
}
