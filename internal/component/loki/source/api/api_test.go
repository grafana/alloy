package api

import (
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
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/phayes/freeport"

	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	promCfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/common/relabel"
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

	receiver := fake.NewClient(func() {})
	defer receiver.Stop()

	args := testArgsWith(t, func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8532
		a.ForwardTo = []loki.LogsReceiver{receiver.LogsReceiver()}
		a.UseIncomingTimestamp = true
	})
	opts := defaultOptions()
	_, shutdown := startTestComponent(t, opts, args, ctx)
	defer shutdown()

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
		"source": "test",
		"foo":    "bar",
		"fizz":   "buzz",
	})
}

func TestLokiSourceAPI_Update(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	receiver := fake.NewClient(func() {})
	defer receiver.Stop()

	args := testArgsWith(t, func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8583
		a.ForwardTo = []loki.LogsReceiver{receiver.LogsReceiver()}
		a.UseIncomingTimestamp = true
		a.Labels = map[string]string{"test_label": "before"}
	})
	opts := defaultOptions()
	c, shutdown := startTestComponent(t, opts, args, ctx)
	defer shutdown()

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
	var receivers = make([]*fake.Client, receiversCount)
	for i := 0; i < receiversCount; i++ {
		receivers[i] = fake.NewClient(func() {})
	}

	args := testArgsWith(t, func(a *Arguments) {
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

	defer comp.stop()

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	const messagesCount = 100
	for i := 0; i < messagesCount; i++ {
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
			for i := 0; i < receiversCount; i++ {
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
	httpPort := getFreePort(t)
	grpcPort := getFreePort(t, httpPort)
	tests := []struct {
		name            string
		args            Arguments
		newArgs         Arguments
		restartRequired bool
	}{
		{
			name:            "identical args don't require server restart",
			args:            testArgsWithPorts(httpPort, grpcPort),
			newArgs:         testArgsWithPorts(httpPort, grpcPort),
			restartRequired: false,
		},
		{
			name: "change in address requires server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.Server.HTTP.ListenAddress = "localhost"
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: true,
		},
		{
			name:            "change in port requires server restart",
			args:            testArgsWithPorts(httpPort, grpcPort),
			newArgs:         testArgsWithPorts(getFreePort(t, httpPort, grpcPort), grpcPort),
			restartRequired: true,
		},
		{
			name: "change in forwardTo does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.ForwardTo = []loki.LogsReceiver{}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
		{
			name: "change in labels does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.Labels = map[string]string{"some": "label"}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
		{
			name: "change in relabel rules does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.RelabelRules = relabel.Rules{}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
		{
			name: "change in use incoming timestamp does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.UseIncomingTimestamp = !args.UseIncomingTimestamp
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comp, err := New(
				defaultOptions(),
				tc.args,
			)
			require.NoError(t, err)

			// in order to cleanly update, we want to make sure the server is running first.
			waitForServerToBeReady(t, comp)

			serverBefore := comp.server
			err = comp.Update(tc.newArgs)
			require.NoError(t, err)

			restarted := serverBefore != comp.server
			assert.Equal(t, restarted, tc.restartRequired)

			// in order to cleanly shutdown, we want to make sure the server is running first.
			waitForServerToBeReady(t, comp)
			comp.stop()
		})
	}
}

func TestLokiSourceAPI_TLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Generate test certificate and key for TLS
	testCert, testKey, err := generateTestCertAndKey()
	require.NoError(t, err)

	receiver := fake.NewClient(func() {})
	defer receiver.Stop()

	args := testArgsWith(t, func(a *Arguments) {
		a.Server.HTTP.ListenPort = getFreePort(t)
		a.Server.HTTP.TLSConfig = &fnet.TLSConfig{
			Cert: testCert,
			Key:  alloytypes.Secret(testKey),
		}
		a.ForwardTo = []loki.LogsReceiver{receiver.LogsReceiver()}
		a.UseIncomingTimestamp = true
	})
	opts := defaultOptions()
	_, shutdown := startTestComponent(t, opts, args, ctx)
	defer shutdown()

	// Create TLS-enabled Loki client
	lokiClient := newTestLokiClientTLS(t, args, opts)
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
		func() bool { return len(receiver.Received()) == 1 },
		10*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received := receiver.Received()[0]
	assert.Equal(t, received.Line, "hello world over TLS!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"source": "test",
		"foo":    "bar",
		"fizz":   "buzz",
	})
}

// newTestLokiClientTLS creates a Loki client configured for TLS connections
func newTestLokiClientTLS(t *testing.T, args Arguments, opts component.Options) client.Client {
	url := flagext.URLValue{}
	err := url.Set(fmt.Sprintf(
		"https://%s:%d/api/v1/push",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	))
	require.NoError(t, err)

	lokiClient, err := client.New(
		client.NewMetrics(nil),
		client.Config{
			URL:     url,
			Timeout: 10 * time.Second,
			Client: promCfg.HTTPClientConfig{
				TLSConfig: promCfg.TLSConfig{
					InsecureSkipVerify: true,
				},
			},
		},
		0,
		opts.Logger,
	)
	require.NoError(t, err)
	return lokiClient
}

func TestDefaultServerConfig(t *testing.T) {
	args := testArgs(t)
	args.Server = nil // user did not define server options

	comp, err := New(
		defaultOptions(),
		args,
	)
	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		resp, err := http.Get(fmt.Sprintf(
			"http://%v:%d/wrong/url",
			"localhost",
			fnet.DefaultHTTPPort,
		))
		return err == nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")

	comp.stop()
}

func startTestComponent(
	t *testing.T,
	opts component.Options,
	args Arguments,
	ctx context.Context,
) (component.Component, func()) {

	comp, err := New(opts, args)
	require.NoError(t, err)
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	return comp, func() {
		// in order to cleanly shutdown, we want to make sure the server is running first.
		waitForServerToBeReady(t, comp)
		comp.stop()
	}
}

func waitForServerToBeReady(t *testing.T, comp *Component) {
	require.Eventuallyf(t, func() bool {
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
			"%s://%v:%d/wrong/url",
			protocol,
			serverConfig.HTTP.ListenAddress,
			serverConfig.HTTP.ListenPort,
		)

		var resp *http.Response
		var err error

		if protocol == "https" {
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: tlsConfig,
				},
				Timeout: 1 * time.Second,
			}
			resp, err = client.Get(url)
		} else {
			client := &http.Client{Timeout: 1 * time.Second}
			resp, err = client.Get(url)
		}

		return err == nil && resp != nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func mapToChannels(clients []*fake.Client) []loki.LogsReceiver {
	channels := make([]loki.LogsReceiver, len(clients))
	for i := range clients {
		channels[i] = clients[i].LogsReceiver()
	}
	return channels
}

func newTestLokiClient(t *testing.T, args Arguments, opts component.Options) client.Client {
	url := flagext.URLValue{}
	err := url.Set(fmt.Sprintf(
		"http://%s:%d/api/v1/push",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	))
	require.NoError(t, err)

	lokiClient, err := client.New(
		client.NewMetrics(nil),
		client.Config{
			URL:     url,
			Timeout: 5 * time.Second,
		},
		0,
		opts.Logger,
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

func testArgsWith(t *testing.T, mutator func(arguments *Arguments)) Arguments {
	a := testArgs(t)
	mutator(&a)
	return a
}

func testArgs(t *testing.T) Arguments {
	httpPort := getFreePort(t)
	grpPort := getFreePort(t, httpPort)
	return testArgsWithPorts(httpPort, grpPort)
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

func getFreePort(t *testing.T, exclude ...int) int {
	const maxRetries = 10
	for range maxRetries {
		port, err := freeport.GetFreePort()
		require.NoError(t, err)
		if !slices.Contains(exclude, port) {
			return port
		}
	}

	t.Fatal("fail to get free port")
	return 0
}
