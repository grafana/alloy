package basic_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/basic"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/auth"
)

// Test performs a basic integration test which runs the otelcol.auth.basic
// component and ensures that it can be used for authentication.
func Test(t *testing.T) {
	// Create an HTTP server which will assert that basic auth has been injected
	// into the request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok, "no basic auth found")
		assert.Equal(t, "foo", username, "basic auth username didn't match")
		assert.Equal(t, "bar", password, "basic auth password didn't match")

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := componenttest.TestContext(t)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	l := util.TestLogger(t)

	// Create and run our component
	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.basic")
	require.NoError(t, err)

	cfg := `
		username = "foo"
		password = "bar"
	`
	var args basic.Arguments
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
	require.NotNil(t, exports.Handler)

	t.Run("ClientAuth", func(t *testing.T) {
		clientExtension, err := exports.Handler.GetExtension(auth.Client)
		require.NoError(t, err)
		require.NotNil(t, clientExtension)
		clientAuth, ok := clientExtension.Extension.(extauth.Client)
		require.True(t, ok, "handler does not implement configauth.ClientAuthenticator")

		rt, err := clientAuth.RoundTripper(http.DefaultTransport)
		require.NoError(t, err)
		cli := &http.Client{Transport: rt}

		// Wait until the request finishes. We don't assert anything else here; our
		// HTTP handler won't write the response until it ensures that the basic auth
		// was found.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := cli.Do(req)
		require.NoError(t, err, "HTTP request failed")
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ServerAuth", func(t *testing.T) {
		serverExtension, err := exports.Handler.GetExtension(auth.Server)
		require.NoError(t, err)
		require.NotNil(t, serverExtension)
		serverAuth, ok := serverExtension.Extension.(extauth.Server)
		require.True(t, ok, "handler does not implement configauth.ServerAuthenticator")
		b64EncodingAuth := base64.StdEncoding.EncodeToString([]byte("foo:bar"))
		_, err = serverAuth.Authenticate(ctx, map[string][]string{"Authorization": {"Basic " + b64EncodingAuth}})
		require.NoError(t, err)
	})

}
