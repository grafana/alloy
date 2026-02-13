package google_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/google"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/extensionauth"
	"golang.org/x/oauth2"
	"gotest.tools/assert"
)

func init() {
	// Make sure metadata.OnGCE always returns true, since the result is
	// cached.
	os.Setenv("GCE_METADATA_HOST", "127.0.0.1")
}

func TestRoundTripper(t *testing.T) {
	// Mimic metadata server, and return the fake access token.
	fakeToken := oauth2.Token{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    1,
	}
	b, err := json.Marshal(fakeToken)
	require.NoError(t, err)
	tokenString := string(b)
	srvProvidingTokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, tokenString)
	}))
	defer srvProvidingTokens.Close()
	t.Setenv("GCE_METADATA_HOST", srvProvidingTokens.Listener.Addr().String())

	// Create and run our component
	l := util.TestLogger(t)
	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.google")
	require.NoError(t, err)

	args := google.Arguments{
		Project:      "my-project",
		QuotaProject: "other-project",
		TokenType:    "access_token",
	}

	go func() {
		err := ctrl.Run(t.Context(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	startedComponent, err := ctrl.GetComponent()
	require.NoError(t, err, "no component added in controller.")
	require.NoError(t, waitHealthy(t.Context(), startedComponent.(*auth.Auth), time.Second))

	// Get the authentication extension from our component and use it to make a
	// request to our test server.
	exports, ok := ctrl.Exports().(auth.Exports)
	require.True(t, ok, "extension doesn't export auth exports struct")
	require.NotNil(t, exports.Handler)

	ext, err := exports.Handler.GetExtension(auth.Client)
	require.NoError(t, err)
	require.NotNil(t, ext.Extension, "handler extension is nil")

	clientAuth, ok := ext.Extension.(extauth.HTTPClient)
	require.True(t, ok, "handler does not implement configauth.ClientAuthenticator")

	// Validate that the request has the required headers.
	rt, err := clientAuth.RoundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, r.Header.Get("X-Goog-User-Project"), "other-project")
		assert.Equal(t, r.Header.Get("X-Goog-Project-ID"), "my-project")
		assert.Equal(t, r.Header.Get("foo"), "bar")
		if r.Header.Get("Authorization") != "tokenType accessToken" {
			// Don't print this out in-case it is a real access token.
			t.Error("Authorization header was incorrect. FindDefaultCredentials may have found real credentials.")
		}
		return &http.Response{}, nil
	}))
	require.NoError(t, err)
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err = rt.(*oauth2.Transport).RoundTrip(&http.Request{Header: header})
	require.NoError(t, err)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

// waitHealthy waits for the component to be healthy before continuing the test.
// this prevents the test from executing before the underlying auth extension is started.
func waitHealthy(ctx context.Context, authComponent *auth.Auth, timeout time.Duration) error {
	// Channel to signal whether the component is healthy or not.
	healthChannel := make(chan bool)

	// Loop continuously checking for the current health of the component.
	go func() {
		for {
			healthz := authComponent.CurrentHealth().Health
			if healthz == component.HealthTypeHealthy {
				healthChannel <- true
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(timeout):
				return
			default:
			}
		}
	}()

	// Wait for channel to be written to or timeout to occur.
	select {
	case <-healthChannel:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context timed out")
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for the component to be healthy")
	}
}
