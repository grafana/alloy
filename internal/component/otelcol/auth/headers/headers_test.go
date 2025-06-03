package headers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/headers"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/extensionauth"
)

// Test performs a basic integration test which runs the otelcol.auth.headers
// component and ensures that it can be used for authentication.
func Test(t *testing.T) {
	// Create an HTTP server which will assert that headers auth has been injected
	// into the request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get("X-Scope-Org-ID")
		assert.Equal(t, "fake", orgID, "X-Scope-Org-ID header didn't match")

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := componenttest.TestContext(t)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	l := util.TestLogger(t)

	// Create and run our component
	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.headers")
	require.NoError(t, err)

	cfg := `
		header {
			key   = "X-Scope-Org-ID"
			value = "fake"
		}
	`
	var args headers.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Get the authentication extension from our component and use it to make a
	// request to our test server.
	exports := ctrl.Exports().(auth.Exports)

	ext, err := exports.Handler.GetExtension(auth.Client)
	require.NoError(t, err)
	require.NotNil(t, ext.Extension, "handler extension is nil")

	clientAuth, ok := ext.Extension.(extauth.HTTPClient)
	require.True(t, ok, "handler does not implement configauth.ClientAuthenticator")

	rt, err := clientAuth.RoundTripper(http.DefaultTransport)
	require.NoError(t, err)
	cli := &http.Client{Transport: rt}

	// Wait until the request finishes. We don't assert anything else here; our
	// HTTP handler won't write the response until it ensures that the headers
	// were set.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := cli.Do(req)
	require.NoError(t, err, "HTTP request failed")
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		cfg                  string
		expectedKey          string
		expectedValue        string
		expectedAction       any
		expectUnmarshalError bool
	}{
		{
			cfg: `
			header {
				key    = "X-Scope-Org-ID"
				value  = "fake"
			}
			`,
			expectedKey:    "X-Scope-Org-ID",
			expectedValue:  "fake",
			expectedAction: headerssetterextension.UPSERT,
		},
		{
			cfg: `
			header {
				key    = "X-Scope-Org-ID"
				value  = "fake"
				action = "insert"
			}
			`,
			expectedKey:    "X-Scope-Org-ID",
			expectedValue:  "fake",
			expectedAction: headerssetterextension.INSERT,
		},
		{
			cfg: `
			header {
				key    = "X-Scope-Org-ID"
				value  = "fake"
				action = "update"
			}
			`,
			expectedKey:    "X-Scope-Org-ID",
			expectedValue:  "fake",
			expectedAction: headerssetterextension.UPDATE,
		},
		{
			cfg: `
			header {
				key    = "X-Scope-Org-ID"
				value  = "fake"
				action = "upsert"
			}
			`,
			expectedKey:    "X-Scope-Org-ID",
			expectedValue:  "fake",
			expectedAction: headerssetterextension.UPSERT,
		},
		{
			cfg: `
			header {
				key    = "X-Scope-Org-ID"
				value  = "fake"
				action = "delete"
			}
			`,
			expectedKey:    "X-Scope-Org-ID",
			expectedValue:  "fake",
			expectedAction: headerssetterextension.DELETE,
		},
		{
			cfg: `
			header {
				key    = "X-Scope-Org-ID"
				value  = "fake"
				action = "NonExistingAction"
			}
			`,
			expectUnmarshalError: true,
		},
	}

	for _, tc := range tests {
		var args headers.Arguments
		err := syntax.Unmarshal([]byte(tc.cfg), &args)

		if tc.expectUnmarshalError {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)

		ext, err := args.ConvertClient()

		require.NoError(t, err)
		otelArgs, ok := (ext).(*headerssetterextension.Config)
		require.True(t, ok)

		require.Equal(t, len(otelArgs.HeadersConfig), 1)
		require.Equal(t, *otelArgs.HeadersConfig[0].Key, tc.expectedKey)
		require.Equal(t, *otelArgs.HeadersConfig[0].Value, tc.expectedValue)
		require.Equal(t, otelArgs.HeadersConfig[0].Action, tc.expectedAction)
	}
}
