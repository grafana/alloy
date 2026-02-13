package google_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/google"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/extensionauth"
	"golang.org/x/oauth2"
	"gotest.tools/assert"
)

func TestRoundTripper(t *testing.T) {
	fakeToken := oauth2.Token{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    1,
	}
	b, err := json.Marshal(fakeToken)
	require.NoError(t, err)
	tokenString := string(b)
	// Mimic metadata server, and return the fake access token.
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

	// Get the authentication extension from our component and use it to make a
	// request to our test server.
	exports := ctrl.Exports().(auth.Exports)

	ext, err := exports.Handler.GetExtension(auth.Client)
	require.NoError(t, err)
	require.NotNil(t, ext.Extension, "handler extension is nil")

	clientAuth, ok := ext.Extension.(extauth.HTTPClient)
	require.True(t, ok, "handler does not implement configauth.ClientAuthenticator")

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
	oauthRt := rt.(*oauth2.Transport)
	_, err = oauthRt.Source.Token()
	require.NoError(t, err)
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err = oauthRt.RoundTrip(&http.Request{Header: header})
	require.NoError(t, err)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
