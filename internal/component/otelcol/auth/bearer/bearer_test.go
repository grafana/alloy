package bearer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/otelcol/auth"
	"github.com/grafana/agent/internal/component/otelcol/auth/bearer"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/auth"
)

// Test performs a basic integration test which runs the otelcol.auth.bearer
// component and ensures that it can be used for authentication.
func Test(t *testing.T) {
	type TestDefinition struct {
		testName          string
		expectedHeaderVal string
		riverConfig       string
	}

	tests := []TestDefinition{
		{
			testName:          "Test1",
			expectedHeaderVal: "Bearer example_access_key_id",
			riverConfig: `
			token = "example_access_key_id"
			`,
		},
		{
			testName:          "Test2",
			expectedHeaderVal: "Bearer example_access_key_id",
			riverConfig: `
			token = "example_access_key_id"
			scheme = "Bearer"
			`,
		},
		{
			testName:          "Test3",
			expectedHeaderVal: "MyScheme example_access_key_id",
			riverConfig: `
			token = "example_access_key_id"
			scheme = "MyScheme"
			`,
		},
		{
			testName:          "Test4",
			expectedHeaderVal: "example_access_key_id",
			riverConfig: `
			token = "example_access_key_id"
			scheme = ""
			`,
		},
	}

	for _, tt := range tests {
		// Create an HTTP server which will assert that bearer auth has been injected
		// into the request.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeaderVal, authHeader, "auth header didn't match")

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		ctx := componenttest.TestContext(t)
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		l := util.TestLogger(t)

		// Create and run our component
		ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.bearer")
		require.NoError(t, err)

		var args bearer.Arguments
		require.NoError(t, river.Unmarshal([]byte(tt.riverConfig), &args))

		go func() {
			err := ctrl.Run(ctx, args)
			require.NoError(t, err)
		}()

		require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
		require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

		// Get the authentication extension from our component and use it to make a
		// request to our test server.
		exports := ctrl.Exports().(auth.Exports)
		require.NotNil(t, exports.Handler.Extension, "handler extension is nil")

		clientAuth, ok := exports.Handler.Extension.(extauth.Client)
		require.True(t, ok, "handler does not implement configauth.ClientAuthenticator")

		rt, err := clientAuth.RoundTripper(http.DefaultTransport)
		require.NoError(t, err)
		cli := &http.Client{Transport: rt}

		// Wait until the request finishes. We don't assert anything else here; our
		// HTTP handler won't write the response until it ensures that the bearer
		// auth was found.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := cli.Do(req)
		require.NoError(t, err, "HTTP request failed")
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
}
