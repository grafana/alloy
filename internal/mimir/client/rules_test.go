package client

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestMimirClient_X(t *testing.T) {
	requestCh := make(chan *http.Request, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCh <- r
		fmt.Fprintln(w, "hello")
	}))
	defer ts.Close()

	for _, tc := range []struct {
		test                 string
		namespace            string
		name                 string
		prometheusHTTPPrefix string
		expURLPath           string
	}{
		{
			test:                 "regular-characters",
			namespace:            "my-namespace",
			name:                 "my-name",
			expURLPath:           "/prometheus/config/v1/rules/my-namespace/my-name",
			prometheusHTTPPrefix: "/prometheus",
		},
		{
			test:                 "special-characters-spaces",
			namespace:            "My: Namespace",
			name:                 "My: Name",
			prometheusHTTPPrefix: "/prometheus",
			expURLPath:           "/prometheus/config/v1/rules/My:%20Namespace/My:%20Name",
		},
		{
			test:                 "special-characters-slashes",
			namespace:            "My/Namespace",
			name:                 "My/Name",
			prometheusHTTPPrefix: "/prometheus",
			expURLPath:           "/prometheus/config/v1/rules/My%2FNamespace/My%2FName",
		},
		{
			test:                 "special-characters-slash-first",
			namespace:            "My/Namespace",
			name:                 "/first-char-slash",
			prometheusHTTPPrefix: "/prometheus",
			expURLPath:           "/prometheus/config/v1/rules/My%2FNamespace/%2Ffirst-char-slash",
		},
		{
			test:                 "special-characters-slash-last",
			namespace:            "My/Namespace",
			name:                 "last-char-slash/",
			prometheusHTTPPrefix: "/prometheus",
			expURLPath:           "/prometheus/config/v1/rules/My%2FNamespace/last-char-slash%2F",
		},
		{
			test:                 "regular-characters-with-customized-prometheus-http-prefix",
			namespace:            "My/Namespace",
			name:                 "last-char-slash/",
			prometheusHTTPPrefix: "/mimir",
			expURLPath:           "/mimir/config/v1/rules/My%2FNamespace/last-char-slash%2F",
		},
	} {
		t.Run(tc.test, func(t *testing.T) {
			client, err := New(slog.New(slog.DiscardHandler), Config{
				Address:              ts.URL,
				PrometheusHTTPPrefix: tc.prometheusHTTPPrefix,
			}, prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))
			require.NoError(t, err)

			ctx := t.Context()
			require.NoError(t, client.DeleteRuleGroup(ctx, tc.namespace, tc.name))

			req := <-requestCh
			require.Equal(t, tc.expURLPath, req.URL.EscapedPath())
		})
	}
}

func TestMimirClient_CreateRuleGroup(t *testing.T) {
	bodyCh := make(chan []byte, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		bodyCh <- body
		fmt.Fprintln(w, "success")
	}))
	defer ts.Close()

	client, err := New(slog.New(slog.DiscardHandler), Config{
		Address: ts.URL,
	}, prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))
	require.NoError(t, err)

	for _, tc := range []struct {
		test string
		expr string
	}{
		{
			test: "single-line",
			expr: "up",
		},
		{
			test: "leading-indentation",
			expr: "  (\n    up\n  )",
		},
		{
			test: "leading-blank-line",
			expr: "\n# comment\n  (\n    up\n  )",
		},
	} {
		t.Run(tc.test, func(t *testing.T) {
			rg := MimirRuleGroup{
				RuleGroup: rulefmt.RuleGroup{
					Name: "group",
					Rules: []rulefmt.Rule{
						{
							Record: "record",
							Expr:   tc.expr,
						},
					},
				},
			}
			require.NoError(t, client.CreateRuleGroup(t.Context(), "namespace", rg))

			var got MimirRuleGroup
			require.NoError(t, yaml.Unmarshal(<-bodyCh, &got))
			require.Equal(t, rg, got)
		})
	}
}
