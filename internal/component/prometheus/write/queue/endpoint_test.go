package queue

import (
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	reg := prometheus.NewRegistry()
	end, err := newEndpoint(EndpointConfig{
		Name: "test",
		URL:  "example.com",
	}, 1*time.Minute, 10, 5*time.Second, t.TempDir(), reg, log.NewNopLogger())
	require.NoError(t, err)
	// This will unregister the metrics
	end.Start()
	end.Stop()

	// This will trigger a panic if duplicate metrics found.
	end2, err := newEndpoint(EndpointConfig{
		Name: "test",
		URL:  "example.com",
	}, 1*time.Minute, 10, 5*time.Second, t.TempDir(), reg, log.NewNopLogger())
	require.NoError(t, err)
	end2.Start()
	end2.Stop()
}
