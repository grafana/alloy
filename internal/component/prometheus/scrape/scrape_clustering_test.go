package scrape

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/assertmetrics"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/grafana/alloy/internal/util/testtarget"
)

const (
	testTimeout = 30 * time.Second
)

var (
	peer1Self = peer.Peer{Name: "peer1", Addr: "peer1", Self: true, State: peer.StateParticipant}
	peer2     = peer.Peer{Name: "peer2", Addr: "peer2", Self: false, State: peer.StateParticipant}
	peer3     = peer.Peer{Name: "peer3", Addr: "peer3", Self: false, State: peer.StateParticipant}

	// There is a race condition in prometheus where calls to NewManager can race over a package-global variable when
	// calling targetMetadataCache.registerManager(m). This is a workaround to prevent this for now.
	// TODO(thampiotr): Open an issue in prometheus to fix this?
	promManagerMutex sync.Mutex
)

type testCase struct {
	name                        string
	excludedLabels              []string
	expectedOwnershipLabels     []string
	rotateCredentials           bool
	initialTargetsAssignment    map[peer.Peer][]int
	updatedTargetsAssignment    map[peer.Peer][]int
	expectedStalenessInjections []int
	expectedNoStaleness         []int
	expectedMovedTargetsTotal   int
}

var testCases = []testCase{
	{
		name:                    "credentials rotate during ownership handoff",
		excludedLabels:          []string{"__param_sig"},
		expectedOwnershipLabels: []string{"__address__"},
		rotateCredentials:       true,
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer2: {1},
		},
		expectedNoStaleness:       []int{1},
		expectedMovedTargetsTotal: 1,
	},
	{
		name:                    "entire ownership group disappears",
		excludedLabels:          []string{"__address__"},
		expectedOwnershipLabels: []string{},
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2},
		},
		updatedTargetsAssignment:    map[peer.Peer][]int{},
		expectedStalenessInjections: []int{1, 2},
	},
	{
		name:                    "grouped targets move without staleness",
		excludedLabels:          []string{"__address__"},
		expectedOwnershipLabels: []string{},
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer2: {1, 2, 3},
		},
		expectedNoStaleness:       []int{1, 2, 3},
		expectedMovedTargetsTotal: 3,
	},
	{
		name:                    "group moves while one member disappears",
		excludedLabels:          []string{"__address__"},
		expectedOwnershipLabels: []string{},
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer2: {1, 2},
		},
		expectedNoStaleness:       []int{1, 2, 3},
		expectedMovedTargetsTotal: 3,
	},
	{
		name: "no targets move",
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3},
			peer2:     {4, 5},
			peer3:     {6, 7},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3},
			peer2:     {4, 5},
			peer3:     {6, 7},
		},
		expectedMovedTargetsTotal: 0,
	},
	{
		name: "one target added",
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2},
			peer2:     {4, 5},
			peer3:     {6, 7},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3},
			peer2:     {4, 5},
			peer3:     {6, 7},
		},
		expectedMovedTargetsTotal: 0,
	},
	{
		name: "staleness injected when two targets disappear",
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3},
			peer2:     {4, 5},
			peer3:     {6, 7},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {2},
			peer2:     {4, 5},
			peer3:     {6, 7},
		},
		expectedStalenessInjections: []int{1, 3},
		expectedMovedTargetsTotal:   0,
	},
	{
		name: "no staleness injected when two targets move to other instances",
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3, 4, 5},
			peer2:     {6, 7},
			peer3:     {8, 9},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {2, 4, 5},
			peer2:     {1, 6, 7},
			peer3:     {3, 8, 9},
		},
		expectedMovedTargetsTotal: 2,
	},
	{
		name: "staleness injected when one target disappeared and another moved",
		initialTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {1, 2, 3, 4, 5},
			peer2:     {6, 7},
			peer3:     {8, 9},
		},
		updatedTargetsAssignment: map[peer.Peer][]int{
			peer1Self: {2, 4, 5},
			peer2:     {1, 6, 7},
			peer3:     {8, 9},
		},
		expectedStalenessInjections: []int{3},
		expectedMovedTargetsTotal:   1,
	},
}

func TestDetectingMovedTargets(t *testing.T) {
	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			alloyMetricsReg := client.NewRegistry()
			fakeCluster := &fakeCluster{
				peers: []peer.Peer{peer1Self, peer2, peer3},
			}
			opts := testOptions(t, alloyMetricsReg, fakeCluster)
			args := testArgs()
			args.Clustering.ExcludedLabels = tc.excludedLabels

			appender := testappender.NewCollectingAppender()
			args.ForwardTo = []storage.Appendable{testappender.ConstantAppendable{Inner: appender}}

			testTargets, shutdownTargets := createTestTargets(tc)
			defer shutdownTargets()

			// Set initial targets
			args.Targets = getActiveTargets(tc.initialTargetsAssignment, testTargets)
			if tc.rotateCredentials {
				args.Targets = withSignature(args.Targets, "old")
			}
			setUpClusterLookup(fakeCluster, tc.initialTargetsAssignment, testTargets, tc.expectedOwnershipLabels)

			// Create and start the component
			promManagerMutex.Lock()
			s, err := New(opts, args)
			promManagerMutex.Unlock()

			require.NoError(t, err)
			ctx, cancelRun := context.WithTimeout(t.Context(), testTimeout)
			runErr := make(chan error)
			go func() {
				err := s.Run(ctx)
				runErr <- err
			}()

			// Verify metrics and scraping of the right targets
			waitForMetricValue(t, alloyMetricsReg, "prometheus_scrape_targets_gauge", float64(len(tc.initialTargetsAssignment[peer1Self])))
			waitForMetricValue(t, alloyMetricsReg, "prometheus_scrape_targets_moved_total", float64(0))
			waitForTargetsToBeScraped(t, appender, tc.initialTargetsAssignment[peer1Self])

			// Update targets
			args.Targets = getActiveTargets(tc.updatedTargetsAssignment, testTargets)
			if tc.rotateCredentials {
				args.Targets = withSignature(args.Targets, "new")
			}
			setUpClusterLookup(fakeCluster, tc.updatedTargetsAssignment, testTargets, tc.expectedOwnershipLabels)
			require.NoError(t, s.Update(args))

			// Verify metrics and scraping of the right targets
			waitForMetricValue(t, alloyMetricsReg, "prometheus_scrape_targets_gauge", float64(len(tc.updatedTargetsAssignment[peer1Self])))
			waitForMetricValue(t, alloyMetricsReg, "prometheus_scrape_targets_moved_total", float64(tc.expectedMovedTargetsTotal))
			waitForTargetsToBeScraped(t, appender, tc.updatedTargetsAssignment[peer1Self])

			// Verify staleness injections
			waitForStalenessInjections(t, appender, tc.expectedStalenessInjections)
			if len(tc.expectedNoStaleness) > 0 {
				// Wait for the scrape manager to stop the old loops, then observe
				// beyond their delayed stale-marker window before cancelling Run.
				require.Eventually(t, func() bool {
					return len(s.scraper.TargetsActive()[opts.ID]) == len(tc.updatedTargetsAssignment[peer1Self])
				}, testTimeout, 10*time.Millisecond)
				require.Never(t, func() bool {
					for _, id := range tc.expectedNoStaleness {
						sample := appender.LatestSampleFor(fmt.Sprintf(`{__name__="test_counter", instance="%d", job="prometheus.scrape.test"}`, id))
						if sample != nil && value.IsStaleNaN(sample.Value) {
							return true
						}
					}
					return false
				}, 5*args.ScrapeInterval, 10*time.Millisecond)
			}

			cancelRun()
			require.NoError(t, <-runErr)
			for _, id := range tc.expectedNoStaleness {
				verifyTestTargetExposed(t, id, appender)
			}
		})
	}
}

func withSignature(targets []discovery.Target, signature string) []discovery.Target {
	result := make([]discovery.Target, 0, len(targets))
	for _, target := range targets {
		builder := discovery.NewTargetBuilderFrom(target)
		builder.Set("__param_sig", signature)
		result = append(result, builder.Target())
	}
	return result
}

func testArgs() Arguments {
	var args Arguments
	args.SetToDefault()
	args.Clustering.Enabled = true
	args.ScrapeInterval = 100 * time.Millisecond
	args.ScrapeTimeout = args.ScrapeInterval
	args.HonorLabels = true
	err := args.Validate()
	if err != nil {
		panic(fmt.Errorf("invalid arguments for test: %w", err))
	}
	return args
}

func testOptions(t *testing.T, alloyMetricsReg *client.Registry, fakeCluster *fakeCluster) component.Options {
	opts := component.Options{
		Logger:     util.TestAlloyLogger(t).Slog(),
		Registerer: alloyMetricsReg,
		ID:         "prometheus.scrape.test",
		GetServiceData: func(name string) (any, error) {
			switch name {
			case http.ServiceName:
				return http.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "alloy.internal:1245",
					BaseHTTPPath:     "/",
					DialFunc:         (&net.Dialer{}).DialContext,
				}, nil

			case cluster.ServiceName:
				return fakeCluster, nil
			case labelstore.ServiceName:
				return labelstore.New(nil, alloyMetricsReg), nil
			case livedebugging.ServiceName:
				return livedebugging.NewLiveDebugging(), nil
			default:
				return nil, fmt.Errorf("service %q does not exist", name)
			}
		},
	}
	return opts
}

func getActiveTargets(assignment map[peer.Peer][]int, testTargets map[int]*testtarget.TestTarget) []discovery.Target {
	active := make([]discovery.Target, 0)
	for _, targets := range assignment {
		for _, id := range targets {
			active = append(active, testTargets[id].Target())
		}
	}
	return active
}

func createTestTargets(tc testCase) (map[int]*testtarget.TestTarget, func()) {
	testTargets := map[int]*testtarget.TestTarget{}
	gatherTestTargets := func(targets []int) {
		for _, id := range targets {
			if _, ok := testTargets[id]; !ok {
				testTargets[id] = testTargetWithId(id)
			}
		}
	}

	for _, targets := range tc.initialTargetsAssignment {
		gatherTestTargets(targets)
	}
	for _, targets := range tc.updatedTargetsAssignment {
		gatherTestTargets(targets)
	}

	shutdownTargets := func() {
		for _, t := range testTargets {
			t.Close()
		}
	}
	return testTargets, shutdownTargets
}

func setUpClusterLookup(fakeCluster *fakeCluster, assignment map[peer.Peer][]int, targets map[int]*testtarget.TestTarget, ownershipLabels []string) {
	lookup := make(map[shard.Key][]peer.Peer)
	for owningPeer, ownedTargets := range assignment {
		for _, id := range ownedTargets {
			target := targets[id].Target()
			key := target.NonMetaLabelsHash()
			if ownershipLabels != nil {
				key = target.SpecificLabelsHash(ownershipLabels)
			}
			lookup[shard.Key(key)] = []peer.Peer{owningPeer}
		}
	}
	fakeCluster.mut.Lock()
	fakeCluster.lookupMap = lookup
	fakeCluster.mut.Unlock()
}

func waitForTargetsToBeScraped(t *testing.T, appender testappender.CollectingAppender, targets []int) {
	require.EventuallyWithT(
		t,
		func(t *assert.CollectT) {
			for _, id := range targets {
				verifyTestTargetExposed(t, id, appender)
			}
		},
		15*time.Second,
		10*time.Millisecond,
	)
}

func waitForMetricValue(t *testing.T, alloyMetricsReg *client.Registry, name string, value float64) {
	require.EventuallyWithT(
		t,
		func(t *assert.CollectT) {
			assertmetrics.AssertValueInReg(
				t,
				alloyMetricsReg,
				name,
				labels.EmptyLabels(),
				value,
			)
		},
		testTimeout,
		10*time.Millisecond,
	)
}

func testTargetWithId(id int) *testtarget.TestTarget {
	t := testtarget.NewTestTarget()
	t.AddCounter(client.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
		ConstLabels: map[string]string{
			"instance": fmt.Sprintf("%d", id),
		},
	}).Add(100 + float64(id))
	t.AddGauge(client.GaugeOpts{
		Name: "test_gauge",
		Help: "A test gauge",
		ConstLabels: map[string]string{
			"instance": fmt.Sprintf("%d", id),
		},
	}).Set(10 + float64(id))
	return t
}

func verifyTestTargetExposed(t assert.TestingT, id int, appender testappender.CollectingAppender) {
	counter := appender.LatestSampleFor(fmt.Sprintf(`{__name__="test_counter", instance="%d", job="prometheus.scrape.test"}`, id))
	assert.NotNil(t, counter)
	if counter == nil {
		return
	}
	assert.Equal(t, 100+float64(id), counter.Value)

	gauge := appender.LatestSampleFor(fmt.Sprintf(`{__name__="test_gauge", instance="%d", job="prometheus.scrape.test"}`, id))
	assert.NotNil(t, gauge)
	if gauge == nil {
		return
	}
	assert.Equal(t, 10+float64(id), gauge.Value)
}

func waitForStalenessInjections(t *testing.T, appender testappender.CollectingAppender, expectedTargets []int) {
	assert.EventuallyWithT(
		t,
		func(t *assert.CollectT) {
			for _, targetId := range expectedTargets {
				verifyStalenessInjectedForTarget(t, targetId, appender)
			}
		},
		testTimeout,
		10*time.Millisecond,
	)
}

func verifyStalenessInjectedForTarget(t assert.TestingT, targetId int, appender testappender.CollectingAppender) {
	counter := appender.LatestSampleFor(fmt.Sprintf(`{__name__="test_counter", instance="%d", job="prometheus.scrape.test"}`, targetId))
	assert.NotNil(t, counter)
	if counter == nil {
		return
	}
	assert.True(t, value.IsStaleNaN(counter.Value))

	gauge := appender.LatestSampleFor(fmt.Sprintf(`{__name__="test_gauge", instance="%d", job="prometheus.scrape.test"}`, targetId))
	assert.NotNil(t, gauge)
	if gauge == nil {
		return
	}
	assert.True(t, value.IsStaleNaN(gauge.Value))
}

type fakeCluster struct {
	mut       sync.RWMutex
	lookupMap map[shard.Key][]peer.Peer
	peers     []peer.Peer
	onLookup  func(shard.Key)
}

func (f *fakeCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	if f.onLookup != nil {
		f.onLookup(key)
	}
	return f.lookupMap[key], nil
}

func (f *fakeCluster) Peers() []peer.Peer {
	return f.peers
}

func (f *fakeCluster) Ready() bool {
	return true
}

func (f *fakeCluster) Enabled() bool {
	return true
}
