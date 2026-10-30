//go:build alloyintegrationtests

package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/grafana/alloy/internal/component/pyroscope"
)

func TestEBPFProfilesReachOTLP(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost:14321/profiles", nil)
		require.NoError(c, err)
		resp, err := client.Do(req)
		require.NoError(c, err)
		defer resp.Body.Close()
		require.Equal(c, http.StatusOK, resp.StatusCode)
		var exports []json.RawMessage
		require.NoError(c, json.NewDecoder(resp.Body).Decode(&exports))
		var foundWorkload, foundCPU, foundFunction bool
		for _, raw := range exports {
			profiles, err := (&pprofile.JSONUnmarshaler{}).UnmarshalProfiles(raw)
			require.NoError(c, err)
			dictionary := profiles.Dictionary()
			for _, rp := range profiles.ResourceProfiles().All() {
				service, ok := rp.Resource().Attributes().Get("service.name")
				if !ok || service.Str() != "profile-workload" {
					continue
				}
				foundWorkload = true
				for _, sp := range rp.ScopeProfiles().All() {
					require.Equal(c, pyroscope.ScopeNameEBPF, sp.Scope().Name())
					for _, prof := range sp.Profiles().All() {
						if dictionary.StringTable().At(int(prof.SampleType().TypeStrindex())) != "cpu" {
							continue
						}
						require.Equal(c, "nanoseconds", dictionary.StringTable().At(int(prof.SampleType().UnitStrindex())))
						for _, sample := range prof.Samples().All() {
							var hasPositiveValue bool
							for _, value := range sample.Values().All() {
								hasPositiveValue = hasPositiveValue || value > 0
							}
							foundCPU = foundCPU || hasPositiveValue
							if !hasPositiveValue {
								continue
							}
							stack := dictionary.StackTable().At(int(sample.StackIndex()))
							for _, idx := range stack.LocationIndices().All() {
								location := dictionary.LocationTable().At(int(idx))
								for _, line := range location.Lines().All() {
									function := dictionary.FunctionTable().At(int(line.FunctionIndex()))
									foundFunction = foundFunction || dictionary.StringTable().At(int(function.NameStrindex())) == "main.burnCPU"
								}
							}
						}
					}
				}
			}
		}
		require.True(c, foundWorkload, "discovery and relabeling must identify the workload service")
		require.True(c, foundCPU, "the workload must produce positive CPU samples at the OTLP sink")
		require.True(c, foundFunction, "a positive CPU sample must contain main.burnCPU in its stack after eBPF collection, conversion, and export")
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}
