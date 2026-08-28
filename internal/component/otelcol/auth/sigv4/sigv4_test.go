package sigv4_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/sigv4"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extauth "go.opentelemetry.io/collector/extension/extensionauth"
)

// Test performs a basic integration test which runs the otelcol.auth.sigv4
// component and ensures that it can be used for authentication.
func Test(t *testing.T) {
	type TestDefinition struct {
		testName              string
		awsAccessKeyId        string
		awsSecredAccessKey    string
		region                string
		service               string
		assumeRoleARN         string
		assumeRoleSessionName string
		assumeRoleStsRegion   string
		alloyConfig           string
	}

	tests := []TestDefinition{
		{
			testName:              "Test1",
			awsAccessKeyId:        "example_access_key_id",
			awsSecredAccessKey:    "example_secret_access_key",
			region:                "example_region",
			service:               "example_service",
			assumeRoleARN:         "",
			assumeRoleSessionName: "role_session_name",
			assumeRoleStsRegion:   "",
			alloyConfig:           "",
		},
		{
			testName:              "Test2",
			awsAccessKeyId:        "example_access_key_id",
			awsSecredAccessKey:    "example_secret_access_key",
			region:                "example_region",
			service:               "example_service",
			assumeRoleARN:         "",
			assumeRoleSessionName: "",
			assumeRoleStsRegion:   "region",
			alloyConfig:           "",
		},
		{
			testName:              "Test3",
			awsAccessKeyId:        "example_access_key_id",
			awsSecredAccessKey:    "example_secret_access_key",
			region:                "example_region",
			service:               "",
			assumeRoleARN:         "",
			assumeRoleSessionName: "",
			assumeRoleStsRegion:   "",
			alloyConfig:           "",
		},
		{
			testName:              "Test4",
			awsAccessKeyId:        "example_access_key_id",
			awsSecredAccessKey:    "example_secret_access_key",
			region:                "",
			service:               "example_service",
			assumeRoleARN:         "",
			assumeRoleSessionName: "",
			assumeRoleStsRegion:   "",
			alloyConfig:           "",
		},
	}

	{
		var tt = &tests[0]
		tt.alloyConfig = fmt.Sprintf(`
			assume_role {
				session_name = "%s"
			}
			region = "%s"
			service = "%s"
		`, tt.assumeRoleSessionName, tt.region, tt.service)
	}
	{
		var tt = &tests[1]
		tt.alloyConfig = fmt.Sprintf(`
			assume_role {
				sts_region = "%s"
			}
			region = "%s"
			service = "%s"
		`, tt.assumeRoleStsRegion, tt.region, tt.service)
	}
	{
		var tt = &tests[2]
		tt.alloyConfig = fmt.Sprintf(`
			region = "%s"
		`, tt.region)
	}
	{
		var tt = &tests[3]
		tt.alloyConfig = fmt.Sprintf(`
		service = "%s"
		`, tt.service)
	}

	for _, tt := range tests {
		// Create an HTTP server which will assert that sigv4 auth has been injected
		// into the request.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The date looks like "20230306T125949Z;"
			dateTimeSplit := strings.Split(r.Header.Get("X-Amz-Date"), "T")
			assert.Equal(t, len(dateTimeSplit), 2)
			date := dateTimeSplit[0]
			assert.Equal(t, len(date), 8)

			authHeaderSplit := strings.Split(r.Header.Get("Authorization"), " ")

			assert.Equal(t, authHeaderSplit[0], "AWS4-HMAC-SHA256")

			credential := fmt.Sprintf("Credential=%s/%s/%s/%s/aws4_request,", tt.awsAccessKeyId, date, tt.region, tt.service)
			assert.Equal(t, authHeaderSplit[1], credential)

			assert.Equal(t, authHeaderSplit[2], "SignedHeaders=host;x-amz-date,")

			signatureSplit := strings.Split(authHeaderSplit[3], "=")
			assert.Equal(t, 2, len(signatureSplit))
			assert.Equal(t, "Signature", signatureSplit[0])

			// SHA256 will always produce a 64 character string
			require.Equal(t, 64, len(signatureSplit[1]))

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		t.Setenv("AWS_ACCESS_KEY_ID", tt.awsAccessKeyId)
		t.Setenv("AWS_SECRET_ACCESS_KEY", tt.awsSecredAccessKey)

		ctx := componenttest.TestContext(t)
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		l := util.TestLogger(t)

		// Create and run our component
		ctrl, err := componenttest.NewControllerFromID(l, "otelcol.auth.sigv4")
		require.NoError(t, err)

		cfg := tt.alloyConfig
		t.Logf("Alloy configuration: %s", cfg)
		var args sigv4.Arguments
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
		// HTTP handler won't write the response until it ensures that the sigv4
		// data is set.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := cli.Do(req)
		require.NoError(t, err, "HTTP request failed")
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
}
