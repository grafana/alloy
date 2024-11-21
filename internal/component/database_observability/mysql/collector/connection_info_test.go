package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestConnectionInfoRun(t *testing.T) {
	const baseExpectedMetrics = `
	# HELP connection_info Information about the connection
	# TYPE connection_info gauge
	connection_info{db_instance_identifier="%s",provider_name="%s",region="%s"} 1
`

	testCases := []struct {
		name            string
		dsn             string
		expectedMetrics string
	}{
		{
			name:            "generic dsn",
			dsn:             "user:pass@tcp(localhost:3306)/db",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "", "", ""),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/db",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "aws", "us-east-1"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:      tc.dsn,
			Registry: reg,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Run(context.Background())
		require.NoError(t, err)

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)
	}
}
