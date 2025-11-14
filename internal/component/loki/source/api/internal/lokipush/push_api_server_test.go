package lokipush

// This code is copied from Promtail (3478e180211c17bfe2f3f3305f668d5520f40481) with changes kept to the minimum.
// The lokipush package is used to configure and run the HTTP server that can receive loki push API requests and
// forward them to other loki components.

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	frelabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/syntax"
)

type fakeBatchReceiver struct {
	entries  chan []loki.Entry
	received []loki.Entry
	mtx      sync.Mutex
	wg       sync.WaitGroup
}

func newFakeBatchReceiver() *fakeBatchReceiver {
	c := &fakeBatchReceiver{
		entries: make(chan []loki.Entry),
	}
	c.wg.Go(func() {
		defer c.wg.Done()
		for batch := range c.entries {
			c.mtx.Lock()
			c.received = append(c.received, batch...)
			c.mtx.Unlock()
		}
	})
	return c
}

func (c *fakeBatchReceiver) Chan() chan []loki.Entry {
	return c.entries
}

func (c *fakeBatchReceiver) Received() []loki.Entry {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	cpy := make([]loki.Entry, len(c.received))
	copy(cpy, c.received)
	return cpy
}

func (c *fakeBatchReceiver) Stop() {
	close(c.entries)
	c.wg.Wait()
}

const localhost = "127.0.0.1"

func TestLokiPushTarget(t *testing.T) {
	logger := log.NewNopLogger()
	pt, port, eh := createPushServer(t, logger)

	pt.SetLabels(model.LabelSet{
		"pushserver": "pushserver1",
		"dropme":     "label",
	})
	pt.SetKeepTimestamp(true)

	relabelRule := frelabel.Config{}
	relabelStr := `
action = "labeldrop"
regex = "dropme"
`
	err := syntax.Unmarshal([]byte(relabelStr), &relabelRule)
	require.NoError(t, err)
	pt.SetRelabelRules(frelabel.Rules{&relabelRule})

	// Build a client to send logs
	serverURL := flagext.URLValue{}
	err = serverURL.Set("http://" + localhost + ":" + strconv.Itoa(port) + "/api/v1/push")
	require.NoError(t, err)

	ccfg := client.Config{
		URL:       serverURL,
		Timeout:   1 * time.Second,
		BatchWait: 1 * time.Second,
		BatchSize: 100 * 1024,
	}
	m := client.NewMetrics(prometheus.DefaultRegisterer)
	pc, err := client.New(m, ccfg, logger)
	require.NoError(t, err)
	defer pc.Stop()

	// Send some logs
	labels := model.LabelSet{
		"stream":             "stream1",
		"__anotherdroplabel": "dropme",
	}
	for i := 0; i < 100; i++ {
		pc.Chan() <- loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Unix(int64(i), 0),
				Line:      "line" + strconv.Itoa(i),
				StructuredMetadata: push.LabelsAdapter{
					{Name: "i", Value: strconv.Itoa(i)},
					{Name: "anotherMetaData", Value: "val"},
				},
			},
		}
	}

	// Wait for them to appear in the test handler
	countdown := 10000
	for len(eh.Received()) != 100 && countdown > 0 {
		time.Sleep(1 * time.Millisecond)
		countdown--
	}

	// Make sure we didn't timeout
	require.Equal(t, 100, len(eh.Received()))

	// Verify labels
	expectedLabels := model.LabelSet{
		"pushserver": "pushserver1",
		"stream":     "stream1",
	}

	expectedStructuredMetadata := push.LabelsAdapter{
		{Name: "i", Value: strconv.Itoa(0)},
		{Name: "anotherMetaData", Value: "val"},
	}

	// Spot check the first value in the result to make sure relabel rules were applied properly
	require.Equal(t, expectedLabels, eh.Received()[0].Labels)

	// Spot check the first value in the result to make sure structured metadata was received properly
	require.Equal(t, expectedStructuredMetadata, eh.Received()[0].StructuredMetadata)

	// With keep timestamp enabled, verify timestamp
	require.Equal(t, time.Unix(99, 0).Unix(), eh.Received()[99].Timestamp.Unix())

	pt.Shutdown()
}

func TestLokiPushTargetForRedirect(t *testing.T) {
	logger := log.NewNopLogger()
	pt, port, eh := createPushServer(t, logger)

	pt.SetLabels(model.LabelSet{
		"pushserver": "pushserver1",
		"dropme":     "label",
	})
	pt.SetKeepTimestamp(true)

	relabelRule := frelabel.Config{}
	relabelStr := `
action = "labeldrop"
regex = "dropme"
`
	err := syntax.Unmarshal([]byte(relabelStr), &relabelRule)
	require.NoError(t, err)
	pt.SetRelabelRules(frelabel.Rules{&relabelRule})

	// Build a client to send logs
	serverURL := flagext.URLValue{}
	err = serverURL.Set("http://" + localhost + ":" + strconv.Itoa(port) + "/loki/api/v1/push")
	require.NoError(t, err)

	ccfg := client.Config{
		URL:       serverURL,
		Timeout:   1 * time.Second,
		BatchWait: 1 * time.Second,
		BatchSize: 100 * 1024,
	}
	m := client.NewMetrics(prometheus.DefaultRegisterer)
	pc, err := client.New(m, ccfg, logger)
	require.NoError(t, err)
	defer pc.Stop()

	// Send some logs
	labels := model.LabelSet{
		"stream":             "stream1",
		"__anotherdroplabel": "dropme",
	}
	for i := 0; i < 100; i++ {
		pc.Chan() <- loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Unix(int64(i), 0),
				Line:      "line" + strconv.Itoa(i),
			},
		}
	}

	// Wait for them to appear in the test handler
	countdown := 10000
	for len(eh.Received()) != 100 && countdown > 0 {
		time.Sleep(1 * time.Millisecond)
		countdown--
	}

	// Make sure we didn't timeout
	require.Equal(t, 100, len(eh.Received()))

	// Verify labels
	expectedLabels := model.LabelSet{
		"pushserver": "pushserver1",
		"stream":     "stream1",
	}
	// Spot check the first value in the result to make sure relabel rules were applied properly
	require.Equal(t, expectedLabels, eh.Received()[0].Labels)

	// With keep timestamp enabled, verify timestamp
	require.Equal(t, time.Unix(99, 0).Unix(), eh.Received()[99].Timestamp.Unix())

	pt.Shutdown()
}

func TestLokiPushTargetWithXScopeOrgIDHeader(t *testing.T) {
	logger := log.NewNopLogger()
	pt, port, eh := createPushServer(t, logger)

	pt.SetLabels(model.LabelSet{
		"pushserver": "pushserver1",
		"dropme":     "label",
	})
	pt.SetKeepTimestamp(true)

	relabelRule := frelabel.Config{}
	relabelStr := `
action = "labeldrop"
regex = "dropme"
`
	err := syntax.Unmarshal([]byte(relabelStr), &relabelRule)
	require.NoError(t, err)
	pt.SetRelabelRules(frelabel.Rules{&relabelRule})

	// Build a client to send logs
	serverURL := flagext.URLValue{}
	err = serverURL.Set("http://" + localhost + ":" + strconv.Itoa(port) + "/api/v1/push")
	require.NoError(t, err)

	ccfg := client.Config{
		URL:       serverURL,
		Timeout:   1 * time.Second,
		BatchWait: 1 * time.Second,
		BatchSize: 100 * 1024,
		Headers: map[string]string{
			"X-Scope-OrgID": "tenant1",
		},
	}
	m := client.NewMetrics(prometheus.DefaultRegisterer)
	pc, err := client.New(m, ccfg, logger)
	require.NoError(t, err)
	defer pc.Stop()

	// Send some logs
	labels := model.LabelSet{
		"stream":             "stream1",
		"__anotherdroplabel": "dropme",
	}
	for i := range 100 {
		pc.Chan() <- loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Unix(int64(i), 0),
				Line:      "line" + strconv.Itoa(i),
				StructuredMetadata: push.LabelsAdapter{
					{Name: "i", Value: strconv.Itoa(i)},
					{Name: "anotherMetaData", Value: "val"},
				},
			},
		}
	}

	// Wait for them to appear in the test handler
	countdown := 10000
	for len(eh.Received()) != 100 && countdown > 0 {
		time.Sleep(1 * time.Millisecond)
		countdown--
	}

	// Make sure we didn't timeout
	require.Equal(t, 100, len(eh.Received()))

	// Verify labels
	expectedLabels := model.LabelSet{
		"pushserver":    "pushserver1",
		"stream":        "stream1",
		"__tenant_id__": "tenant1",
	}

	expectedStructuredMetadata := push.LabelsAdapter{
		{Name: "i", Value: strconv.Itoa(0)},
		{Name: "anotherMetaData", Value: "val"},
	}

	// Spot check the first value in the result to make sure relabel rules were applied properly
	require.Equal(t, expectedLabels, eh.Received()[0].Labels)

	// Spot check the first value in the result to make sure structured metadata was received properly
	require.Equal(t, expectedStructuredMetadata, eh.Received()[0].StructuredMetadata)

	// With keep timestamp enabled, verify timestamp
	require.Equal(t, time.Unix(99, 0).Unix(), eh.Received()[99].Timestamp.Unix())

	pt.Shutdown()
}

func TestPlaintextPushTarget(t *testing.T) {
	logger := log.NewNopLogger()
	//Create PushAPIServerOld
	eh := newFakeBatchReceiver()
	defer eh.Stop()

	// Get a randomly available port by open and closing a TCP socket
	addr, err := net.ResolveTCPAddr("tcp", localhost+":0")
	require.NoError(t, err)
	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	err = l.Close()
	require.NoError(t, err)

	serverConfig := &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: localhost,
			ListenPort:    port,
		},
		GRPC: &fnet.GRPCConfig{ListenPort: getFreePort(t)},
	}

	pt, err := NewPushAPIServer(logger, serverConfig, eh, prometheus.NewRegistry(), 0)
	require.NoError(t, err)

	err = pt.Run()
	require.NoError(t, err)

	pt.SetLabels(model.LabelSet{
		"pushserver": "pushserver2",
		"keepme":     "label",
	})
	pt.SetKeepTimestamp(true)

	// Send some logs
	ts := time.Now()
	body := new(bytes.Buffer)
	for i := range 100 {
		body.WriteString("line" + strconv.Itoa(i))
		_, err := http.Post(fmt.Sprintf("http://%s:%d/api/v1/raw", localhost, port), "text/json", body)
		require.NoError(t, err)
		body.Reset()
	}

	// Wait for them to appear in the test handler
	countdown := 10000
	for len(eh.Received()) != 100 && countdown > 0 {
		time.Sleep(1 * time.Millisecond)
		countdown--
	}

	// Make sure we didn't timeout
	require.Equal(t, 100, len(eh.Received()))

	// Verify labels
	expectedLabels := model.LabelSet{
		"pushserver": "pushserver2",
		"keepme":     "label",
	}
	// Spot check the first value in the result to make sure relabel rules were applied properly
	require.Equal(t, expectedLabels, eh.Received()[0].Labels)

	// Timestamp is always set in the handler, we expect received timestamps to be slightly higher than the timestamp when we started sending logs.
	require.GreaterOrEqual(t, eh.Received()[99].Timestamp.Unix(), ts.Unix())

	pt.Shutdown()
}

func TestPlaintextPushTargetWithXScopeOrgIDHeader(t *testing.T) {
	logger := log.NewNopLogger()
	//Create PushAPIServerOld

	eh := newFakeBatchReceiver()
	defer eh.Stop()

	// Get a randomly available port by open and closing a TCP socket
	addr, err := net.ResolveTCPAddr("tcp", localhost+":0")
	require.NoError(t, err)
	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	err = l.Close()
	require.NoError(t, err)

	serverConfig := &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: localhost,
			ListenPort:    port,
		},
		GRPC: &fnet.GRPCConfig{ListenPort: getFreePort(t)},
	}

	pt, err := NewPushAPIServer(logger, serverConfig, eh, prometheus.NewRegistry(), 0)
	require.NoError(t, err)

	err = pt.Run()
	require.NoError(t, err)

	pt.SetLabels(model.LabelSet{
		"pushserver": "pushserver2",
		"keepme":     "label",
	})
	pt.SetKeepTimestamp(true)

	// Send some logs
	ts := time.Now()
	body := new(bytes.Buffer)
	client := &http.Client{}
	for i := 0; i < 100; i++ {
		body.WriteString("line" + strconv.Itoa(i))
		url := fmt.Sprintf("http://%s:%d/api/v1/raw", localhost, port)

		// Create a new request
		req, err := http.NewRequest("POST", url, body)
		require.NoError(t, err)
		req.Header.Add("Content-Type", "text/json")
		req.Header.Add("X-Scope-OrgID", "tenant1")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body.Reset()
	}

	// Wait for them to appear in the test handler
	countdown := 10000
	for len(eh.Received()) != 100 && countdown > 0 {
		time.Sleep(1 * time.Millisecond)
		countdown--
	}

	// Make sure we didn't timeout
	require.Equal(t, 100, len(eh.Received()))

	// Verify labels
	expectedLabels := model.LabelSet{
		"pushserver": "pushserver2",
		"keepme":     "label",
	}
	// Spot check the first value in the result to make sure relabel rules were applied properly
	require.Equal(t, expectedLabels, eh.Received()[0].Labels)

	// Timestamp is always set in the handler, we expect received timestamps to be slightly higher than the timestamp when we started sending logs.
	require.GreaterOrEqual(t, eh.Received()[99].Timestamp.Unix(), ts.Unix())

	pt.Shutdown()
}

func TestReady(t *testing.T) {
	logger := log.NewNopLogger()
	// Get a randomly available port by open and closing a TCP socket
	addr, err := net.ResolveTCPAddr("tcp", localhost+":0")
	require.NoError(t, err)
	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	err = l.Close()
	require.NoError(t, err)

	serverConfig := &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: localhost,
			ListenPort:    port,
		},
		GRPC: &fnet.GRPCConfig{ListenPort: getFreePort(t)},
	}

	pt, err := NewPushAPIServer(logger, serverConfig, nil, prometheus.NewRegistry(), 100<<20)
	require.NoError(t, err)

	err = pt.Run()
	require.NoError(t, err)

	pt.SetLabels(model.LabelSet{
		"pushserver": "pushserver2",
		"keepme":     "label",
	})
	pt.SetKeepTimestamp(true)

	url := fmt.Sprintf("http://%s:%d/ready", localhost, port)
	response, err := http.Get(url)
	if err != nil {
		require.NoError(t, err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		require.NoError(t, err)
	}
	responseCode := fmt.Sprint(response.StatusCode)
	responseBody := string(body)

	fmt.Println(responseBody)
	wantedResponse := "ready"
	if responseBody != wantedResponse {
		t.Errorf("got the response %q, want %q", responseBody, wantedResponse)
	}
	wantedCode := "200"
	if responseCode != wantedCode {
		t.Errorf("Got the response code %q, want %q", responseCode, wantedCode)
	}

	t.Cleanup(pt.Shutdown)
}

func getFreePort(t *testing.T) int {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	return port
}

func createPushServer(t *testing.T, logger log.Logger) (*PushAPIServer, int, *fakeBatchReceiver) {
	//Create PushAPIServerOld
	eh := newFakeBatchReceiver()
	t.Cleanup(func() {
		eh.Stop()
	})

	// Get a randomly available port by open and closing a TCP socket
	port := getFreePort(t)

	serverConfig := &fnet.ServerConfig{
		HTTP: &fnet.HTTPConfig{
			ListenAddress: localhost,
			ListenPort:    port,
		},
		GRPC: &fnet.GRPCConfig{ListenPort: getFreePort(t)},
	}

	pt, err := NewPushAPIServer(logger, serverConfig, eh, prometheus.NewRegistry(), 100<<20)
	require.NoError(t, err)

	err = pt.Run()
	require.NoError(t, err)
	return pt, port, eh
}
