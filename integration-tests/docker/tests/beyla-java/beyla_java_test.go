//go:build alloyintegrationtests

package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const javaAppURL = "http://localhost:18080/"
const javaServiceName = "petclinic"

func generateJavaTraffic(t *testing.T) {
	t.Helper()

	client := &http.Client{Timeout: 5 * common.DefaultRetryInterval}

	require.Eventually(t, func() bool {
		resp, err := client.Get(javaAppURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 400
	}, common.DefaultTimeout, common.DefaultRetryInterval)

	for range 20 {
		resp, err := client.Get(javaAppURL)
		require.NoError(t, err)
		resp.Body.Close()
	}
}

func TestBeylaJavaSDKTraces(t *testing.T) {
	generateJavaTraffic(t)

	tags := map[string]string{
		"service.name":           javaServiceName,
		"telemetry.sdk.name":     "opentelemetry",
		"telemetry.sdk.language": "java",
	}
	common.TracesTest(t, tags, "beyla-java")
}
