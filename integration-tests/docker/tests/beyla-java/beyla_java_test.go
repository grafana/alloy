//go:build alloyintegrationtests

package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const javaAppURL = "http://localhost:18080/"
const javaServiceName = "petclinic"

func waitForJavaApp(t *testing.T, client *http.Client) {
	t.Helper()

	require.Eventually(t, func() bool {
		resp, err := client.Get(javaAppURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 400
	}, common.DefaultTimeout, common.DefaultRetryInterval)
}

func triggerJavaRequest(client *http.Client) func(c *assert.CollectT) {
	return func(c *assert.CollectT) {
		resp, err := client.Get(javaAppURL)
		if !assert.NoError(c, err) {
			return
		}
		if resp != nil {
			defer resp.Body.Close()
			assert.GreaterOrEqual(c, resp.StatusCode, http.StatusOK)
			assert.Less(c, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

func TestBeylaJavaSDKTraces(t *testing.T) {
	client := &http.Client{Timeout: 5 * common.DefaultRetryInterval}
	waitForJavaApp(t, client)

	tags := map[string]string{
		"service.name":           javaServiceName,
		"telemetry.sdk.name":     "opentelemetry",
		"telemetry.sdk.language": "java",
	}
	common.TracesTestWithProbe(t, tags, "beyla-java", triggerJavaRequest(client))
}
