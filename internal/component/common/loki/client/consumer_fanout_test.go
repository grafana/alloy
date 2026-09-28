package client

import (
	"net/url"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestFanoutConsumer(t *testing.T) {
	runConsumerTest(t, func(t *testing.T, consume consumeFunc) {
		testEndpointConfig, receivedRequests, closeServer := newServerAndEndpointConfig(t)

		consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), testEndpointConfig)
		require.NoError(t, err)
		consumer.Start()

		defer func() {
			consumer.Stop()
			closeServer()
		}()

		entries := newTestEntries(model.LabelSet{"pizza-flavour": "fugazzeta"}, 100)
		require.NoError(t, consume(t.Context(), consumer, entries))
		requireReceivedEntries(t, receivedRequests, entries)
	})
}

func TestFanoutConsumer_MultipleConfigs(t *testing.T) {
	runConsumerTest(t, func(t *testing.T, consume consumeFunc) {
		testEndpointConfig, receivedRequests, closeServer := newServerAndEndpointConfig(t)
		testEndpointConfig2, receivedRequests2, closeServer2 := newServerAndEndpointConfig(t)
		testEndpointConfig2.Name = "test-client-2"

		consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), testEndpointConfig, testEndpointConfig2)
		require.NoError(t, err)
		consumer.Start()

		defer func() {
			consumer.Stop()
			closeServer()
			closeServer2()
		}()

		entries := newTestEntries(model.LabelSet{"pizza-flavour": "fugazzeta"}, 100)
		require.NoError(t, consume(t.Context(), consumer, entries))
		requireReceivedEntries(t, receivedRequests, entries)
		requireReceivedEntries(t, receivedRequests2, entries)
	})
}

func TestFanoutConsumer_InvalidConfig(t *testing.T) {
	t.Run("no endpoints", func(t *testing.T) {
		_, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry())
		require.Error(t, err)
	})

	t.Run("repeated endpoint", func(t *testing.T) {
		host, _ := url.Parse("http://localhost:3100")
		config := Config{URL: flagext.URLValue{URL: host}}
		_, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), config, config)
		require.Error(t, err)
	})
}

func TestFanoutConsumer_NoDuplicateMetricsPanic(t *testing.T) {
	var (
		host, _ = url.Parse("http://localhost:3100")
		reg     = prometheus.NewRegistry()
	)

	require.NotPanics(t, func() {
		for range 2 {
			_, err := NewFanoutConsumer(logging.NewSlogNop(), reg, Config{URL: flagext.URLValue{URL: host}})
			require.NoError(t, err)
		}
	})
}

func TestFanoutConsumer_StopWithFullSendQueue(t *testing.T) {
	runConsumerTest(t, func(t *testing.T, consume consumeFunc) {
		const drainTimeout = time.Second

		server, blocked, release := newBlockedServer()
		defer server.Close()
		defer release()

		serverURL, err := url.Parse(server.URL)
		require.NoError(t, err)

		endpointConfig := Config{
			Name: "test-client",
			URL:  flagext.URLValue{URL: serverURL},
			// Long enough that the in-flight request stays parked for the whole test.
			Timeout:   time.Minute,
			BatchSize: 1,
			BackoffConfig: backoff.Config{
				MinBackoff: time.Millisecond,
				MaxBackoff: 10 * time.Millisecond,
				MaxRetries: 0,
			},
			QueueConfig: QueueConfig{
				Capacity:        1,
				MinShards:       1,
				DrainTimeout:    drainTimeout,
				BlockOnOverflow: true,
			},
		}

		consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), endpointConfig)
		require.NoError(t, err)
		consumer.Start()

		feedUntilBlocked(t, blocked, consumer, consume)

		done := make(chan struct{})
		go func() {
			consumer.Stop()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * drainTimeout):
			release()
			t.Fatal("Stop did not finish in time")
		}
	})
}

func TestFanoutConsumer_NoLeakOnFailedEndpoint(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	host, err := url.Parse("http://localhost:3100")
	require.NoError(t, err)

	var (
		cfg = Config{URL: flagext.URLValue{URL: host}}
		// HTTPClientConfig.Validate allows at most one bearer token source, so
		// creating the endpoint for this config fails.
		invalidClientCfg = Config{
			URL: flagext.URLValue{URL: host},
			Client: config.HTTPClientConfig{
				BearerToken:     "my-token",
				BearerTokenFile: "my-token-file",
			},
		}
	)

	// Using same config twice.
	_, err = NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), cfg, cfg)
	require.Error(t, err)

	// Using two different configs but endpoint cannot be created by the second one.
	_, err = NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), cfg, invalidClientCfg)
	require.Error(t, err)
}
