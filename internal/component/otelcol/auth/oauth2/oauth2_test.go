package oauth2_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/oauth2"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/extensionauth"
	"gotest.tools/assert"
)

// Test performs a oauth2 integration test which runs the otelcol.auth.oauth2
// component and ensures that it can be used for authentication.
func Test(t *testing.T) {
	tests := []struct {
		testName      string
		accessToken   string
		tokenType     string
		refreshToken  string
		configBuilder func(string) string
	}{
		{
			"runWithRequiredParams",
			"TestAccessToken",
			"TestTokenType",
			"TestRefreshToken",
			func(srvProvidingTokensURL string) string {
				return fmt.Sprintf(`
					client_id     = "someclientid"
					client_secret = "someclientsecret"
					token_url     = "%s/oauth2/default/v1/token"
				`, srvProvidingTokensURL)
			},
		},
		{
			"runWithOptionalParams",
			"TestAccessToken",
			"TestTokenType",
			"TestRefreshToken",
			func(srvProvidingTokensURL string) string {
				return fmt.Sprintf(`
					client_id       = "someclientid2"
					client_secret   = "someclientsecret2"
					token_url       = "%s/oauth2/default/v1/token"
					endpoint_params = {"audience" = ["someaudience"]}
					scopes          = ["api.metrics"]
					timeout         = "1s"
					tls {
						insecure = true
					}
				`, srvProvidingTokensURL)
			},
		},
	}

	//TODO: Could we call t.Parallel() here? I am not sure if httptest.NewServer() usage is thread safe.
	//      For now we do the tests synchronously because there aren't that many of them anyway.
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			// Create an HTTP server which will assert that oauth2 auth has been injected
			// into the request.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				authHeader := r.Header.Get("Authorization")
				expectedAuthHeader := fmt.Sprintf("%s %s", tt.tokenType, tt.accessToken)
				assert.Equal(t, expectedAuthHeader, authHeader, "auth header didn't match")

				//TODO: Also write checks for `endpoint_params`` and `scopes``
			}))
			defer srv.Close()
			t.Logf("Created server which will require authentication on address %s", srv.URL)

			srvProvidingTokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "access_token=%s&token_type=%s&refresh_token=%s", tt.accessToken, tt.tokenType, tt.refreshToken)
			}))
			defer srvProvidingTokens.Close()
			t.Logf("Created server which will provide authentication tokens on address %s", srvProvidingTokens.URL)

			ctx := componenttest.TestContext(t)
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()

			l := util.TestLogger(t)

			// Create and run our component
			ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.oauth2")
			require.NoError(t, err)

			cfg := tt.configBuilder(srvProvidingTokens.URL)
			var args oauth2.Arguments
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
			// HTTP handler won't write the response until it ensures that the oauth2 auth
			// was found.
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			require.NoError(t, err)
			resp, err := cli.Do(req)
			require.NoError(t, err, "HTTP request failed")
			require.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}
