package basic_test

import (
	"context"
	"encoding/base64"
	"fmt"
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

const (
	actualUsername = "foo"
	actualPassword = "bar"
)

var (
	cfg = fmt.Sprintf(`
		username = "%s"
		password = "%s"
	`, actualUsername, actualPassword)
)

// Test performs a basic integration test which runs the otelcol.auth.basic
// component and ensures that it can be used for authentication.
func TestClientAuth(t *testing.T) {
	// Create an HTTP server which will assert that basic auth has been injected
	// into the request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok, "no basic auth found")
		assert.Equal(t, actualUsername, username, "basic auth username didn't match")
		assert.Equal(t, actualPassword, password, "basic auth password didn't match")

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := componenttest.TestContext(t)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ctrl := newTestComponent(t, ctx)

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Get the authentication extension from our component and use it to make a
	// request to our test server.
	exports := ctrl.Exports().(auth.Exports)
	require.NotNil(t, exports.Handler)

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
}

// TestServerAuth verifies the server auth component starts up properly and we can
// authenticate with the provided credentials.
func TestServerAuth(t *testing.T) {
	ctx := componenttest.TestContext(t)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ctrl := newTestComponent(t, ctx)
	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	exports, ok := ctrl.Exports().(auth.Exports)
	require.True(t, ok, "extension doesn't export auth exports struct")
	require.NotNil(t, exports.Handler)

	// This test fails due to a data race condition.
	// The test passes if the -race flag is not passed to the test, simulated by a sleep.
	// This issue arises from both the initialization of the basic_auth extension
	// and the scheduling mechanism used in the component test controller.
	// The problem is related to the way componenttest is structured and the interactions
	// between the Run method and the scheduler. The WaitRunning() method returns when the
	// channel is closed in the Run() method, but Run is actually invoked on the following line.
	// In auth.go, the component is scheduled via the scheduler, but the extension doesn't
	// actually start until the component is processed. As a result, ctrl.WaitRunning()
	// returns before the auth extension is fully running.
	// A more reliable method for confirming the extension is running would resolve this issue.
	// Otherwise, this test might need to be removed. If anyone has any feedback or thoughts
	// I would appreciate it.
	time.Sleep(time.Second)

	serverAuthExtension, err := exports.Handler.GetExtension(auth.Server)

	require.NoError(t, err)
	require.NotNil(t, serverAuthExtension.ID)
	require.NotNil(t, serverAuthExtension.Extension)

	otelServerExtension, ok := serverAuthExtension.Extension.(extauth.Server)
	require.True(t, ok, "extension did not implement server authentication")

	b64EncodingAuth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", actualUsername, actualPassword)))
	_, err = otelServerExtension.Authenticate(ctx, map[string][]string{"Authorization": {"Basic " + b64EncodingAuth}})
	require.NoError(t, err)
}

// newTestComponent brings up and runs the test component.
func newTestComponent(t *testing.T, ctx context.Context) *componenttest.Controller {
	t.Helper()
	l := util.TestLogger(t)

	// Create and run our component
	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.basic")
	require.NoError(t, err)

	var args basic.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	return ctrl
}
