package bearer_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/bearer"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/extensionauth"
)

// Test performs a basic integration test which runs the otelcol.auth.bearer
// component and ensures that it can be used for authentication.
func TestClient(t *testing.T) {
	type TestDefinition struct {
		testName          string
		expectedHeaderVal string
		alloyConfig       string
	}

	tests := []TestDefinition{
		{
			testName:          "Test1",
			expectedHeaderVal: "Bearer example_access_key_id",
			alloyConfig: `
			token = "example_access_key_id"
			`,
		},
		{
			testName:          "Test2",
			expectedHeaderVal: "Bearer example_access_key_id",
			alloyConfig: `
			token = "example_access_key_id"
			scheme = "Bearer"
			`,
		},
		{
			testName:          "Test3",
			expectedHeaderVal: "MyScheme example_access_key_id",
			alloyConfig: `
			token = "example_access_key_id"
			scheme = "MyScheme"
			`,
		},
		{
			testName:          "Test4",
			expectedHeaderVal: "example_access_key_id",
			alloyConfig: `
			token = "example_access_key_id"
			scheme = ""
			`,
		},
		{
			testName:          "Test5",
			expectedHeaderVal: "example_access_key_id",
			alloyConfig: `
			token = "example_access_key_id"
			scheme = ""
			header = "testHeader"
			`,
		},
	}

	for _, tt := range tests {
		ctx := componenttest.TestContext(t)
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		ctrl, header := newTestComponent(t, ctx, tt.alloyConfig)

		// Create an HTTP server which will assert that bearer auth has been injected
		// into the request.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get(header)
			assert.Equal(t, tt.expectedHeaderVal, authHeader, "auth header didn't match")

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		// Get the authentication extension from our component and use it to make a
		// request to our test server.
		exports := ctrl.Exports().(auth.Exports)
		require.NotNil(t, exports.Handler, "handler extension is nil")

		clientExtension, err := exports.Handler.GetExtension(auth.Client)
		require.NoError(t, err)

		clientAuth, ok := clientExtension.Extension.(extauth.HTTPClient)
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

func TestServer(t *testing.T) {
	type TestDefinition struct {
		testName    string
		token       string
		scheme      string
		alloyConfig string
	}
	token := "123"
	tokenCfg := fmt.Sprintf(`token = "%s"`, token)
	tests := []TestDefinition{
		{
			testName: "Test1",
			token:    token,
			scheme:   "Bearer",
			alloyConfig: fmt.Sprintf(`
			%s
			`, tokenCfg),
		},
		{
			testName: "Test2",
			token:    token,
			scheme:   "Bearer",
			alloyConfig: fmt.Sprintf(`
			%s
			scheme = "Bearer"
			`, tokenCfg),
		},
		{
			testName: "Test3",
			token:    token,
			scheme:   "MyScheme",
			alloyConfig: fmt.Sprintf(`
			%s
			scheme = "MyScheme"
			`, tokenCfg),
		},
		{
			testName: "Test4",
			token:    token,
			scheme:   "",
			alloyConfig: fmt.Sprintf(`
			%s
			scheme = ""
			`, tokenCfg),
		},
		{
			testName: "Test5",
			token:    token,
			scheme:   "",
			alloyConfig: fmt.Sprintf(`
			%s
			scheme = ""
			header = "testHeader"
			`, tokenCfg),
		},
	}

	for _, td := range tests {
		ctx := componenttest.TestContext(t)
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		// Spin up component
		ctrl, header := newTestComponent(t, ctx, td.alloyConfig)
		exports := ctrl.Exports()
		require.NotNil(t, exports)

		authExport, ok := exports.(auth.Exports)
		require.True(t, ok, "component doesn't export auth export struct")

		// Get handler from exports
		handler := authExport.Handler
		require.NotNil(t, handler)

		// Get the server auth extension
		serverExtension, err := handler.GetExtension(auth.Server)
		require.NoError(t, err)
		require.NotNil(t, serverExtension.Extension)
		require.NotNil(t, serverExtension.ID)

		// Convert to server auth extension
		otelServerAuthExtension, ok := serverExtension.Extension.(extauth.Server)
		require.True(t, ok, "extension does not implement server authentication")

		scheme := fmt.Sprintf("%s %s", td.scheme, td.token)

		// Trim the space in case bearer token is set to an empty string
		scheme = strings.TrimSpace(scheme)
		_, err = otelServerAuthExtension.Authenticate(ctx, map[string][]string{header: {scheme}})
		require.NoError(t, err, td.testName)
	}
}

func newTestComponent(t *testing.T, ctx context.Context, alloyConfig string) (*componenttest.Controller, string) {
	t.Helper()
	l := util.TestLogger(t)

	// Create and run our component
	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.bearer")
	require.NoError(t, err)

	var args bearer.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyConfig), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	return ctrl, args.Header
}
