package client

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
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

func TestMimirClient_CreateRuleGroupKeepsExpressionWhitespace(t *testing.T) {
	bodyCh := make(chan []byte, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		bodyCh <- body
	}))
	defer ts.Close()

	client, err := New(slog.New(slog.DiscardHandler), Config{
		Address:              ts.URL,
		PrometheusHTTPPrefix: "/prometheus",
	}, prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))
	require.NoError(t, err)

	for _, tc := range []struct {
		test string
		expr string
	}{
		{
			// A folded block that starts with a blank line keeps that leading newline.
			test: "leading-newline",
			expr: "\n# The blank line and nested indentation are intentional.\n  (\n    up\n  )",
		},
		{
			test: "leading-spaces",
			expr: "  sum(up)\n  by (job)",
		},
		{
			test: "two-leading-newlines",
			expr: "\n\n  up",
		},
	} {
		t.Run(tc.test, func(t *testing.T) {
			group := MimirRuleGroup{RuleGroup: rulefmt.RuleGroup{
				Name:     "group",
				Interval: model.Duration(time.Minute),
				Rules: []rulefmt.Rule{{
					Alert:       "alert",
					Expr:        tc.expr,
					Annotations: map[string]string{"description": tc.expr},
				}},
			}}
			require.NoError(t, client.CreateRuleGroup(t.Context(), "namespace", group))

			// Mimir decodes the payload into a rulefmt.RuleGroup.
			var got rulefmt.RuleGroup
			require.NoError(t, yaml.Unmarshal(<-bodyCh, &got))
			require.Equal(t, group.RuleGroup, got)
		})
	}
}
