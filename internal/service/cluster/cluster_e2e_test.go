package cluster_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/alloycli"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/cluster/discovery"
	_ "github.com/grafana/alloy/internal/service/cluster/internal/testcomponent"
	httpservice "github.com/grafana/alloy/internal/service/http"
	remotecfgservice "github.com/grafana/alloy/internal/service/remotecfg"
)

type testCase struct {
	name                   string
	nodeCountInitial       int
	initialIsolatedNodes   []string // list of node names that are initially in isolated network space
	assertionsInitial      func(t *assert.CollectT, state *testState)
	changes                func(state *testState)
	assertionsFinal        func(t *assert.CollectT, state *testState)
	assertionsTimeout      time.Duration
	extraAllowedErrors     []string
	alloyConfig            string
	minimumClusterSize     int
	minimumSizeWaitTimeout time.Duration
}

func TestClusterE2E(t *testing.T) {
	tests := []testCase{
		{
			name:             "three nodes just join",
			alloyConfig:      `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial: 3,
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 3`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 3`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2",ready="true"} 3`,
					)
					verifyPeers(t, p, 3)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:             "three nodes with slow components",
			nodeCountInitial: 3,
			alloyConfig: `
   		testcomponents.cluster_state_tracker "foo" {}

		testcomponents.ticker "tick" {
			period = "500ms"
			max_value = 30
		}
		
		testcomponents.slow_update "test" {
			counter = testcomponents.ticker.tick.counter
			update_lag = "5s"
		}
		`,
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 3`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 3`,
						`ticker_counter{component_id="testcomponents.ticker.tick",component_path="/"} 30`,
						`slow_update_counter{component_id="testcomponents.slow_update.test",component_path="/"} 30`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2",ready="true"} 3`,
					)
					verifyPeers(t, p, 3)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:             "4 nodes are joined by another 4 nodes",
			alloyConfig:      `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial: 4,
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="true"} 4`,
					)
					verifyPeers(t, p, 4)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
			changes: func(state *testState) {
				for i := range 4 {
					startNewNode(t, state, fmt.Sprintf("new-node-%d", i))
				}
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 8`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 8`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="new-node-0___new-node-1___new-node-2___new-node-3___node-0___node-1___node-2___node-3",ready="true"} 8`,
					)
					verifyPeers(t, p, 8)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:             "8 node cluster - 4 nodes leave",
			alloyConfig:      `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial: 8,
			extraAllowedErrors: []string{
				"failed to rejoin list of peers",
				"failed to broadcast leave message to cluster",
				"timeout waiting for leave broadcast",
			},
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 8`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 8`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3___node-4___node-5___node-6___node-7",ready="true"} 8`,
					)
					verifyPeers(t, p, 8)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
			changes: func(state *testState) {
				for i := range 4 {
					state.peers[i].shutdown()
				}
				state.peers = state.peers[4:]
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-4___node-5___node-6___node-7",ready="true"} 4`,
					)
					verifyPeers(t, p, 4)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:             "4 nodes are joined by a node with a name conflict",
			alloyConfig:      `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial: 4,
			extraAllowedErrors: []string{
				`Conflicting address for node-0`,
				`Conflicting address for node-1`,
			},
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="true"} 4`,
					)
					verifyPeers(t, p, 4)
					verifyClusterReady(t, p)
				}
				verifyLookupInvariants(t, state.peers)
			},
			changes: func(state *testState) {
				// These names conflict with existing node names
				startNewNode(t, state, "node-0")
				startNewNode(t, state, "node-1")
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="true"} 4`,
					)
					verifyClusterReady(t, p)
					verifyPeers(t, p, 4)
					// NOTE: this and error logs are the only reliable indication that something went wrong...
					// Currently, the cluster will continue operating with name conflicts, potentially doing some
					// duplicated work. But this is not necessarily a bad choice of how to handle this.
					metricsContain(t, p.address, `cluster_node_gossip_received_events_total{cluster_name="cluster_e2e_test",event="node_conflict"}`)
				}
				verifyLookupInvariants(t, state.peers[:4]) // only check the healthy peers, ignore the conflicting ones
			},
		},
		{
			name:             "two split brain clusters join together after network fixed",
			alloyConfig:      `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial: 4,
			initialIsolatedNodes: []string{
				"node-1", "node-2",
			},
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 2`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 2`,
					)
					if p.nodeName == "node-1" || p.nodeName == "node-2" {
						verifyMetrics(t, p,
							`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-1___node-2",ready="true"} 2`,
						)
					} else {
						verifyMetrics(t, p,
							`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-3",ready="true"} 2`,
						)
					}
					verifyClusterReady(t, p)
					verifyPeers(t, p, 2)
				}
			},
			changes: func(state *testState) {
				joinIsolatedNetworks(state)
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="true"} 4`,
					)
					verifyClusterReady(t, p)
					verifyPeers(t, p, 4)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:               "cluster with minimum size - sufficient nodes join",
			alloyConfig:        `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial:   4,
			minimumClusterSize: 5,
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="false"} 4`,
						`cluster_minimum_size{cluster_name="cluster_e2e_test"} 5`,
						`cluster_ready_for_traffic{cluster_name="cluster_e2e_test"} 0`,
					)
					verifyClusterNotReady(t, p)
				}
			},
			changes: func(state *testState) {
				startNewNode(t, state, "node-4")
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 5`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 5`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3___node-4",ready="true"} 5`,
						`cluster_minimum_size{cluster_name="cluster_e2e_test"} 5`,
						`cluster_ready_for_traffic{cluster_name="cluster_e2e_test"} 1`,
					)
					verifyClusterReady(t, p)
					verifyPeers(t, p, 5)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:                   "too small cluster - deadline passes",
			alloyConfig:            `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial:       4,
			minimumClusterSize:     5,
			minimumSizeWaitTimeout: 5 * time.Second,
			extraAllowedErrors: []string{
				"deadline passed, marking cluster as ready to admit traffic",
				"failed to broadcast leave message to cluster",
				"timeout waiting for leave broadcast",
			},
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="false"} 4`,
						`cluster_minimum_size{cluster_name="cluster_e2e_test"} 5`,
						`cluster_ready_for_traffic{cluster_name="cluster_e2e_test"} 0`,
					)
					verifyClusterNotReady(t, p)
				}
			},
			changes: func(state *testState) {
				// Wait for the minimum size wait timeout to pass
				time.Sleep(5 * time.Second)
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3",ready="true"} 4`,
						`cluster_minimum_size{cluster_name="cluster_e2e_test"} 5`,
						`cluster_ready_for_traffic{cluster_name="cluster_e2e_test"} 1`,
					)
					verifyClusterReady(t, p)
					verifyPeers(t, p, 4)
				}
				verifyLookupInvariants(t, state.peers)
			},
		},
		{
			name:               "cluster dips below minimum size",
			alloyConfig:        `testcomponents.cluster_state_tracker "foo" {}`,
			nodeCountInitial:   5,
			minimumClusterSize: 5,
			extraAllowedErrors: []string{
				"minimum cluster size requirements are not met - marking cluster as not ready for traffic",
				"failed to broadcast leave message to cluster",
				"timeout waiting for leave broadcast",
			},
			assertionsInitial: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 5`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 5`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-0___node-1___node-2___node-3___node-4",ready="true"} 5`,
						`cluster_minimum_size{cluster_name="cluster_e2e_test"} 5`,
						`cluster_ready_for_traffic{cluster_name="cluster_e2e_test"} 1`,
					)
					verifyClusterReady(t, p)
					verifyPeers(t, p, 5)
				}
			},
			changes: func(state *testState) {
				state.peers[0].shutdown()
				state.peers = state.peers[1:]
			},
			assertionsFinal: func(t *assert.CollectT, state *testState) {
				for _, p := range state.peers {
					verifyMetrics(t, p,
						`cluster_node_info{state="participant"} 1`,
						`cluster_node_peers{cluster_name="cluster_e2e_test",state="participant"} 4`,
						`cluster_node_gossip_alive_peers{cluster_name="cluster_e2e_test"} 4`,
						`cluster_state{component_id="testcomponents.cluster_state_tracker.foo",component_path="/",peers="node-1___node-2___node-3___node-4",ready="false"} 4`,
						`cluster_minimum_size{cluster_name="cluster_e2e_test"} 5`,
						`cluster_ready_for_traffic{cluster_name="cluster_e2e_test"} 0`,
					)
					verifyClusterNotReady(t, p)
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
		if tc.extraAllowedErrors == nil {
			tc.extraAllowedErrors = []string{}
		}
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setDefaults(&tc)
			ctx, cancel := context.WithTimeout(t.Context(), tc.assertionsTimeout*2+5*time.Second)
			defer cancel()

			state := &testState{
				ctx:      ctx,
				peers:    make([]*testPeer, 0),
				testCase: &tc,
			}

			for i := 0; i < tc.nodeCountInitial; i++ {
				startNewNode(t, state, fmt.Sprintf("node-%d", i))
			}
			t.Logf("Initial nodes started")

			assert.EventuallyWithT(t, func(t *assert.CollectT) {
				tc.assertionsInitial(t, state)
			}, 60*time.Second, 1*time.Second)
			t.Logf("Initial assertions checked")

			tc.changes(state)
			t.Logf("Changes applied")

			assert.EventuallyWithT(t, func(t *assert.CollectT) {
				tc.assertionsFinal(t, state)
			}, 60*time.Second, 1*time.Second)
			t.Logf("Final assertions checked")

			for _, p := range state.peers {
				p.logsWriter.disableErrorChecking.Store(true) // stop checking log errors for shutdown
			}
			cancel()
			state.shutdownGroup.Wait()
			t.Logf("Shutdown complete")
		})
	}
}

func verifyClusterNotReady(t *assert.CollectT, p *testPeer) {
	clusterService, ok := p.clusterService.Data().(cluster.Cluster)
	require.True(t, ok)
	require.False(t, clusterService.Ready(), "Cluster should not be ready")
}

func verifyClusterReady(t *assert.CollectT, p *testPeer) {
	clusterService, ok := p.clusterService.Data().(cluster.Cluster)
	require.True(t, ok)
	require.True(t, clusterService.Ready(), "Cluster should be ready")
}

type testPeer struct {
	nodeName          string
	address           string
	clusterService    *cluster.Service
	httpService       *httpservice.Service
	ctx               context.Context
	shutdown          context.CancelFunc
	mutex             sync.Mutex
	discoverablePeers []string // list of peers that this peer can discover
	logsWriter        *logsWriter
}

func (p *testPeer) discoveryFn(_ discovery.Options) (discovery.DiscoverFn, error) {
	return func() ([]string, error) {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		return p.discoverablePeers, nil
	}, nil
}

func (p *testPeer) setDiscoverablePeers(addresses []string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.discoverablePeers = addresses
}

type testState struct {
	peers         []*testPeer
	ctx           context.Context
	shutdownGroup sync.WaitGroup
	testCase      *testCase
}

func startNewNode(t *testing.T, state *testState, nodeName string) {
	nodeAddress := fmt.Sprintf("127.0.0.1:%d", getFreePort(t))

	// Get list of join peers addresses that exist so far
	joinPeers := make([]string, 0, len(state.peers))
	isNewPeerIsolated := slices.Contains(state.testCase.initialIsolatedNodes, nodeName)
	for _, p := range state.peers {
		isThisPeerIsolated := slices.Contains(state.testCase.initialIsolatedNodes, p.nodeName)
		if isNewPeerIsolated && isThisPeerIsolated || !isNewPeerIsolated && !isThisPeerIsolated {
			joinPeers = append(joinPeers, p.address)
		}
	}

	t.Logf("Starting new node %s with join peers: %v", nodeName, joinPeers)

	// Create a node-specific context
	peerCtx, peerCancel := context.WithCancel(state.ctx)

	newPeer := &testPeer{
		nodeName:          nodeName,
		address:           nodeAddress,
		ctx:               peerCtx,
		shutdown:          peerCancel,
		discoverablePeers: joinPeers,
	}

	nodeDirPath := path.Join(t.TempDir(), nodeName)
	newPeer.logsWriter = &logsWriter{
		t:                  t,
		out:                os.Stdout,
		prefix:             fmt.Sprintf("%s: ", nodeName),
		extraAllowedErrors: state.testCase.extraAllowedErrors,
	}
	logger, err := logging.New(
		newPeer.logsWriter,
		logging.Options{
			Level:  logging.LevelDebug,
			Format: logging.FormatDefault,
		},
	)
	require.NoError(t, err)

	tracer, err := tracing.New(tracing.DefaultOptions)
	require.NoError(t, err)

	reg := prometheus.NewRegistry()
	clusterService, err := alloycli.NewClusterService(alloycli.ClusterOptions{
		Log:     log.With(logger, "service", "cluster"),
		Tracer:  tracer,
		Metrics: reg,

		EnableClustering:       true,
		NodeName:               nodeName,
		AdvertiseAddress:       nodeAddress,
		ListenAddress:          nodeAddress,
		RejoinInterval:         1 * time.Second,
		AdvertiseInterfaces:    advertise.DefaultInterfaces,
		ClusterMaxJoinPeers:    5,
		ClusterName:            "cluster_e2e_test",
		MinimumClusterSize:     state.testCase.minimumClusterSize,
		MinimumSizeWaitTimeout: state.testCase.minimumSizeWaitTimeout,
	}, newPeer.discoveryFn)
	require.NoError(t, err)
	newPeer.clusterService = clusterService

	httpService := httpservice.New(httpservice.Options{
		Logger:   logger,
		Tracer:   tracer,
		Gatherer: reg,

		ReadyFunc:  func() bool { return true },
		ReloadFunc: func() error { return nil },

		HTTPListenAddr: nodeAddress,
	})
	newPeer.httpService = httpService

	configFilePath := path.Join(nodeDirPath, "config.alloy")
	remoteCfgService, err := remotecfgservice.New(remotecfgservice.Options{
		Logger:      log.With(logger, "service", "remotecfg"),
		ConfigPath:  configFilePath,
		StoragePath: nodeDirPath,
		Metrics:     reg,
	})
	require.NoError(t, err)

	f, err := runtime.New(runtime.Options{
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
	require.NoError(t, err)

	// Add the new peer to the state
	state.peers = append(state.peers, newPeer)

	// Increment the WaitGroup before starting the node
	state.shutdownGroup.Go(func() {
		// Ensure the WaitGroup is decremented when the node finishes running
		f.Run(peerCtx)
	})

	src, err := runtime.ParseSource(t.Name(), []byte(state.testCase.alloyConfig))
	require.NoError(t, err)
	err = f.LoadSource(src, nil, configFilePath)
	require.NoError(t, err)

	err = clusterService.ChangeState(peerCtx, peer.StateParticipant)
	require.NoError(t, err)
}

func joinIsolatedNetworks(state *testState) {
	allPeerAddresses := make([]string, 0, len(state.peers))
	for _, p := range state.peers {
		allPeerAddresses = append(allPeerAddresses, p.address)
	}
	for _, p := range state.peers {
		p.setDiscoverablePeers(allPeerAddresses)
	}
}

func verifyPeers(t *assert.CollectT, p *testPeer, expectedLength int) {
	clusterService, ok := p.clusterService.Data().(cluster.Cluster)
	require.True(t, ok)
	peers := clusterService.Peers()
	require.Len(t, peers, expectedLength)
	for _, actualPeer := range peers {
		if actualPeer.Self {
			require.Equal(t, p.nodeName, actualPeer.Name)
			require.Equal(t, p.address, actualPeer.Addr)
			require.Equal(t, peer.StateParticipant, actualPeer.State)
			return
		}
	}
	require.Fail(t, "Could not find self in peers")
}

func verifyMetrics(t *assert.CollectT, p *testPeer, metrics ...string) {
	for _, m := range metrics {
		metricsContain(t, p.address, m+"\n") // add new line to validate the metric has exact value as required
	}
}

// metricsContain fetches metrics from the given URL and checks if they
// contain a line with the specified metric name
func metricsContain(t *assert.CollectT, nodeAddress string, text string) {
	metricsURL := fmt.Sprintf("http://%s/metrics", nodeAddress)
	body, err := fetchMetrics(metricsURL)
	require.NoError(t, err)
	require.Contains(t, body, text, "Could not find %q in the metrics of %q. All metrics: %s", text, nodeAddress, body)
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
	t                    *testing.T
	out                  io.Writer
	prefix               string
	extraAllowedErrors   []string
	disableErrorChecking atomic.Bool
}

var errorsAllowlist = []string{
	"over TCP but UDP probes failed, network may be misconfigured",  // TODO: we should investigate and fix this if a real issue
	"failed to extract directory path from configPath",              // unrelated to this test
	"failed to connect to peers; bootstrapping a new cluster",       // should be allowed only once for first node
	`msg="node exited with error" node=remotecfg err="noop client"`, // related to remotecfg service mock ups
	`msg="failed to rejoin list of peers"`,                          // this is because list of initial join peers is fixed in tests
	`stream closed`,
	`i/o timeout`,
	`failed to receive and remove the stream label header`,
}

func (w *logsWriter) Write(p []byte) (n int, err error) {
	prefixed := []byte(w.prefix)
	prefixed = append(prefixed, p...)

	// Check for warnings or errors in the log message if not disabled
	logMsg := string(p)
	if !w.disableErrorChecking.Load() && (strings.Contains(logMsg, "level=warn") || strings.Contains(logMsg, "level=error")) {
		isAllowed := false

		// Check against global allowlist
		for _, allowedErr := range append(errorsAllowlist, w.extraAllowedErrors...) {
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

func verifyLookupInvariants(t *assert.CollectT, peers []*testPeer) {
	const numStrings = 1000

	// Generate random strings
	randomStrings := make([]string, numStrings)
	for i := range randomStrings {
		randomStrings[i] = fmt.Sprintf("test-key-%d-%d", time.Now().UnixNano(), i)
	}

	// For each string, check which peer it's assigned to
	for _, s := range randomStrings {
		key := shard.StringKey(s)

		ownerships := make(map[string][]string)
		for _, p := range peers {
			clusterService, ok := p.clusterService.Data().(cluster.Cluster)
			require.True(t, ok)

			owningPeers, err := clusterService.Lookup(key, 1, shard.OpReadWrite)
			require.NoError(t, err, "Lookup should not fail")
			require.Len(t, owningPeers, 1, "Lookup should return exactly one peer for key %q", s)

			var ownerNames []string
			for _, owner := range owningPeers {
				ownerNames = append(ownerNames, owner.Name)
			}
			ownerships[p.nodeName] = ownerNames
		}
		require.Len(t, ownerships, len(peers), "Each peer should report an ownership for each key")
		owningPeersUnique := make(map[string]any)
		for nodeName, owningPeers := range ownerships {
			require.Len(t, owningPeers, 1, "Key %q should be owned by exactly one peer, but was owned by %d peers (as seen by %q)",
				s, len(owningPeers), nodeName)
			owningPeersUnique[owningPeers[0]] = struct{}{}
		}
		require.Len(t, owningPeersUnique, 1, "All peers should agree on the owner of key %q, but got different owners: %v", s, ownerships)
	}
}
