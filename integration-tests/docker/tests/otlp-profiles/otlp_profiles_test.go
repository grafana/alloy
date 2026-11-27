//go:build alloyintegrationtests

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/collector/pdata/testdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const (
	alloyProfilesGRPCEndpoint = "localhost:14317"
	alloyProfilesHTTPEndpoint = "http://localhost:14318/v1development/profiles"
)

func TestOTLPProfiles(t *testing.T) {
	var want []json.RawMessage
	for _, transport := range []string{"grpc", "http-protobuf", "http-json"} {
		t.Run(transport, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), common.TestTimeoutEnv(t))
			defer cancel()

			profiles := profilesFixture(transport)
			if transport == "grpc" {
				sendProfilesGRPC(t, ctx, profiles)
			} else {
				sendHTTP(t, ctx, transport, pprofileotlp.NewExportRequestFromProfiles(profiles))
			}
			expected, err := (&pprofile.JSONMarshaler{}).MarshalProfiles(profiles)
			require.NoError(t, err)
			want = append(want, expected)

			for _, destination := range []struct {
				name     string
				endpoint string
			}{
				{name: "via-processor", endpoint: "http://localhost:14319/profiles"},
				{name: "direct", endpoint: "http://localhost:14320/profiles"},
			} {
				t.Run(destination.name, func(t *testing.T) {
					require.EventuallyWithT(t, func(c *assert.CollectT) {
						got, err := receivedProfiles(ctx, destination.endpoint)
						require.NoError(c, err)
						require.Len(c, got, len(want))
						for i, expected := range want {
							require.JSONEq(c, string(expected), string(got[i]))
						}
					}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
				})
			}
		})
	}
}

// Check actual CLI startup: a timeout or an unrelated error must not count as
// successfully rejecting Profiles at the default and public-preview levels.
func TestProfilesRequireExperimental(t *testing.T) {
	for _, level := range []string{"default", "public-preview"} {
		t.Run(level, func(t *testing.T) {
			containerName := "test-otlp-profiles-stability-" + level
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				// Also remove the container if Alloy unexpectedly keeps running.
				_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()
			})
			args := []string{
				"compose", "-f", "docker-compose.yaml", "-p", "test-otlp-profiles",
				"run", "--rm", "--no-deps", "-T", "--name", containerName,
				"alloy", "run", "/etc/alloy/config.alloy", "--disable-reporting",
				"--storage.path=/tmp/profiles-stability", "--server.http.listen-addr=127.0.0.1:0",
			}
			if level != "default" {
				args = append(args, "--stability.level="+level)
			}
			ctx, cancel := context.WithTimeout(t.Context(), common.TestTimeoutEnv(t))
			defer cancel()
			cmd := exec.CommandContext(ctx, "docker", args...)
			cmd.WaitDelay = 5 * time.Second
			output, err := cmd.CombinedOutput()
			require.NoError(t, ctx.Err(), "Alloy must reject Profiles promptly; output: %s", output)
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "expected a configuration error; output: %s", output)
			require.Contains(t, string(output), "profiles output")
			require.Contains(t, string(output), "--stability.level")
		})
	}
}

func sendProfilesGRPC(t *testing.T, ctx context.Context, profiles pprofile.Profiles) {
	t.Helper()

	conn, err := grpc.NewClient(
		alloyProfilesGRPCEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	client := pprofileotlp.NewGRPCClient(conn)
	response, err := client.Export(ctx, pprofileotlp.NewExportRequestFromProfiles(profiles), grpc.WaitForReady(true))
	require.NoError(t, err)
	require.Zero(t, response.PartialSuccess().RejectedProfiles())
	require.Empty(t, response.PartialSuccess().ErrorMessage())
}

func sendHTTP(t *testing.T, ctx context.Context, transport string, request pprofileotlp.ExportRequest) {
	t.Helper()
	contentType := "application/x-protobuf"
	body, err := request.MarshalProto()
	if transport == "http-json" {
		contentType = "application/json"
		body, err = request.MarshalJSON()
	}
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, alloyProfilesHTTPEndpoint, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "Alloy must accept profiles over %s", transport)
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "response: %s", responseBody)
	response := pprofileotlp.NewExportResponse()
	if transport == "http-json" {
		err = response.UnmarshalJSON(responseBody)
	} else {
		err = response.UnmarshalProto(responseBody)
	}
	require.NoError(t, err)
	require.Zero(t, response.PartialSuccess().RejectedProfiles())
	require.Empty(t, response.PartialSuccess().ErrorMessage())
}

func profilesFixture(transport string) pprofile.Profiles {
	profiles := testdata.GenerateProfiles(2)
	resource := profiles.ResourceProfiles().At(0)
	resource.Resource().Attributes().PutStr("service.name", "checkout-e2e")
	resource.Resource().Attributes().PutStr("test.transport", transport)
	scope := resource.ScopeProfiles().At(0)
	scope.Scope().SetName("alloy.profiles.e2e")
	scope.Scope().SetVersion("1.0.0")
	dictionary := profiles.Dictionary()
	strings := dictionary.StringTable()
	cpuIndex := int32(strings.Len())
	strings.Append("cpu", "nanoseconds", "main.checkout", "checkout.go")
	functionIndex := int32(dictionary.FunctionTable().Len())
	function := dictionary.FunctionTable().AppendEmpty()
	function.SetNameStrindex(cpuIndex + 2)
	function.SetFilenameStrindex(cpuIndex + 3)
	function.SetStartLine(10)
	for i := 1; i < dictionary.LocationTable().Len(); i++ {
		line := dictionary.LocationTable().At(i).Lines().AppendEmpty()
		line.SetFunctionIndex(functionIndex)
		line.SetLine(int64(10 + i))
	}
	for i := 0; i < scope.Profiles().Len(); i++ {
		profile := scope.Profiles().At(i)
		profile.SampleType().SetTypeStrindex(cpuIndex)
		profile.SampleType().SetUnitStrindex(cpuIndex + 1)
		profile.PeriodType().SetTypeStrindex(cpuIndex)
		profile.PeriodType().SetUnitStrindex(cpuIndex + 1)
		profile.SetPeriod(int64(10 * time.Millisecond))
	}
	return profiles
}

func receivedProfiles(ctx context.Context, endpoint string) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected sink response status: %s", resp.Status)
	}

	var result []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
