package rules

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/util/workqueue"
)

func TestEventTypeIsHashable(t *testing.T) {
	// This test is here to ensure that the EventType type is hashable according to the workqueue implementation
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	queue.AddRateLimited(event{})
}

func TestRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	address = "GRAFANA_CLOUD_METRICS_URL"
	basic_auth {
		username = "GRAFANA_CLOUD_USER"
		password = "GRAFANA_CLOUD_API_KEY"
	}
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}

func TestBadRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	address = "GRAFANA_CLOUD_METRICS_URL"
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}
