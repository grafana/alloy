package cluster_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
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

type testState struct {
	peerAddresses []string
	ctx           context.Context
	shutdownGroup sync.WaitGroup
}

// TODO(thampiotr): scenarios to cover:
// TODO(thampiotr): nodes leave clean - init 8, remove 4
// TODO(thampiotr): nodes die - init 8, kill 4
// TODO(thampiotr): name conflicts
// TODO(thampiotr): split brain and then merge - init 4 + 4 separated, remove separation
// TODO(thampiotr): network isolation -> all create own? - init 4 all separated, remove separation
// TODO(thampiotr): when component evaluations are super slow?
// TODO(thampiotr): when updating component's NotifyClusterChange is taking too long
func TestClusterE2E(t *testing.T) {
	type testCase struct {
		name              string
		nodeCountInitial  int
		assertionsInitial func(t *assert.CollectT, state *testState)
		changes           func(state *testState)
		assertionsFinal   func(t *assert.CollectT, state *testState)
		assertionsTimeout time.Duration
	}

	tests := []testCase{
		{
			name:             "three nodes just join",
			nodeCountInitial: 3,
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, address := range state.peerAddresses {
					metricsContain(t, address, `cluster_node_info{state="participant"} 1`)
					metricsContain(t, address, `cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 3`)
					metricsContain(t, address, `cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 3`)
				}
			},
		},
		{
			name:             "4 nodes are joined by another 4 nodes",
			nodeCountInitial: 4,
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, address := range state.peerAddresses {
					metricsContain(t, address, `cluster_node_info{state="participant"} 1`)
					metricsContain(t, address, `cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`)
					metricsContain(t, address, `cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`)
				}
			},
			changes: func(state *testState) {
				for i := 0; i < 4; i++ {
					startNewNode(t, state, fmt.Sprintf("new-node-%d", i))
				}
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, address := range state.peerAddresses {
					metricsContain(t, address, `cluster_node_info{state="participant"} 1`)
					metricsContain(t, address, `cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 8`)
					metricsContain(t, address, `cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 8`)
				}
			},
		},
	}

	setDefaults := func(tc *testCase) {
		if tc.assertionsTimeout == 0 {
			tc.assertionsTimeout = 20 * time.Second
		}
		if tc.assertionsInitial == nil {
			tc.assertionsInitial = func(t *assert.CollectT, state *testState) {}
		}
		if tc.changes == nil {
			tc.changes = func(state *testState) {}
		}
		if tc.assertionsFinal == nil {
			tc.assertionsFinal = func(t *assert.CollectT, state *testState) {}
		}
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setDefaults(&tc)
			ctx, cancel := context.WithTimeout(context.Background(), tc.assertionsTimeout*2+5*time.Second)
			defer cancel()

			state := &testState{
				ctx: ctx,
			}

			for i := 0; i < tc.nodeCountInitial; i++ {
				startNewNode(t, state, fmt.Sprintf("node-%d", i))
			}

			assert.EventuallyWithT(t, func(t *assert.CollectT) {
				tc.assertionsInitial(t, state)
			}, 20*time.Second, 200*time.Millisecond)

			tc.changes(state)

			assert.EventuallyWithT(t, func(t *assert.CollectT) {
				tc.assertionsFinal(t, state)
			}, 20*time.Second, 200*time.Millisecond)

			cancel()
			state.shutdownGroup.Wait()
		})
	}
}

// startNewNode creates a new node with the given name, generates a new address for it,
// and appends that address to the state's peerAddresses list
func startNewNode(t *testing.T, state *testState, nodeName string) {
	nodeAddress := fmt.Sprintf("127.0.0.1:%d", getFreePort(t))
	state.peerAddresses = append(state.peerAddresses, nodeAddress)

	nodeDirPath := path.Join(t.TempDir(), nodeName)
	logger, err := logging.New(
		&logsWriter{
			t:      t,
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
		AdvertiseAddress:    nodeAddress,
		ListenAddress:       nodeAddress,
		JoinPeers:           state.peerAddresses,
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

		HTTPListenAddr: nodeAddress,
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

	// Increment the WaitGroup before starting the node
	state.shutdownGroup.Add(1)
	go func() {
		// Ensure the WaitGroup is decremented when the node finishes running
		defer state.shutdownGroup.Done()
		f.Run(state.ctx)
	}()

	src, err := runtime.ParseSource(t.Name(), []byte(""))
	require.NoError(t, err)
	err = f.LoadSource(src, nil, configFilePath)
	require.NoError(t, err)

	err = clusterService.ChangeState(state.ctx, peer.StateParticipant)
	require.NoError(t, err)
}

// metricsContain fetches metrics from the given URL and checks if they
// contain a line with the specified metric name
func metricsContain(t *assert.CollectT, nodeAddress string, text string) {
	metricsURL := fmt.Sprintf("http://%s/metrics", nodeAddress)
	body, err := fetchMetrics(metricsURL)
	require.NoError(t, err)
	require.Contains(t, body, text)
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

type logsWriter struct {
	t      *testing.T
	out    io.Writer
	prefix string
}

var errorsAllowlist = []string{
	"failed to extract directory path from configPath",             // unrelated to this test
	"failed to broadcast leave message to cluster",                 // on shutdown sometimes we can't push to nodes that already shut
	"failed to connect to peers; bootstrapping a new cluster",      // should be allowed only once for first node
	"over TCP but UDP probes failed, network may be misconfigured", // TODO: we should investigate and fix this if not an issue
}

func (w *logsWriter) Write(p []byte) (n int, err error) {
	prefixed := []byte(w.prefix)
	prefixed = append(prefixed, p...)

	// Check for warnings or errors in the log message
	logMsg := string(p)
	if strings.Contains(logMsg, "level=warn") || strings.Contains(logMsg, "level=error") {
		isAllowed := false
		for _, allowedErr := range errorsAllowlist {
			if strings.Contains(logMsg, allowedErr) {
				isAllowed = true
				break
			}
		}
		assert.True(w.t, isAllowed, "Disallowed warning or error found in logs: %s", logMsg)
	}

	_, err = w.out.Write(prefixed)
	return len(p), err
}
