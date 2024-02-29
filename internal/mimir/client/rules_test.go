package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
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
			client, err := New(log.NewNopLogger(), Config{
				Address:              ts.URL,
				PrometheusHTTPPrefix: tc.prometheusHTTPPrefix,
			}, prometheus.NewHistogramVec(prometheus.HistogramOpts{}, instrument.HistogramCollectorBuckets))
			require.NoError(t, err)

			ctx := context.Background()
			require.NoError(t, client.DeleteRuleGroup(ctx, tc.namespace, tc.name))

			req := <-requestCh
			require.Equal(t, tc.expURLPath, req.URL.EscapedPath())
		})
	}
}
