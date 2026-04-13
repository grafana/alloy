//go:build alloyintegrationtests

package main

import (
	"context"
	"net/http"
	"testing"
	"time"

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

	// The injected OpenTelemetry Java SDK may need a few seconds to finish
	// initialising its OTLP exporter after Beyla injects it into the JVM.
	// Keep generating traffic throughout the assertion window so that requests
	// are captured once the SDK is ready, rather than relying solely on the
	// initial burst above.
	ctx, cancel := context.WithTimeout(context.Background(), common.DefaultTimeout)
	defer cancel()
	go func() {
		client := &http.Client{Timeout: 5 * common.DefaultRetryInterval}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := client.Get(javaAppURL)
				if err == nil {
					resp.Body.Close()
				}
			}
		}
	}()

	tags := map[string]string{
		"service.name":           javaServiceName,
		"telemetry.sdk.name":     "opentelemetry",
		"telemetry.sdk.language": "java",
	}
	common.TracesTest(t, tags, "beyla-java")
}
