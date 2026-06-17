package aws_firehose

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/regexp"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloy_config "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const singleRecordRequest = `{"requestId":"a1af4300-6c09-4916-ba8f-12f336176246","timestamp":1684422829730,"records":[{"data":"eyJDSEFOR0UiOi0wLjIzLCJQUklDRSI6NC44LCJUSUNLRVJfU1lNQk9MIjoiTkdDIiwiU0VDVE9SIjoiSEVBTFRIQ0FSRSJ9"}]}`

const expectedRecord = "{\"CHANGE\":-0.23,\"PRICE\":4.8,\"TICKER_SYMBOL\":\"NGC\",\"SECTOR\":\"HEALTHCARE\"}"

func TestComponent(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	opts := component.Options{
		ID:            "loki.source.awsfirehose",
		Logger:        logging.NewSlogNop(),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	collector1, collector2 := loki.NewCollectingConsumer(), loki.NewCollectingConsumer()

	args := Arguments{}

	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args.Server = &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: "localhost",
			ListenPort:    port,
		},
		// assign random grpc port
		GRPC: &fnet.GRPCConfig{ListenPort: 0},
	}
	args.ForwardTo = []loki.Consumer{collector1, collector2}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup

	wg.Go(func() {
		_ = c.Run(ctx)
	})

	// small wait for server start
	time.Sleep(200 * time.Millisecond)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/awsfirehose/api/v1/push", port), strings.NewReader(singleRecordRequest))
	require.NoError(t, err)

	// create client with timeout
	client := http.Client{
		Timeout: time.Second * 5,
	}

	res, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Len(c, collector1.Entries(), 1)
		require.Len(c, collector2.Entries(), 1)
	}, time.Second*10, time.Second, "timed out waiting for receivers to get all messages")

	require.JSONEq(t, expectedRecord, collector1.Entries()[0].Entry.Line)
	require.JSONEq(t, expectedRecord, collector2.Entries()[0].Entry.Line)
	cancel()
	wg.Wait()
}

func TestComponent_UpdateWithNewArguments(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	opts := component.Options{
		ID:            "loki.source.awsfirehose",
		Logger:        logging.NewSlogNop(),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	collector1, collector2 := loki.NewCollectingConsumer(), loki.NewCollectingConsumer()

	args := Arguments{}

	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	// port2 will be used to restart server on another port, and test it's relaunched
	port2, err := freeport.GetFreePort()
	require.NoError(t, err)

	args.Server = &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: "localhost",
			ListenPort:    port,
		},
		// assign random grpc port
		GRPC: &fnet.GRPCConfig{ListenPort: 0},
	}
	args.ForwardTo = []loki.Consumer{collector1}
	args.RelabelRules = alloy_config.Rules{
		{
			SourceLabels: []string{"__aws_firehose_source_arn"},
			Regex:        alloy_config.Regexp{Regexp: regexp.MustCompile("(.*)")},
			Replacement:  "$1",
			TargetLabel:  "source_arn",
			Action:       alloy_config.Replace,
		},
	}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	wg.Go(func() {
		_ = c.Run(ctx)
	})

	// small wait for server start
	time.Sleep(200 * time.Millisecond)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/awsfirehose/api/v1/push", port), strings.NewReader(singleRecordRequest))
	require.NoError(t, err)
	req.Header.Set("X-Amz-Firehose-Source-Arn", "testarn")

	// create client with timeout
	client := http.Client{
		Timeout: time.Second * 5,
	}

	// assert over message received with relabels

	res, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Len(c, collector1.Entries(), 1)
	}, time.Second*10, time.Second, "timed out waiting for receivers to get all messages")

	require.JSONEq(t, expectedRecord, collector1.Entries()[0].Entry.Line)
	require.Equal(t, "testarn", string(collector1.Entries()[0].Labels["source_arn"]))

	collector1.Reset()

	// create new config without relabels, and adding a new forward
	args2 := Arguments{
		ForwardTo: []loki.Consumer{collector1, collector2},
	}
	args2.Server = &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: "0.0.0.0",
			ListenPort:    port2,
		},
		GRPC: &fnet.GRPCConfig{ListenPort: 0},
	}
	require.NoError(t, c.Update(args2))
	time.Sleep(200 * time.Millisecond)

	_, err = client.Do(req)
	require.Error(t, err, "now that the port change, the first request should have errored")

	req2, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/awsfirehose/api/v1/push", port2), strings.NewReader(singleRecordRequest))
	require.NoError(t, err)
	req2.Header.Set("X-Amz-Firehose-Source-Arn", "testarn")

	res, err = client.Do(req2)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Len(c, collector1.Entries(), 1)
		require.Len(c, collector2.Entries(), 1)
	}, time.Second*10, time.Second, "timed out waiting for receivers to get all messages")

	require.JSONEq(t, expectedRecord, collector1.Entries()[0].Entry.Line)
	require.NotContains(t, collector1.Entries()[0].Labels, model.LabelName("source_arn"), "expected received entry to not contain label")
	require.JSONEq(t, expectedRecord, collector2.Entries()[0].Entry.Line)
	require.NotContains(t, collector2.Entries()[0].Labels, model.LabelName("source_arn"), "expected received entry to not contain label")

	cancel()
	wg.Wait()
}
