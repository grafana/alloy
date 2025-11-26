package splunkhec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/oklog/run"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

type HecEvent struct {
	// type of event: set to "metric" or nil if the event represents a metric, or is the payload of the event.
	Event any `json:"event"`
	// dimensions and metric data
	Fields map[string]any `json:"fields,omitempty"`
	// hostname
	Host string `json:"host"`
	// optional description of the source of the event; typically the app's name
	Source string `json:"source,omitempty"`
	// optional name of a Splunk parsing configuration; this is usually inferred by Splunk
	SourceType string `json:"sourcetype,omitempty"`
	// optional name of the Splunk index to store the event in; not required if the token has a default index set in Splunk
	Index string `json:"index,omitempty"`
	// optional epoch time - set to zero if the event timestamp is missing or unknown (will be added at indexing time)
	Time float64 `json:"time,omitempty"`
}

// Test performs a basic integration test which runs the otelcol.receiver.splunkhec
// component and ensures that it can receive and forward data.
func Test(t *testing.T) {
	httpAddr := componenttest.GetFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.splunkhec")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		endpoint = "%s"

		output {
			// no-op: will be overridden by test code.
		}
	`, httpAddr)
	require.NoError(t, err)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	var (
		logsCh    = make(chan plog.Logs, 1)
		metricsCh = make(chan pmetric.Metrics, 1)
	)

	c := &fakeconsumer.Consumer{
		ConsumeMetricsFunc: func(ctx context.Context, m pmetric.Metrics) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case metricsCh <- m:
				return nil
			}
		},
		ConsumeLogsFunc: func(ctx context.Context, l plog.Logs) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case logsCh <- l:
				return nil
			}
		},
	}

	args.Output = &ConsumerArguments{
		Metrics: []otelcol.Consumer{c},
		Logs:    []otelcol.Consumer{c},
	}

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))

	url := fmt.Sprintf("http://%s", httpAddr)

	// send log event
	go func() {
		util.Eventually(t, func(t require.TestingT) {
			event := HecEvent{
				Event: map[string]string{
					"msg":      "some message",
					"severity": "Info",
				},
				Host:       "my_host",
				Source:     "my_source",
				SourceType: "my_source_type",
				Index:      "index",
				Time:       float64(time.Now().Unix()),
			}

			buf := bytes.NewBuffer([]byte{})
			require.NoError(t, json.NewEncoder(buf).Encode(&event))

			_, err := http.DefaultClient.Post(url, "application/json", buf)
			require.NoError(t, err)
		})
	}()

	go func() {
		// send metric event
		util.Eventually(t, func(t require.TestingT) {
			event := HecEvent{
				Event:      "metric",
				Host:       "my_host",
				Source:     "my_source",
				SourceType: "my_source_type",
				Index:      "index",
				Time:       float64(time.Now().Unix()),
				Fields: map[string]any{
					"metric_name:name": 1,
				},
			}

			buf := bytes.NewBuffer([]byte{})
			require.NoError(t, json.NewEncoder(buf).Encode(&event))
			_, err = http.DefaultClient.Post(url, "application/json", buf)
			require.NoError(t, err)
		})
	}()

	resg := &run.Group{}
	// Wait for our client to get log.
	resg.Add(func() error {
		select {
		case l := <-logsCh:
			if l.LogRecordCount() != 1 {
				return errors.New("expected log count 1")
			}
			return nil
		case <-time.After(time.Second):
			return errors.New("failed waiting for logs")
		}
	}, func(_ error) {})

	// Wait for our client to get metric.
	resg.Add(func() error {
		select {
		case m := <-metricsCh:
			if m.MetricCount() != 1 {
				return errors.New("expected metric count 1")
			}
			return nil
		case <-time.After(time.Second):
			return errors.New("failed waiting for metrics")
		}
	}, func(_ error) {})

	require.NoError(t, resg.Run())
}

func TestUnmarshalDefault(t *testing.T) {
	alloyCfg := `
	 	output {}
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	actual, err := args.Convert()
	require.NoError(t, err)

	expected := splunkhecreceiver.Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint:              "localhost:8088",
			CompressionAlgorithms: slices.Clone(otelcol.DefaultCompressionAlgorithms),
			KeepAlivesEnabled:     true,
		},
		RawPath:    "/services/collector/raw",
		HealthPath: "/services/collector/health",
		Splitting:  splunkhecreceiver.SplittingStrategy(SplittingStrategyLine),
	}

	expected.HecToOtelAttrs.Source = "com.splunk.source"
	expected.HecToOtelAttrs.SourceType = "com.splunk.sourcetype"
	expected.HecToOtelAttrs.Index = "com.splunk.index"
	expected.HecToOtelAttrs.Host = "host.name"

	require.Equal(t, &expected, actual)
}
