package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/grafana/regexp"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	promCfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// generateTestCertAndKey generates a self-signed certificate and private key for testing
func generateTestCertAndKey() (string, string, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Org"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:    []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	return string(certPEM), string(keyPEM), nil
}

func TestLokiSourceAPI_Simple(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	args := testArgsWith(func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8532
		a.ForwardTo = []loki.LogsReceiver{handler.Receiver()}
		a.UseIncomingTimestamp = true
	})
	opts := defaultOptions()
	_ = startTestComponent(t, opts, args, ctx)

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	now := time.Now()
	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  push.Entry{Timestamp: now, Line: "hello world!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via loki client")
	}

	require.Eventually(
		t,
		func() bool { return len(handler.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received := handler.Received()[0]
	assert.Equal(t, received.Line, "hello world!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"source": "test",
		"foo":    "bar",
		"fizz":   "buzz",
	})
}

func TestLokiSourceAPI_Update(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	receiver := loki.NewCollectingHandler()
	defer receiver.Stop()

	args := testArgsWith(func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8583
		a.ForwardTo = []loki.LogsReceiver{receiver.Receiver()}
		a.UseIncomingTimestamp = true
		a.Labels = map[string]string{"test_label": "before"}
	})
	opts := defaultOptions()
	c := startTestComponent(t, opts, args, ctx)

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	now := time.Now()
	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  push.Entry{Timestamp: now, Line: "hello world!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via loki client")
	}

	require.Eventually(
		t,
		func() bool { return len(receiver.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received := receiver.Received()[0]
	assert.Equal(t, received.Line, "hello world!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"test_label": "before",
		"source":     "test",
	})

	args.Labels = map[string]string{"test_label": "after"}
	err := c.Update(args)
	require.NoError(t, err)

	receiver.Clear()

	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  push.Entry{Timestamp: now, Line: "hello brave new world!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via loki client")
	}
	require.Eventually(
		t,
		func() bool { return len(receiver.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received = receiver.Received()[0]
	assert.Equal(t, received.Line, "hello brave new world!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"test_label": "after",
		"source":     "test",
	})
}

func TestLokiSourceAPI_FanOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	const receiversCount = 10
	var receivers = make([]*loki.CollectingHandler, receiversCount)
	for i := range receiversCount {
		receivers[i] = loki.NewCollectingHandler()
	}

	args := testArgsWith(func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8537
		a.ForwardTo = mapToChannels(receivers)
	})
	opts := defaultOptions()

	comp, err := New(opts, args)
	require.NoError(t, err)
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	const messagesCount = 100
	for i := range messagesCount {
		entry := loki.Entry{
			Labels: map[model.LabelName]model.LabelValue{"source": "test"},
			Entry:  push.Entry{Line: fmt.Sprintf("test message #%d", i)},
		}
		select {
		case lokiClient.Chan() <- entry:
		case <-ctx.Done():
			t.Log("timed out while sending test entries via loki client")
		}
	}

	require.Eventually(
		t,
		func() bool {
			for i := range receiversCount {
				if len(receivers[i].Received()) != messagesCount {
					return false
				}
			}
			return true
		},
		5*time.Second,
		10*time.Millisecond,
		"did not receive all the expected messages within the timeout",
	)
}

func TestComponent_detectsWhenUpdateRequiresARestart(t *testing.T) {
	tests := []struct {
		name            string
		args            Arguments
		newArgs         Arguments
		changeHttpPort  bool
		restartRequired bool
	}{
		{
			name:            "identical args don't require server restart",
			args:            testArgs(),
			newArgs:         testArgs(),
			restartRequired: false,
		},
		{
			name: "change in address requires server restart",
			args: testArgs(),
			newArgs: testArgsWith(func(args *Arguments) {
				args.Server.HTTP.ListenAddress = "localhost"
			}),
			restartRequired: true,
		},
		{
			name:            "change in port requires server restart",
			args:            testArgs(),
			changeHttpPort:  true,
			newArgs:         testArgs(),
			restartRequired: true,
		},
		{
			name: "change in forwardTo does not require server restart",
			args: testArgs(),
			newArgs: testArgsWith(func(args *Arguments) {
				args.ForwardTo = []loki.LogsReceiver{}
			}),
			restartRequired: false,
		},
		{
			name: "change in labels does not require server restart",
			args: testArgs(),
			newArgs: testArgsWith(func(args *Arguments) {
				args.Labels = map[string]string{"some": "label"}
			}),
			restartRequired: false,
		},
		{
			name: "change in relabel rules does not require server restart",
			args: testArgs(),
			newArgs: testArgsWith(func(args *Arguments) {
				args.RelabelRules = relabel.Rules{}
			}),
			restartRequired: false,
		},
		{
			name: "change in use incoming timestamp does not require server restart",
			args: testArgs(),
			newArgs: testArgsWith(func(args *Arguments) {
				args.UseIncomingTimestamp = !args.UseIncomingTimestamp
			}),
			restartRequired: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			comp := startTestComponent(t, defaultOptions(), tc.args, ctx)

			serverBefore := comp.server

			if tc.changeHttpPort {
				httpPort, err := freeport.GetFreePort()
				require.NoError(t, err)
				tc.newArgs.Server.HTTP.ListenPort = httpPort
			}

			require.NoError(t, comp.Update(tc.newArgs))

			restarted := serverBefore != comp.server
			assert.Equal(t, restarted, tc.restartRequired)

			// in order to cleanly shutdown, we want to make sure the server is running first.
			waitForServerToBeReady(t, comp)
		})
	}
}

func TestLokiSourceAPI_TLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Generate test certificate and key for TLS
	testCert, testKey, err := generateTestCertAndKey()
	require.NoError(t, err)

	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	args := testArgsWith(func(a *Arguments) {
		a.Server.HTTP.TLSConfig = &fnet.TLSConfig{
			Cert: testCert,
			Key:  alloytypes.Secret(testKey),
		}
		a.ForwardTo = []loki.LogsReceiver{handler.Receiver()}
		a.UseIncomingTimestamp = true
	})
	opts := defaultOptions()
	c := startTestComponent(t, opts, args, ctx)

	// Create TLS-enabled Loki client
	lokiClient := newTestLokiClientTLS(t, c.server.HTTPListenAddress(), opts)
	defer lokiClient.Stop()

	now := time.Now()
	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  push.Entry{Timestamp: now, Line: "hello world over TLS!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via TLS loki client")
	}

	require.Eventually(
		t,
		func() bool {
			return len(handler.Received()) == 1
		},
		10*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received := handler.Received()[0]
	assert.Equal(t, received.Line, "hello world over TLS!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"source": "test",
		"foo":    "bar",
		"fizz":   "buzz",
	})
}

// newTestLokiClientTLS creates a Loki client configured for TLS connections
func newTestLokiClientTLS(t *testing.T, httpListenAddress string, opts component.Options) client.Consumer {
	url := flagext.URLValue{}
	err := url.Set(fmt.Sprintf(
		"https://%s/api/v1/push",
		httpListenAddress,
	))
	require.NoError(t, err)

	c, err := client.NewFanoutConsumer(opts.Logger, opts.Registerer, client.Config{
		URL:     url,
		Timeout: 10 * time.Second,
		Client: promCfg.HTTPClientConfig{
			TLSConfig: promCfg.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
	})

	require.NoError(t, err)
	return c
}

func TestDefaultServerConfig(t *testing.T) {
	args := testArgs()
	args.Server = nil // user did not define server options

	comp, err := New(
		defaultOptions(),
		args,
	)

	ctx := t.Context()
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		resp, err := http.Get(fmt.Sprintf(
			"http://%v:%d/wrong/url",
			"localhost",
			fnet.DefaultHTTPPort,
		))
		return err == nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func startTestComponent(
	t *testing.T,
	opts component.Options,
	args Arguments,
	ctx context.Context,
) *Component {

	comp, err := New(opts, args)
	require.NoError(t, err)
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	waitForServerToBeReady(t, comp)
	return comp
}

func TestShutdown(t *testing.T) {
	args := testArgsWith(func(a *Arguments) {
		a.Server.GracefulShutdownTimeout = 5 * time.Second
		a.ForwardTo = []loki.LogsReceiver{loki.NewLogsReceiver()}
	})

	opts := defaultOptions()

	comp, err := New(opts, args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	waitForServerToBeReady(t, comp)

	// First request should be forwarded on channel
	_, err = http.DefaultClient.Do(newRequest(t, context.Background(), comp.server.HTTPListenAddress()))
	require.NoError(t, err)

	codes := make(chan int)
	for range 5 {
		go func() {
			res, err := http.DefaultClient.Do(newRequest(t, context.Background(), comp.server.HTTPListenAddress()))
			if err != nil || res == nil {
				// This should not happen but if it does we return -1 here so test will fail.
				codes <- -1
			} else {
				codes <- res.StatusCode
			}
		}()
	}

	// Let requests go through.
	time.Sleep(2 * time.Second)

	// Cancel component and stop server.
	cancel()

	var collected []int
	for c := range codes {
		collected = append(collected, c)
		if len(collected) == 5 {
			break
		}
	}

	require.Equal(t, slices.Repeat([]int{503}, 5), collected)
}

func TestCancelRequest(t *testing.T) {
	args := testArgsWith(func(a *Arguments) {
		a.Server.GracefulShutdownTimeout = 5 * time.Second
		a.ForwardTo = []loki.LogsReceiver{loki.NewLogsReceiver()}
	})

	opts := defaultOptions()

	comp, err := New(opts, args)
	require.NoError(t, err)

	ctx := t.Context()
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	waitForServerToBeReady(t, comp)

	// First request should be forwarded on channel
	_, err = http.DefaultClient.Do(newRequest(t, context.Background(), comp.server.HTTPListenAddress()))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			res, err := http.DefaultClient.Do(newRequest(t, ctx, comp.server.HTTPListenAddress()))
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Nil(t, res)
		})
	}

	wg.Wait()
}

func newRequest(t *testing.T, ctx context.Context, httpListendAddress string) *http.Request {
	body := bytes.Buffer{}
	err := util.SerializeProto(&body, &push.PushRequest{Streams: []push.Stream{{Labels: `{foo="foo"}`, Entries: []push.Entry{{Line: "line"}}}}}, util.RawSnappy)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/loki/api/v1/push", httpListendAddress), &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	return req
}

func waitForServerToBeReady(t *testing.T, comp *Component) {
	// Determine if TLS is enabled to choose the right protocol
	protocol := "http"
	var tlsConfig *tls.Config

	serverConfig := comp.server.ServerConfig()
	if serverConfig.HTTP.TLSConfig != nil {
		protocol = "https"
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	url := fmt.Sprintf(
		"%s://%s/wrong/url",
		protocol,
		comp.server.HTTPListenAddress(),
	)

	client := &http.Client{Timeout: 1 * time.Second}
	if protocol == "https" {
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	require.Eventuallyf(t, func() bool {
		resp, err := client.Get(url)

		return err == nil && resp != nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func mapToChannels(clients []*loki.CollectingHandler) []loki.LogsReceiver {
	channels := make([]loki.LogsReceiver, len(clients))
	for i := range clients {
		channels[i] = clients[i].Receiver()
	}
	return channels
}

func newTestLokiClient(t *testing.T, args Arguments, opts component.Options) client.Consumer {
	url := flagext.URLValue{}
	err := url.Set(fmt.Sprintf(
		"http://%s:%d/api/v1/push",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	))
	require.NoError(t, err)

	lokiClient, err := client.NewFanoutConsumer(
		opts.Logger,
		opts.Registerer,
		client.Config{
			URL:     url,
			Timeout: 5 * time.Second,
			QueueConfig: client.QueueConfig{
				BlockOnOverflow: true,
			},
		},
	)
	require.NoError(t, err)
	return lokiClient
}

func defaultOptions() component.Options {
	return component.Options{
		ID:         "loki.source.api.test",
		Logger:     log.NewNopLogger(),
		Registerer: prometheus.NewRegistry(),
	}
}

func testArgs() Arguments {
	return testArgsWithPorts(0, 0)
}

func testArgsWith(mutator func(arguments *Arguments)) Arguments {
	a := testArgsWithPorts(0, 0)
	mutator(&a)
	return a
}

func testArgsWithPorts(httpPort int, grpcPort int) Arguments {
	return Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "127.0.0.1",
				ListenPort:    httpPort,
			},
			GRPC: &fnet.GRPCConfig{
				ListenAddress: "127.0.0.1",
				ListenPort:    grpcPort,
			},
		},
		ForwardTo: []loki.LogsReceiver{loki.NewLogsReceiver(), loki.NewLogsReceiver()},
		Labels:    map[string]string{"foo": "bar", "fizz": "buzz"},
		RelabelRules: relabel.Rules{
			{
				SourceLabels: []string{"tag"},
				Regex:        relabel.Regexp{Regexp: regexp.MustCompile("ignore")},
				Action:       relabel.Drop,
			},
		},
		UseIncomingTimestamp: false,
		MaxSendMessageSize:   100 * units.MiB,
	}
}
