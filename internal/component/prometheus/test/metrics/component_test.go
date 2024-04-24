package metrics

import (
	"context"
	"fmt"
	"github.com/grafana/alloy/internal/util"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestMetricsGeneration(t *testing.T) {
	lg := util.TestLogger(t)
	opts := component.Options{
		Logger:     lg,
		Registerer: prometheus_client.NewRegistry(),
		GetServiceData: func(name string) (interface{}, error) {
			switch name {
			case http_service.ServiceName:
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "agent.internal:1245",
					BaseHTTPPath:     "/",
					DialFunc:         (&net.Dialer{}).DialContext,
				}, nil
			case labelstore.ServiceName:
				return labelstore.New(lg, nil), nil
			default:
				return nil, fmt.Errorf("service %q does not exist", name)
			}
		},
		OnStateChange: func(e component.Exports) {

		},
	}

	s, err := NewComponent(opts, Arguments{
		NumberOfInstances: 1,
		NumberOfMetrics:   1,
		NumberOfSeries:    1,
		MetricsRefresh:    1 * time.Minute,
		ChurnPercent:      0,
	})
	require.NoError(t, err)
	ctx := context.Background()
	ctx, cncl := context.WithTimeout(ctx, 20*time.Second)
	defer cncl()
	go s.Run(ctx)
	var bb [][]byte
	require.Eventually(t, func() bool {
		bb = s.data()
		return len(bb) > 0 && len(bb[0]) > 2
	}, 10*time.Second, 100*time.Millisecond)
	require.Len(t, bb, 1)
	metrics := string(bb[0])
	require.True(t, strings.Contains(metrics, "counter"))
	require.True(t, strings.Contains(metrics, "agent_metric_test_0"))

}
