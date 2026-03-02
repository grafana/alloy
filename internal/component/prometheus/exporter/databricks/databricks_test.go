package databricks

import (
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	server_hostname     = "dbc-abc123.cloud.databricks.com"
	warehouse_http_path = "/sql/1.0/warehouses/xyz789"
	client_id           = "my-client-id"
	client_secret       = "my-client-secret"
	query_timeout       = "10m"
	billing_lookback    = "48h"
	jobs_lookback       = "4h"
	pipelines_lookback  = "4h"
	queries_lookback    = "2h"
	sla_threshold_seconds = 7200
	collect_task_retries  = true
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		ServerHostname:      "dbc-abc123.cloud.databricks.com",
		WarehouseHTTPPath:   "/sql/1.0/warehouses/xyz789",
		ClientID:            "my-client-id",
		ClientSecret:        alloytypes.Secret("my-client-secret"),
		QueryTimeout:        10 * time.Minute,
		BillingLookback:     48 * time.Hour,
		JobsLookback:        4 * time.Hour,
		PipelinesLookback:   4 * time.Hour,
		QueriesLookback:     2 * time.Hour,
		SLAThresholdSeconds: 7200,
		CollectTaskRetries:  true,
	}

	require.Equal(t, expected, args)
}

func TestAlloyUnmarshal_Defaults(t *testing.T) {
	alloyConfig := `
	server_hostname     = "dbc-abc123.cloud.databricks.com"
	warehouse_http_path = "/sql/1.0/warehouses/xyz789"
	client_id           = "my-client-id"
	client_secret       = "my-client-secret"
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	// Check that defaults are applied
	require.Equal(t, 5*time.Minute, args.QueryTimeout)
	require.Equal(t, 24*time.Hour, args.BillingLookback)
	require.Equal(t, 3*time.Hour, args.JobsLookback)
	require.Equal(t, 3*time.Hour, args.PipelinesLookback)
	require.Equal(t, 2*time.Hour, args.QueriesLookback)
	require.Equal(t, 3600, args.SLAThresholdSeconds)
	require.False(t, args.CollectTaskRetries)
}

func TestConfigName(t *testing.T) {
	var args Arguments
	args.SetToDefault()
	cfg := args.toConfig()
	require.Equal(t, "databricks", cfg.Name())
}

func TestConfigInstanceKey(t *testing.T) {
	args := Arguments{
		ServerHostname: "dbc-abc123.cloud.databricks.com",
	}
	cfg := args.toConfig()

	instanceKey, err := cfg.InstanceKey("")
	require.NoError(t, err)
	require.Equal(t, "dbc-abc123.cloud.databricks.com", instanceKey)
}
