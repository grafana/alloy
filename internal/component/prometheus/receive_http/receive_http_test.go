package receive_http

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
	"net/url"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/schema"
	"github.com/prometheus/prometheus/storage"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
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

func TestForwardsMetrics(t *testing.T) {
	timestamp := time.Now().Add(time.Second).UnixMilli()
	input := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 12},
			{Timestamp: timestamp + 1, Value: 24},
			{Timestamp: timestamp + 2, Value: 48},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 191},
			{Timestamp: timestamp + 1, Value: 1337},
		},
	}}

	expected := []testSample{
		{ts: timestamp, val: 12, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 1, val: 24, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 2, val: 48, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp, val: 191, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
		{ts: timestamp + 1, val: 1337, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	actualSamples := make(chan testSample, 100)

	// Start the component
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    port,
			},
			GRPC: testGRPCConfig(t),
		},
		AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
		ForwardTo:                           testAppendable(actualSamples),
	}

	comp, err := New(testOptions(t), args)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	verifyExpectations(t, input, expected, actualSamples, args, ctx)
}

func TestForwardsMetricsTLS(t *testing.T) {
	timestamp := time.Now().Add(time.Second).UnixMilli()
	input := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 12},
			{Timestamp: timestamp + 1, Value: 24},
			{Timestamp: timestamp + 2, Value: 48},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 191},
			{Timestamp: timestamp + 1, Value: 1337},
		},
	}}

	expected := []testSample{
		{ts: timestamp, val: 12, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 1, val: 24, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 2, val: 48, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp, val: 191, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
		{ts: timestamp + 1, val: 1337, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	actualSamples := make(chan testSample, 100)

	// Generate test certificate and key for TLS
	testCert, testKey, err := generateTestCertAndKey()
	require.NoError(t, err)

	// Start the component with TLS configuration
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    port,
				TLSConfig: &fnet.TLSConfig{
					Cert: testCert,
					Key:  alloytypes.Secret(testKey),
				},
			},
			GRPC: testGRPCConfig(t),
		},
		AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
		ForwardTo:                           testAppendable(actualSamples),
	}
	comp, err := New(testOptions(t), args)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	verifyExpectationsTLS(t, input, expected, actualSamples, args, ctx)
}

func verifyExpectationsTLS(
	t *testing.T,
	input []prompb.TimeSeries,
	expected []testSample,
	actualSamples chan testSample,
	args Arguments,
	ctx context.Context,
) {
	// In case server didn't start yet
	waitForServerToBeReady(t, args)

	// Send the input time series to the component using HTTPS
	endpoint := fmt.Sprintf(
		"https://%s:%d/api/v1/metrics/write",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	)
	err := requestTLS(ctx, endpoint, &prompb.WriteRequest{Timeseries: input})
	require.NoError(t, err)

	// Verify we receive expected metrics
	for _, exp := range expected {
		select {
		case actual := <-actualSamples:
			require.Equal(t, exp, actual)
		case <-ctx.Done():
			t.Fatalf("test timed out")
		}
	}

	select {
	case unexpected := <-actualSamples:
		t.Fatalf("unexpected extra sample received: %v", unexpected)
	default:
	}
}

func requestTLS(ctx context.Context, rawRemoteWriteURL string, req *prompb.WriteRequest) error {
	remoteWriteURL, err := url.Parse(rawRemoteWriteURL)
	if err != nil {
		return err
	}

	client, err := promremote.NewWriteClient("remote-write-client", &promremote.ClientConfig{
		URL:     &config.URL{URL: remoteWriteURL},
		Timeout: model.Duration(30 * time.Second),
		HTTPClientConfig: config.HTTPClientConfig{
			TLSConfig: config.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
	})
	if err != nil {
		return err
	}

	buf, err := proto.Marshal(protoadapt.MessageV2Of(req))
	if err != nil {
		return err
	}

	compressed := snappy.Encode(buf, buf)
	_, err = client.Store(ctx, compressed, 0)
	return err
}

func TestUpdate(t *testing.T) {
	timestamp := time.Now().Add(time.Second).UnixMilli()
	input01 := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 12},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 191},
		},
	}}
	expected01 := []testSample{
		{ts: timestamp, val: 12, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp, val: 191, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	input02 := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp + 1, Value: 24},
			{Timestamp: timestamp + 2, Value: 48},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp + 1, Value: 1337},
		},
	}}
	expected02 := []testSample{
		{ts: timestamp + 1, val: 24, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 2, val: 48, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 1, val: 1337, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	actualSamples := make(chan testSample, 100)

	// Start the component
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    port,
			},
			GRPC: testGRPCConfig(t),
		},
		AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
		ForwardTo:                           testAppendable(actualSamples),
	}
	comp, err := New(testOptions(t), args)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	verifyExpectations(t, input01, expected01, actualSamples, args, ctx)

	otherPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	args = Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    otherPort,
			},
			GRPC: testGRPCConfig(t),
		},
		ForwardTo: testAppendable(actualSamples),
	}
	err = comp.Update(args)
	require.NoError(t, err)

	verifyExpectations(t, input02, expected02, actualSamples, args, ctx)
}

func testGRPCConfig(t *testing.T) *fnet.GRPCConfig {
	return &fnet.GRPCConfig{ListenAddress: "127.0.0.1", ListenPort: getFreePort(t)}
}

func TestServerRestarts(t *testing.T) {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	otherPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	testCases := []struct {
		name          string
		initialArgs   Arguments
		newArgs       Arguments
		shouldRestart bool
	}{
		{
			name: "identical args require no restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           []storage.Appendable{},
			},
			shouldRestart: false,
		},
		{
			name: "forward_to update does not require restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           testAppendable(nil),
			},
			shouldRestart: false,
		},
		{
			name: "hostname change requires restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "127.0.0.1", ListenPort: port},
				},
				ForwardTo: testAppendable(nil),
			},
			shouldRestart: true,
		},
		{
			name: "port change requires restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: otherPort},
				},
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
				ForwardTo:                           testAppendable(nil),
			},
			shouldRestart: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			comp, err := New(testOptions(t), tc.initialArgs)
			require.NoError(t, err)

			serverExit := make(chan error)
			go func() {
				serverExit <- comp.Run(ctx)
			}()

			waitForServerToBeReady(t, comp.args)

			initialServer := comp.server
			require.NotNil(t, initialServer)

			err = comp.Update(tc.newArgs)
			require.NoError(t, err)

			waitForServerToBeReady(t, comp.args)

			require.NotNil(t, comp.server)
			restarted := initialServer != comp.server

			require.Equal(t, tc.shouldRestart, restarted)

			// Shut down cleanly to release ports for other tests
			cancel()
			select {
			case err := <-serverExit:
				if err != nil && err != context.Canceled {
					require.NoError(t, err, "unexpected error on server exit")
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for server to shut down")
			}
		})
	}
}

type testSample struct {
	ts  int64
	val float64
	l   labels.Labels
}

func waitForServerToBeReady(t *testing.T, args Arguments) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Determine if TLS is enabled to choose the right protocol
		protocol := "http"
		var tlsConfig *tls.Config

		if args.Server.HTTP.TLSConfig != nil {
			protocol = "https"
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}

		url := fmt.Sprintf(
			"%s://%v:%d/wrong/path",
			protocol,
			args.Server.HTTP.ListenAddress,
			args.Server.HTTP.ListenPort,
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

		t.Logf("err: %v, resp: %v", err, resp)
		assert.NoError(c, err)
		if resp != nil {
			assert.Equal(c, 404, resp.StatusCode)
		}
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func verifyExpectations(
	t *testing.T,
	input []prompb.TimeSeries,
	expected []testSample,
	actualSamples chan testSample,
	args Arguments,
	ctx context.Context,
) {
	// In case server didn't start yet
	waitForServerToBeReady(t, args)

	// Send the input time series to the component
	endpoint := fmt.Sprintf(
		"http://%s:%d/api/v1/metrics/write",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	)
	err := request(ctx, endpoint, &prompb.WriteRequest{Timeseries: input})
	require.NoError(t, err)

	// Verify we receive expected metrics
	for _, exp := range expected {
		select {
		case actual := <-actualSamples:
			require.Equal(t, exp, actual)
		case <-ctx.Done():
			t.Fatalf("test timed out")
		}
	}

	select {
	case unexpected := <-actualSamples:
		t.Fatalf("unexpected extra sample received: %v", unexpected)
	default:
	}
}

func testAppendable(actualSamples chan testSample) []storage.Appendable {
	hookFn := func(
		ref storage.SeriesRef,
		l labels.Labels,
		ts int64,
		val float64,
		next storage.Appender,
	) (storage.SeriesRef, error) {

		actualSamples <- testSample{ts: ts, val: val, l: l}
		return ref, nil
	}

	return []storage.Appendable{alloyprom.NewInterceptor(
		nil,
		alloyprom.WithAppendHook(
			hookFn))}
}

func request(ctx context.Context, rawRemoteWriteURL string, req *prompb.WriteRequest) error {
	remoteWriteURL, err := url.Parse(rawRemoteWriteURL)
	if err != nil {
		return err
	}

	client, err := promremote.NewWriteClient("remote-write-client", &promremote.ClientConfig{
		URL:     &config.URL{URL: remoteWriteURL},
		Timeout: model.Duration(30 * time.Second),
	})
	if err != nil {
		return err
	}

	buf, err := proto.Marshal(protoadapt.MessageV2Of(req))
	if err != nil {
		return err
	}

	compressed := snappy.Encode(buf, buf)
	_, err = client.Store(ctx, compressed, 0)
	return err
}

func requestV2(ctx context.Context, rawRemoteWriteURL string, req *writev2.Request) error {
	remoteWriteURL, err := url.Parse(rawRemoteWriteURL)
	if err != nil {
		return err
	}

	client, err := promremote.NewWriteClient("remote-write-client-v2", &promremote.ClientConfig{
		URL:           &config.URL{URL: remoteWriteURL},
		Timeout:       model.Duration(30 * time.Second),
		WriteProtoMsg: remote.WriteV2MessageType,
	})
	if err != nil {
		return err
	}

	buf, err := req.Marshal()
	if err != nil {
		return err
	}

	compressed := snappy.Encode(nil, buf)
	_, err = client.Store(ctx, compressed, 0)
	return err
}

func testOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "prometheus.receive_http.test",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		GetServiceData: func(name string) (any, error) {
			return labelstore.New(nil, prometheus.DefaultRegisterer), nil
		},
	}
}

func getFreePort(t *testing.T) int {
	p, err := freeport.GetFreePort()
	require.NoError(t, err)
	return p
}

func TestRemoteWriteV2(t *testing.T) {
	timestamp := time.Now().Add(time.Second).UnixMilli()

	// Create Remote Write v2 request with symbol table
	symbols := []string{
		"",                              // 0: empty string (required)
		"__name__",                      // 1
		"http_requests_total",           // 2
		"method",                        // 3
		"GET",                           // 4
		"status",                        // 5
		"200",                           // 6
		"cpu_usage_seconds",             // 7
		"cpu",                           // 8
		"0",                             // 9
		"Total number of HTTP requests", // 10
		"requests",                      // 11
		"CPU usage in seconds",          // 12
		"seconds",                       // 13
	}

	input := &writev2.Request{
		Symbols: symbols,
		Timeseries: []writev2.TimeSeries{
			{
				// http_requests_total{method="GET", status="200"}
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6}, // __name__=http_requests_total, method=GET, status=200
				Samples: []writev2.Sample{
					{Timestamp: timestamp, Value: 100},
					{Timestamp: timestamp + 1, Value: 150},
				},
				Metadata: writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
					HelpRef: 10, // "Total number of HTTP requests"
					UnitRef: 11, // "requests"
				},
			},
			{
				// cpu_usage_seconds{cpu="0"}
				LabelsRefs: []uint32{1, 7, 8, 9}, // __name__=cpu_usage_seconds, cpu=0
				Samples: []writev2.Sample{
					{Timestamp: timestamp, Value: 45.2},
				},
				Metadata: writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
					HelpRef: 12, // "CPU usage in seconds"
					UnitRef: 13, // "seconds"
				},
			},
		},
	}

	testCases := []struct {
		name                    string
		appendMetadata          bool
		enableTypeAndUnitLabels bool
		expectedSamples         []testSample
		expectedMetadata        map[string]testMetadata
	}{
		{
			name:                    "with metadata forwarding",
			appendMetadata:          true,
			enableTypeAndUnitLabels: false,
			expectedSamples: []testSample{
				{ts: timestamp, val: 100, l: labels.FromStrings("__name__", "http_requests_total", "method", "GET", "status", "200")},
				{ts: timestamp + 1, val: 150, l: labels.FromStrings("__name__", "http_requests_total", "method", "GET", "status", "200")},
				{ts: timestamp, val: 45.2, l: labels.FromStrings("__name__", "cpu_usage_seconds", "cpu", "0")},
			},
			expectedMetadata: map[string]testMetadata{
				"http_requests_total": {
					metricName: "http_requests_total",
					metricType: "counter",
					help:       "Total number of HTTP requests",
					unit:       "requests",
				},
				"cpu_usage_seconds": {
					metricName: "cpu_usage_seconds",
					metricType: "gauge",
					help:       "CPU usage in seconds",
					unit:       "seconds",
				},
			},
		},
		{
			name:                    "with type and unit labels",
			appendMetadata:          false,
			enableTypeAndUnitLabels: true,
			expectedSamples: []testSample{
				{ts: timestamp, val: 100, l: labels.FromStrings("__name__", "http_requests_total", "__type__", "counter", "__unit__", "requests", "method", "GET", "status", "200")},
				{ts: timestamp + 1, val: 150, l: labels.FromStrings("__name__", "http_requests_total", "__type__", "counter", "__unit__", "requests", "method", "GET", "status", "200")},
				{ts: timestamp, val: 45.2, l: labels.FromStrings("__name__", "cpu_usage_seconds", "__type__", "gauge", "__unit__", "seconds", "cpu", "0")},
			},
			expectedMetadata: nil, // No metadata should be forwarded
		},
		{
			name:                    "with both metadata and type/unit labels",
			appendMetadata:          true,
			enableTypeAndUnitLabels: true,
			expectedSamples: []testSample{
				{ts: timestamp, val: 100, l: labels.FromStrings("__name__", "http_requests_total", "__type__", "counter", "__unit__", "requests", "method", "GET", "status", "200")},
				{ts: timestamp + 1, val: 150, l: labels.FromStrings("__name__", "http_requests_total", "__type__", "counter", "__unit__", "requests", "method", "GET", "status", "200")},
				{ts: timestamp, val: 45.2, l: labels.FromStrings("__name__", "cpu_usage_seconds", "__type__", "gauge", "__unit__", "seconds", "cpu", "0")},
			},
			expectedMetadata: map[string]testMetadata{
				"http_requests_total": {
					metricName: "http_requests_total",
					metricType: "counter",
					help:       "Total number of HTTP requests",
					unit:       "requests",
				},
				"cpu_usage_seconds": {
					metricName: "cpu_usage_seconds",
					metricType: "gauge",
					help:       "CPU usage in seconds",
					unit:       "seconds",
				},
			},
		},
		{
			name:                    "without metadata or type/unit labels",
			appendMetadata:          false,
			enableTypeAndUnitLabels: false,
			expectedSamples: []testSample{
				{ts: timestamp, val: 100, l: labels.FromStrings("__name__", "http_requests_total", "method", "GET", "status", "200")},
				{ts: timestamp + 1, val: 150, l: labels.FromStrings("__name__", "http_requests_total", "method", "GET", "status", "200")},
				{ts: timestamp, val: 45.2, l: labels.FromStrings("__name__", "cpu_usage_seconds", "cpu", "0")},
			},
			expectedMetadata: nil, // No metadata should be forwarded
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualSamples := make(chan testSample, 100)
			actualMetadata := make(chan testMetadata, 10)

			// Start the component
			port, err := freeport.GetFreePort()
			require.NoError(t, err)
			args := Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{
						ListenAddress: "localhost",
						ListenPort:    port,
					},
					GRPC: testGRPCConfig(t),
				},
				ForwardTo:                           testAppendableWithMetadata(actualSamples, actualMetadata),
				AppendMetadata:                      tc.appendMetadata,
				EnableTypeAndUnitLabels:             tc.enableTypeAndUnitLabels,
				AcceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType), string(remote.WriteV2MessageType)},
			}
			comp, err := New(testOptions(t), args)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			go func() {
				require.NoError(t, comp.Run(ctx))
			}()

			// Wait for server to be ready
			waitForServerToBeReady(t, args)

			// Send the remote write v2 request
			endpoint := fmt.Sprintf(
				"http://%s:%d/api/v1/metrics/write",
				args.Server.HTTP.ListenAddress,
				args.Server.HTTP.ListenPort,
			)
			err = requestV2(ctx, endpoint, input)
			require.NoError(t, err)

			// Verify samples were received
			for _, exp := range tc.expectedSamples {
				select {
				case actual := <-actualSamples:
					require.Equal(t, exp, actual)
				case <-ctx.Done():
					t.Fatalf("test timed out waiting for samples")
				}
			}

			// Verify metadata if expected
			if tc.expectedMetadata != nil {
				receivedMetadata := make(map[string]testMetadata)
				for i := 0; i < len(tc.expectedMetadata); i++ {
					select {
					case m := <-actualMetadata:
						receivedMetadata[m.metricName] = m
					case <-time.After(2 * time.Second):
						t.Fatalf("timeout waiting for metadata, received %d of %d", i, len(tc.expectedMetadata))
					}
				}
				require.Equal(t, tc.expectedMetadata, receivedMetadata)
			}

			// Ensure no unexpected samples
			select {
			case unexpected := <-actualSamples:
				t.Fatalf("unexpected extra sample received: %v", unexpected)
			default:
			}

			// Ensure no unexpected metadata
			select {
			case unexpected := <-actualMetadata:
				if tc.expectedMetadata == nil {
					t.Fatalf("unexpected metadata received when append_metadata=false: %v", unexpected)
				} else {
					t.Fatalf("unexpected extra metadata received: %v", unexpected)
				}
			default:
			}
		})
	}
}

type testMetadata struct {
	metricName string
	metricType string
	help       string
	unit       string
}

func testAppendableWithMetadata(actualSamples chan testSample, actualMetadata chan testMetadata) []storage.Appendable {
	appendHook := func(
		ref storage.SeriesRef,
		l labels.Labels,
		ts int64,
		val float64,
		next storage.Appender,
	) (storage.SeriesRef, error) {

		actualSamples <- testSample{ts: ts, val: val, l: l}
		return ref, nil
	}

	metadataHook := func(
		ref storage.SeriesRef,
		l labels.Labels,
		m metadata.Metadata,
		next storage.Appender,
	) (storage.SeriesRef, error) {

		metricName := schema.NewMetadataFromLabels(l).Name
		actualMetadata <- testMetadata{
			metricName: metricName,
			metricType: string(m.Type),
			help:       m.Help,
			unit:       m.Unit,
		}
		return ref, nil
	}

	return []storage.Appendable{alloyprom.NewInterceptor(
		nil,
		alloyprom.WithAppendHook(appendHook),
		alloyprom.WithMetadataHook(metadataHook),
	)}
}

func TestAcceptedRemoteWriteProtobufMessages(t *testing.T) {
	testCases := []struct {
		name                                string
		acceptedRemoteWriteProtobufMessages []string
		useExperimentalStability            bool
		expectError                         bool
		expectedErrorContains               string
	}{
		{
			name:                                "invalid protocol version",
			acceptedRemoteWriteProtobufMessages: []string{"invalid.protocol"},
			useExperimentalStability:            false,
			expectError:                         true,
			expectedErrorContains:               "unsupported protocol version",
		},
		{
			name:                                "v2 without experimental stability",
			acceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV2MessageType)},
			useExperimentalStability:            false,
			expectError:                         true,
			expectedErrorContains:               "experimental feature",
		},
		{
			name:                                "v2 with experimental stability",
			acceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV2MessageType)},
			useExperimentalStability:            true,
			expectError:                         false,
		},
		{
			name:                                "v1 only (default configuration)",
			acceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType)},
			useExperimentalStability:            false,
			expectError:                         false,
		},
		{
			name:                                "empty list",
			acceptedRemoteWriteProtobufMessages: []string{},
			useExperimentalStability:            false,
			expectError:                         true,
			expectedErrorContains:               "must not be empty",
		},
		{
			name:                                "duplicate message types",
			acceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType), string(remote.WriteV1MessageType)},
			useExperimentalStability:            false,
			expectError:                         false,
		},
		{
			name:                                "both v1 and v2 with experimental",
			acceptedRemoteWriteProtobufMessages: []string{string(remote.WriteV1MessageType), string(remote.WriteV2MessageType)},
			useExperimentalStability:            true,
			expectError:                         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port, err := freeport.GetFreePort()
			require.NoError(t, err)

			args := Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{
						ListenAddress: "localhost",
						ListenPort:    port,
					},
					GRPC: testGRPCConfig(t),
				},
				ForwardTo:                           []storage.Appendable{alloyprom.NewFanout(nil, "", prometheus.NewRegistry(), nil)},
				AcceptedRemoteWriteProtobufMessages: tc.acceptedRemoteWriteProtobufMessages,
			}

			opts := testOptions(t)
			opts.MinStability = featuregate.StabilityGenerallyAvailable
			if tc.useExperimentalStability {
				opts.MinStability = featuregate.StabilityExperimental
			}

			comp, err := New(opts, args)

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedErrorContains != "" {
					require.Contains(t, err.Error(), tc.expectedErrorContains)
				}
				require.Nil(t, comp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, comp)
			}
		})
	}
}
