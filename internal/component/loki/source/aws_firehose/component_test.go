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
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloy_config "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/util"
)

const singleRecordRequest = `{"requestId":"a1af4300-6c09-4916-ba8f-12f336176246","timestamp":1684422829730,"records":[{"data":"eyJDSEFOR0UiOi0wLjIzLCJQUklDRSI6NC44LCJUSUNLRVJfU1lNQk9MIjoiTkdDIiwiU0VDVE9SIjoiSEVBTFRIQ0FSRSJ9"}]}`

const expectedRecord = "{\"CHANGE\":-0.23,\"PRICE\":4.8,\"TICKER_SYMBOL\":\"NGC\",\"SECTOR\":\"HEALTHCARE\"}"

// receiver implements a simple routine that receives loki.Entry from a channel and
// stores them in a slice for later assertion.
type receiver struct {
	ch       chan loki.Entry
	received []loki.Entry
	mux      sync.RWMutex
}

// newReceiver creates a new receiver.
func newReceiver(ch chan loki.Entry) *receiver {
	return &receiver{
		ch:       ch,
		received: make([]loki.Entry, 0),
	}
}

// run runs the main receiver routine, until the passed context is canceled.
func (r *receiver) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-r.ch:
			r.mux.Lock()
			r.received = append(r.received, e)
			r.mux.Unlock()
		}
	}
}

func TestComponentFromNestedController(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	opts := component.Options{
		ID:            "foo/loki.source.awsfirehose.default",
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}
	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	r1, r2 := newReceiver(ch1.Chan()), newReceiver(ch2.Chan())

	// call cancelReceivers to terminate them
	receiverContext, cancelReceivers := context.WithCancel(t.Context())
	go r1.run(receiverContext)
	go r2.run(receiverContext)

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
	args.ForwardTo = []loki.LogsReceiver{ch1, ch2}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)
	require.NotNil(t, c)

	cancelReceivers()
}

func TestComponent(t *testing.T) {
	opts := component.Options{
		ID:            "loki.source.awsfirehose",
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	r1, r2 := newReceiver(ch1.Chan()), newReceiver(ch2.Chan())

	// call cancelReceivers to terminate them
	receiverContext, cancelReceivers := context.WithCancel(t.Context())
	go r1.run(receiverContext)
	go r2.run(receiverContext)

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
	args.ForwardTo = []loki.LogsReceiver{ch1, ch2}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	componentCtx, cancelComponent := context.WithCancel(t.Context())
	go c.Run(componentCtx)
	defer cancelComponent()

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
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.Eventually(t, func() bool {
		r1.mux.RLock()
		r2.mux.RLock()
		defer func() {
			r1.mux.RUnlock()
			r2.mux.RUnlock()
		}()
		return len(r1.received) == 1 && len(r2.received) == 1
	}, time.Second*10, time.Second, "timed out waiting for receivers to get all messages")

	cancelReceivers()

	// r1 and r2 should have received one entry each
	r1.mux.RLock()
	r2.mux.RLock()
	defer func() {
		r1.mux.RUnlock()
		r2.mux.RUnlock()
	}()
	require.Len(t, r1.received, 1)
	require.Len(t, r2.received, 1)
	require.JSONEq(t, expectedRecord, r1.received[0].Line)
	require.JSONEq(t, expectedRecord, r2.received[0].Line)
}

func TestComponent_UpdateWithNewArguments(t *testing.T) {
	opts := component.Options{
		ID:            "loki.source.awsfirehose",
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	r1, r2 := newReceiver(ch1.Chan()), newReceiver(ch2.Chan())

	// call cancelReceivers to terminate them
	receiverContext, cancelReceivers := context.WithCancel(t.Context())
	go r1.run(receiverContext)
	go r2.run(receiverContext)
	defer cancelReceivers()

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
	args.ForwardTo = []loki.LogsReceiver{ch1}
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

	componentCtx, cancelComponent := context.WithCancel(t.Context())
	go c.Run(componentCtx)
	defer cancelComponent()

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
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.Eventually(t, func() bool {
		r1.mux.RLock()
		defer r1.mux.RUnlock()
		return len(r1.received) == 1
	}, time.Second*10, time.Second, "timed out waiting for receivers to get all messages")

	r1.mux.RLock()
	require.Len(t, r1.received, 1)
	require.JSONEq(t, expectedRecord, r1.received[0].Line)
	require.Equal(t, "testarn", string(r1.received[0].Labels["source_arn"]))
	r1.mux.RUnlock()

	//
	// create new config without relabels, and adding a new forward
	//

	args2 := Arguments{
		ForwardTo: []loki.LogsReceiver{ch1, ch2},
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

	// clear received entries
	r1.received = nil

	_, err = client.Do(req)
	require.Error(t, err, "now that the port change, the first request should have errored")

	req2, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/awsfirehose/api/v1/push", port2), strings.NewReader(singleRecordRequest))
	require.NoError(t, err)
	req2.Header.Set("X-Amz-Firehose-Source-Arn", "testarn")

	res, err = client.Do(req2)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.Eventually(t, func() bool {
		r1.mux.RLock()
		r2.mux.RLock()
		defer func() {
			r1.mux.RUnlock()
			r2.mux.RUnlock()
		}()
		return len(r1.received) == 1 && len(r2.received) == 1
	}, time.Second*10, time.Second, "timed out waiting for receivers to get all messages")

	r1.mux.RLock()
	r2.mux.RLock()
	defer func() {
		r1.mux.RUnlock()
		r2.mux.RUnlock()
	}()
	require.Len(t, r1.received, 1)
	require.Len(t, r2.received, 1)
	require.JSONEq(t, expectedRecord, r1.received[0].Line)
	require.NotContains(t, r1.received[0].Labels, model.LabelName("source_arn"), "expected received entry to not contain label")
	require.JSONEq(t, expectedRecord, r2.received[0].Line)
	require.NotContains(t, r2.received[0].Labels, model.LabelName("source_arn"), "expected received entry to not contain label")
}
