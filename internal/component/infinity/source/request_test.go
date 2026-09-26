package source

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func specFromConfig(cfg string) (querySpec, error) {
	args, err := parse(cfg)
	if err != nil {
		return querySpec{}, err
	}
	return newQuerySpec(args.Queries[0]), nil
}

func TestBuildRequestGETWithParamsAndHeaders(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://example.com/api?a=1"
		url_options {
			params  = { "key" = "secret-value" }
			headers = { "X-Token" = "t0k" }
		}
	}`)
	require.NoError(t, err)

	req, err := buildRequest(t.Context(), s)
	require.NoError(t, err)
	require.Equal(t, "GET", req.Method)
	require.Equal(t, "1", req.URL.Query().Get("a"))
	require.Equal(t, "secret-value", req.URL.Query().Get("key"))
	require.Equal(t, "t0k", req.Header.Get("X-Token"))
	require.Nil(t, req.Body)
}

func TestBuildRequestRawBody(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://example.com/api"
		url_options {
			method            = "post"
			body              = "{\"a\":1}"
			body_content_type = "application/json"
		}
	}`)
	require.NoError(t, err)

	req, err := buildRequest(t.Context(), s)
	require.NoError(t, err)
	require.Equal(t, "POST", req.Method)
	require.Equal(t, "application/json", req.Header.Get("Content-Type"))
	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`, string(b))
}

func TestBuildRequestURLEncodedForm(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://example.com/api"
		url_options {
			method    = "POST"
			body_type = "x-www-form-urlencoded"
			body_form = { "a" = "1", "b" = "two words" }
		}
	}`)
	require.NoError(t, err)

	req, err := buildRequest(t.Context(), s)
	require.NoError(t, err)
	require.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	vals, err := url.ParseQuery(string(b))
	require.NoError(t, err)
	require.Equal(t, "two words", vals.Get("b"))
}

func TestBuildRequestMultipartForm(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://example.com/api"
		url_options {
			method    = "POST"
			body_type = "form-data"
			body_form = { "a" = "1" }
		}
	}`)
	require.NoError(t, err)

	req, err := buildRequest(t.Context(), s)
	require.NoError(t, err)
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, form.Value["a"])
}

func TestBuildRequestGraphQL(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		type = "graphql"
		url  = "http://example.com/graphql"
		url_options {
			body_graphql_query     = "query($n: Int) { items(first: $n) { id } }"
			body_graphql_variables = "{\"n\": 5}"
		}
	}`)
	require.NoError(t, err)

	req, err := buildRequest(t.Context(), s)
	require.NoError(t, err)
	require.Equal(t, "POST", req.Method)
	require.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var payload struct {
		Query     string         `json:"query"`
		Variables map[string]int `json:"variables"`
	}
	require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
	require.Contains(t, payload.Query, "items(first: $n)")
	require.Equal(t, 5, payload.Variables["n"])
}

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"http://example.com/api":                    "http://example.com/api",
		"http://example.com/api?key=abc&page=2":     "http://example.com/api?key=REDACTED&page=REDACTED",
		"https://user:pass@example.com/api?token=x": "https://example.com/api?token=REDACTED",
		"::not a url": "<invalid url>",
	}
	for in, want := range tests {
		require.Equal(t, want, redactURL(in), in)
	}
}

func TestBuildRequestDefaultAccept(t *testing.T) {
	tests := []struct {
		qtype, want string
	}{
		{"json", "application/json;q=0.9,text/plain"},
		{"graphql", "application/json;q=0.9,text/plain"},
		{"csv", "text/csv"},
		{"tsv", "text/csv"},
		{"xml", "text/xml;q=0.9,text/plain"},
		{"html", "text/xml;q=0.9,text/plain"},
	}
	for _, tt := range tests {
		t.Run(tt.qtype, func(t *testing.T) {
			opts := ""
			if tt.qtype == typeGraphQL {
				opts = `url_options {
					body_graphql_query = "{ a }"
				}`
			}
			s, err := specFromConfig(fmt.Sprintf(`query "q" {
				type = %q
				url  = "http://example.com/api"
				%s
			}`, tt.qtype, opts))
			require.NoError(t, err)
			req, err := buildRequest(t.Context(), s)
			require.NoError(t, err)
			require.Equal(t, []string{tt.want}, req.Header.Values("Accept"))
		})
	}
}

func TestBuildRequestUserAccept(t *testing.T) {
	s, err := specFromConfig(`query "q" {
		url = "http://example.com/api"
		url_options {
			headers = { "accept" = "application/vnd.api+json" }
		}
	}`)
	require.NoError(t, err)
	req, err := buildRequest(t.Context(), s)
	require.NoError(t, err)
	require.Equal(t, []string{"application/vnd.api+json"}, req.Header.Values("Accept"))
}
