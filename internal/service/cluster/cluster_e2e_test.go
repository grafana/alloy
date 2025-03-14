package cluster_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/grafana/ckit/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/alloycli"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
	httpservice "github.com/grafana/alloy/internal/service/http"
	remotecfgservice "github.com/grafana/alloy/internal/service/remotecfg"
)

// TODO(thampiotr): to add:
// TODO(thampiotr): check for error and warning messages that are not allow-listed

// TODO(thampiotr): scenarios to cover:
// TODO(thampiotr): nodes join - init 4, add 4
// TODO(thampiotr): nodes leave clean - init 8, remove 4
// TODO(thampiotr): nodes die - init 8, kill 4
// TODO(thampiotr): split brain and then merge - init 4 + 4 separated, remove separation
// TODO(thampiotr): network isolation -> all create own? - init 4 all separated, remove separation
// TODO(thampiotr): when component evaluations are super slow?
// TODO(thampiotr): when updating component's NotifyClusterChange is taking too long
func TestClusterE2E(t *testing.T) {
	tests := []struct {
		name       string
		nodeCount  int
		assertions func(t *assert.CollectT, peerAddresses []string)
	}{
		{
			name:      "three nodes",
			nodeCount: 3,
			assertions: func(t *assert.CollectT, peerAddresses []string) {
				for _, address := range peerAddresses {
					metricsContain(t, address, `cluster_node_info{state="participant"} 1`)
					metricsContain(t, address, `cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 3`)
					metricsContain(t, address, `cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 3`)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var peerAddresses []string
			wg := sync.WaitGroup{}

			for i := 0; i < tc.nodeCount; i++ {
				nodeAddress := fmt.Sprintf("127.0.0.1:%d", getFreePort(t))
				peerAddresses = append(peerAddresses, nodeAddress)
				startNewNode(t, ctx, fmt.Sprintf("node-%d", i), nodeAddress, peerAddresses, &wg)
			}

			// Check that all nodes have the specified metric
			if len(peerAddresses) > 0 {
				assert.EventuallyWithT(t, func(t *assert.CollectT) {
					tc.assertions(t, peerAddresses)
				}, 20*time.Second, 500*time.Millisecond)
			}
		})
	}
}

// metricsContain fetches metrics from the given URL and checks if they
// contain a line with the specified metric name
func metricsContain(t *assert.CollectT, nodeAddress string, metricName string) {
	metricsURL := fmt.Sprintf("http://%s/metrics", nodeAddress)
	body, err := fetchMetrics(metricsURL)
	require.NoError(t, err)
	require.Contains(t, body, metricName)
}

// fetchMetrics performs an HTTP GET request to the metrics endpoint and returns the response body
func fetchMetrics(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("GET request failed: %w", err)
	}
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

func startNewNode(t *testing.T, ctx context.Context, nodeName string, address string, peerAddresses []string, wg *sync.WaitGroup) {
	nodeDirPath := path.Join(t.TempDir(), nodeName)
	logger, err := logging.New(
		&prefixWriter{
			out:    os.Stdout,
			prefix: fmt.Sprintf("%s: ", nodeName),
		},
		logging.Options{
			Level:  logging.LevelDebug,
			Format: logging.FormatDefault,
		},
	)
	require.NoError(t, err)

	tracer, err := tracing.New(tracing.DefaultOptions)
	require.NoError(t, err)

	reg := prometheus.NewRegistry()
	clusterService, err := alloycli.BuildClusterService(alloycli.ClusterOptions{
		Log:     log.With(logger, "service", "cluster"),
		Tracer:  tracer,
		Metrics: reg,

		EnableClustering:    true,
		NodeName:            nodeName,
		AdvertiseAddress:    address,
		ListenAddress:       address,
		JoinPeers:           peerAddresses,
		RejoinInterval:      5 * time.Second,
		AdvertiseInterfaces: advertise.DefaultInterfaces,
		ClusterMaxJoinPeers: 5,
		ClusterName:         "cluster_e2e_test",
	})
	require.NoError(t, err)

	httpService := httpservice.New(httpservice.Options{
		Logger:   logger,
		Tracer:   tracer,
		Gatherer: reg,

		ReadyFunc:  func() bool { return true },
		ReloadFunc: func() (*runtime.Source, error) { return nil, nil },

		HTTPListenAddr: address,
	})

	configFilePath := path.Join(nodeDirPath, "config.alloy")
	remoteCfgService, err := remotecfgservice.New(remotecfgservice.Options{
		Logger:      log.With(logger, "service", "remotecfg"),
		ConfigPath:  configFilePath,
		StoragePath: nodeDirPath,
		Metrics:     reg,
	})
	require.NoError(t, err)

	f := runtime.New(runtime.Options{
		Logger:               logger,
		Tracer:               tracer,
		DataPath:             nodeDirPath,
		Reg:                  reg,
		MinStability:         featuregate.StabilityExperimental,
		EnableCommunityComps: false,
		Services: []service.Service{
			clusterService,
			httpService,
			remoteCfgService,
		},
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		f.Run(ctx)
	}()

	src, err := runtime.ParseSource(t.Name(), []byte(""))
	require.NoError(t, err)
	err = f.LoadSource(src, nil, configFilePath)
	require.NoError(t, err)

	err = clusterService.ChangeState(ctx, peer.StateParticipant)
	require.NoError(t, err)
}

// getFreePort returns a free port that can be used for testing
func getFreePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to find free port")

	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	err = listener.Close()
	require.NoError(t, err, "Failed to close listener")

	return port
}

type prefixWriter struct {
	out    io.Writer
	prefix string
}

func (w *prefixWriter) Write(p []byte) (n int, err error) {
	prefixed := []byte(w.prefix)
	prefixed = append(prefixed, p...)
	_, err = w.out.Write(prefixed)
	return len(p), err
}
