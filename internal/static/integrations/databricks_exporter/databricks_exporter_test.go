package databricks_exporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig_UnmarshalYAML(t *testing.T) {
	yamlConfig := `
server_hostname: dbc-abc123.cloud.databricks.com
warehouse_http_path: /sql/1.0/warehouses/xyz789
client_id: my-client-id
client_secret: my-client-secret
query_timeout: 10m
billing_lookback: 48h
jobs_lookback: 4h
pipelines_lookback: 4h
queries_lookback: 2h
sla_threshold_seconds: 7200
collect_task_retries: true
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	require.NoError(t, err)

	expected := Config{
		ServerHostname:      "dbc-abc123.cloud.databricks.com",
		WarehouseHTTPPath:   "/sql/1.0/warehouses/xyz789",
		ClientID:            "my-client-id",
		ClientSecret:        "my-client-secret",
		QueryTimeout:        10 * time.Minute,
		BillingLookback:     48 * time.Hour,
		JobsLookback:        4 * time.Hour,
		PipelinesLookback:   4 * time.Hour,
		QueriesLookback:     2 * time.Hour,
		SLAThresholdSeconds: 7200,
		CollectTaskRetries:  true,
	}

	require.Equal(t, expected, config)
}

func TestConfig_UnmarshalYAML_Defaults(t *testing.T) {
	yamlConfig := `
server_hostname: dbc-abc123.cloud.databricks.com
warehouse_http_path: /sql/1.0/warehouses/xyz789
client_id: my-client-id
client_secret: my-client-secret
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	require.NoError(t, err)

	// Check that defaults are applied
	require.Equal(t, 5*time.Minute, config.QueryTimeout)
	require.Equal(t, 24*time.Hour, config.BillingLookback)
	require.Equal(t, 3*time.Hour, config.JobsLookback)
	require.Equal(t, 3*time.Hour, config.PipelinesLookback)
	require.Equal(t, 2*time.Hour, config.QueriesLookback)
	require.Equal(t, 3600, config.SLAThresholdSeconds)
	require.False(t, config.CollectTaskRetries)
}

func TestConfig_Name(t *testing.T) {
	config := Config{}
	require.Equal(t, "databricks", config.Name())
}

func TestConfig_InstanceKey(t *testing.T) {
	config := Config{
		ServerHostname: "dbc-abc123.cloud.databricks.com",
	}

	instanceKey, err := config.InstanceKey("")
	require.NoError(t, err)
	require.Equal(t, "dbc-abc123.cloud.databricks.com", instanceKey)
}
