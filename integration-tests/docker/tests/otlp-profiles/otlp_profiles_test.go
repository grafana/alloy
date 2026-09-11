//go:build alloyintegrationtests

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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
	sinkCountEndpoint         = "http://localhost:14319/count"
)

func TestOTLPProfiles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), common.TestTimeoutEnv(t))
	defer cancel()

	sendProfilesGRPC(t, ctx, testdata.GenerateProfiles(1))
	sendProfilesHTTP(t, ctx, testdata.GenerateProfiles(1))

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		count, err := receivedProfileCount(ctx)
		require.NoError(c, err)
		require.Equal(c, 2, count)
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
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
	_, err = client.Export(ctx, pprofileotlp.NewExportRequestFromProfiles(profiles))
	require.NoError(t, err)
}

func sendProfilesHTTP(t *testing.T, ctx context.Context, profiles pprofile.Profiles) {
	t.Helper()

	body, err := pprofileotlp.NewExportRequestFromProfiles(profiles).MarshalProto()
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, alloyProfilesHTTPEndpoint, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func receivedProfileCount(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sinkCountEndpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected sink response status: %s", resp.Status)
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Count, nil
}
