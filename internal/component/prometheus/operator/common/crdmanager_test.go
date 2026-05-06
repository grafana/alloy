package common

import (
	"fmt"
	"testing"

	"golang.org/x/exp/maps"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
	"k8s.io/apimachinery/pkg/util/intstr"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestClearConfigsSameNsSamePrefix(t *testing.T) {
	logger := log.NewNopLogger()
	m := newCrdManager(
		component.Options{
			Logger:         logger,
			GetServiceData: func(name string) (any, error) { return nil, nil },
		},
		cluster.Mock(),
		logger,
		&operator.DefaultArguments,
		KindServiceMonitor,
		labelstore.New(logger, prometheus.DefaultRegisterer),
	)

	m.discoveryManager = newMockDiscoveryManager()
	m.scrapeManager = newMockScrapeManager()

	targetPort := intstr.FromInt(9090)
	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"group": "my-group",
				},
			},
			Endpoints: []promopv1.Endpoint{
				{
					TargetPort:    &targetPort,
					ScrapeTimeout: "5s",
					Interval:      "10s",
				},
			},
		},
	})
	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor-another",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"group": "my-group",
				},
			},
			Endpoints: []promopv1.Endpoint{
				{
					TargetPort:    &targetPort,
					ScrapeTimeout: "5s",
					Interval:      "10s",
				},
			},
		}})

	require.ElementsMatch(t, []string{"serviceMonitor/monitoring/svcmonitor-another/0", "serviceMonitor/monitoring/svcmonitor/0"}, maps.Keys(m.discoveryConfigs))
	m.clearConfigs("monitoring", "svcmonitor")
	require.ElementsMatch(t, []string{"monitoring/svcmonitor", "monitoring/svcmonitor-another"}, maps.Keys(m.crdsToMapKeys))
	require.ElementsMatch(t, []string{"serviceMonitor/monitoring/svcmonitor-another/0"}, maps.Keys(m.discoveryConfigs))
	require.ElementsMatch(t, []string{"serviceMonitor/monitoring/svcmonitor-another"}, maps.Keys(m.debugInfo))
}

func TestClearConfigsProbe(t *testing.T) {
	logger := log.NewNopLogger()
	m := newCrdManager(
		component.Options{
			Logger:         logger,
			GetServiceData: func(name string) (any, error) { return nil, nil },
		},
		cluster.Mock(),
		logger,
		&operator.DefaultArguments,
		KindProbe,
		labelstore.New(logger, prometheus.DefaultRegisterer),
	)

	m.discoveryManager = newMockDiscoveryManager()
	m.scrapeManager = newMockScrapeManager()

	m.onAddProbe(&promopv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "probe",
		},
		Spec: promopv1.ProbeSpec{},
	})
	m.onAddProbe(&promopv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "probe-another",
		},
		Spec: promopv1.ProbeSpec{}})

	require.ElementsMatch(t, []string{"probe/monitoring/probe-another", "probe/monitoring/probe"}, maps.Keys(m.discoveryConfigs))
	m.clearConfigs("monitoring", "probe")
	require.ElementsMatch(t, []string{"monitoring/probe", "monitoring/probe-another"}, maps.Keys(m.crdsToMapKeys))
	require.ElementsMatch(t, []string{"probe/monitoring/probe-another"}, maps.Keys(m.discoveryConfigs))
	require.ElementsMatch(t, []string{"probe/monitoring/probe-another"}, maps.Keys(m.debugInfo))
}

type mockDiscoveryManager struct {
}

func newMockDiscoveryManager() *mockDiscoveryManager {
	return &mockDiscoveryManager{}
}

func (m *mockDiscoveryManager) Run() error {
	return nil
}

func (m *mockDiscoveryManager) SyncCh() <-chan map[string][]*targetgroup.Group {
	return nil
}

func (m *mockDiscoveryManager) ApplyConfig(cfg map[string]discovery.Configs) error {
	return nil
}

type mockScrapeManager struct {
}

func newMockScrapeManager() *mockScrapeManager {
	return &mockScrapeManager{}
}

func (m *mockScrapeManager) Run(tsets <-chan map[string][]*targetgroup.Group) error {
	return nil
}

func (m *mockScrapeManager) Stop() {

}

func (m *mockScrapeManager) TargetsActive() map[string][]*scrape.Target {
	return nil
}

func (m *mockScrapeManager) ApplyConfig(cfg *config.Config) error {
	return nil
}

// testCluster is a fake cluster implementation for testing filterTargets.
type testCluster struct {
	lookupMap map[shard.Key][]peer.Peer
	peers     []peer.Peer
	ready     bool
}

func (f *testCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	if peers, ok := f.lookupMap[key]; ok {
		return peers, nil
	}
	// If key not in lookupMap, return an error to trigger fallback behavior.
	return nil, fmt.Errorf("no peers for key")
}

func (f *testCluster) Peers() []peer.Peer {
	return f.peers
}

func (f *testCluster) Ready() bool {
	return f.ready
}

func TestFilterTargetsWithoutZoneAware(t *testing.T) {
	selfPeer := peer.Peer{Name: "self", Addr: "127.0.0.1", Self: true, State: peer.StateParticipant}
	otherPeer := peer.Peer{Name: "other", Addr: "127.0.0.2", Self: false, State: peer.StateParticipant}

	target1 := model.LabelSet{"__address__": "10.0.0.1:9090"}
	target2 := model.LabelSet{"__address__": "10.0.0.2:9090"}
	target3 := model.LabelSet{
		"__address__":   "10.0.0.3:9090",
		metaPodNodeName: "node-3",
	}

	key1 := shard.StringKey(nonMetaLabelString(target1))
	key2 := shard.StringKey(nonMetaLabelString(target2))
	key3 := shard.StringKey(nonMetaLabelString(target3))

	c := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer, otherPeer},
		lookupMap: map[shard.Key][]peer.Peer{
			key1: {selfPeer},
			key2: {otherPeer},
			key3: {selfPeer},
		},
	}

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source:  "source1",
				Targets: []model.LabelSet{target1, target2, target3},
			},
		},
	}

	// Without zone-aware (empty localZone, nil ring), all targets go through normal hash ring.
	result, stats := filterTargets(input, c, "", nil, nil)

	require.Len(t, result["job1"], 1)
	// target1 -> selfPeer (kept), target2 -> otherPeer (dropped), target3 -> selfPeer (kept)
	require.Len(t, result["job1"][0].Targets, 2)
	require.Contains(t, result["job1"][0].Targets, target1)
	require.Contains(t, result["job1"][0].Targets, target3)

	// Verify stats
	require.Equal(t, 3, stats.total)
	require.Equal(t, 2, stats.kept)
	require.Equal(t, 1, stats.dropped)
	require.Equal(t, 0, stats.sameZone)
	require.Equal(t, 0, stats.diffZone)
	require.Equal(t, 0, stats.noZone)
}

func TestFilterTargetsZoneAware(t *testing.T) {
	selfPeer := peer.Peer{Name: "self", Addr: "127.0.0.1:12345", Self: true, State: peer.StateParticipant}
	sameZonePeer := peer.Peer{Name: "same-zone", Addr: "127.0.0.2:12345", Self: false, State: peer.StateParticipant}
	diffZonePeer := peer.Peer{Name: "diff-zone", Addr: "127.0.0.3:12345", Self: false, State: peer.StateParticipant}

	// Targets in us-east-1a (same zone as local) via pod node name
	targetSameZone1 := model.LabelSet{
		"__address__":   "10.0.0.1:9090",
		metaPodNodeName: "node-a1",
	}
	targetSameZone2 := model.LabelSet{
		"__address__":   "10.0.0.2:9090",
		metaPodNodeName: "node-a2",
	}

	// Target in us-east-1b (different zone) via pod node name
	targetDiffZone := model.LabelSet{
		"__address__":   "10.0.0.3:9090",
		metaPodNodeName: "node-b1",
	}

	// Target with no zone info (should fall through to global ring)
	targetNoZone := model.LabelSet{
		"__address__": "10.0.0.4:9090",
	}

	nodeToZone := map[string]string{
		"node-a1": "us-east-1a",
		"node-a2": "us-east-1a",
		"node-b1": "us-east-1b",
	}

	keyNoZone := shard.StringKey(nonMetaLabelString(targetNoZone))

	// Global cluster ring contains all peers (self, same-zone, diff-zone).
	c := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer, sameZonePeer, diffZonePeer},
		lookupMap: map[shard.Key][]peer.Peer{
			keyNoZone: {selfPeer},
		},
	}

	// Local zone ring contains only peers in us-east-1a (self and same-zone peer).
	localRing := shard.Ring(512)
	localRing.SetPeers([]peer.Peer{selfPeer, sameZonePeer})

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source:  "source1",
				Targets: []model.LabelSet{targetSameZone1, targetSameZone2, targetDiffZone, targetNoZone},
			},
		},
	}

	// With zone-aware clustering, localZone = "us-east-1a"
	result, stats := filterTargets(input, c, "us-east-1a", localRing, nodeToZone)

	require.Len(t, result["job1"], 1)
	resultTargets := result["job1"][0].Targets

	// targetSameZone1: pod node name -> nodeToZone -> "us-east-1a" (same zone) -> local ring
	// targetSameZone2: pod node name -> nodeToZone -> "us-east-1a" (same zone) -> local ring
	// targetDiffZone: pod node name -> nodeToZone -> "us-east-1b" (diff zone) -> dropped
	// targetNoZone: no pod node name -> no zone -> global ring -> selfPeer owns it -> kept
	require.Contains(t, resultTargets, targetNoZone)
	for _, tgt := range resultTargets {
		// No target from a different zone should appear
		require.NotEqual(t, targetDiffZone, tgt)
	}

	// Verify stats
	require.Equal(t, 4, stats.total)
	require.Equal(t, 1, stats.diffZone) // targetDiffZone
	require.Equal(t, 2, stats.sameZone) // targetSameZone1, targetSameZone2
	require.Equal(t, 1, stats.noZone)   // targetNoZone
}

func TestFilterTargetsZoneAwareCrossZoneNotDropped(t *testing.T) {
	// This test verifies the fix for the correctness bug where targets in a
	// different zone could be scraped by nobody. With the sub-ring approach, each
	// zone's peers use their own ring for same-zone targets, ensuring coverage.

	// Simulate two nodes: nodeA in zone-1, nodeC in zone-2.
	selfPeer := peer.Peer{Name: "nodeA", Addr: "127.0.0.1:12345", Self: true, State: peer.StateParticipant}
	peerC := peer.Peer{Name: "nodeC", Addr: "127.0.0.3:12345", Self: false, State: peer.StateParticipant}

	// Target in zone-2 (via pod node name).
	targetZone2 := model.LabelSet{
		"__address__":   "10.0.0.1:9090",
		metaPodNodeName: "node-b1",
	}

	// Target in zone-1 (via pod node name).
	targetZone1 := model.LabelSet{
		"__address__":   "10.0.0.2:9090",
		metaPodNodeName: "node-a1",
	}

	nodeToZone := map[string]string{
		"node-a1": "us-east-1a",
		"node-b1": "us-east-1b",
	}

	// Global cluster has both peers.
	globalCluster := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer, peerC},
	}

	// NodeA's local ring contains only itself (only peer in zone-1).
	localRingA := shard.Ring(512)
	localRingA.SetPeers([]peer.Peer{selfPeer})

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source:  "source1",
				Targets: []model.LabelSet{targetZone2, targetZone1},
			},
		},
	}

	// From NodeA's perspective (zone-1):
	resultA, statsA := filterTargets(input, globalCluster, "us-east-1a", localRingA, nodeToZone)
	require.Len(t, resultA["job1"], 1)

	// targetZone2 is in zone-2 -> skipped by NodeA (zone filter).
	// targetZone1 is in zone-1 -> goes through NodeA's local ring (only self) -> kept.
	require.Len(t, resultA["job1"][0].Targets, 1)
	require.Contains(t, resultA["job1"][0].Targets, targetZone1)
	require.Equal(t, 1, statsA.sameZone)
	require.Equal(t, 1, statsA.diffZone)

	// Now simulate from NodeC's perspective (zone-2):
	selfPeerC := peer.Peer{Name: "nodeC", Addr: "127.0.0.3:12345", Self: true, State: peer.StateParticipant}
	peerA := peer.Peer{Name: "nodeA", Addr: "127.0.0.1:12345", Self: false, State: peer.StateParticipant}

	globalClusterC := &testCluster{
		ready: true,
		peers: []peer.Peer{peerA, selfPeerC},
	}

	// NodeC's local ring contains only itself (only peer in zone-2).
	localRingC := shard.Ring(512)
	localRingC.SetPeers([]peer.Peer{selfPeerC})

	resultC, statsC := filterTargets(input, globalClusterC, "us-east-1b", localRingC, nodeToZone)
	require.Len(t, resultC["job1"], 1)

	// targetZone2 is in zone-2 -> goes through NodeC's local ring (only self) -> kept.
	// targetZone1 is in zone-1 -> skipped by NodeC (zone filter).
	require.Len(t, resultC["job1"][0].Targets, 1)
	require.Contains(t, resultC["job1"][0].Targets, targetZone2)
	require.Equal(t, 1, statsC.sameZone)
	require.Equal(t, 1, statsC.diffZone)

	// KEY ASSERTION: Every target is scraped by exactly one node.
	// targetZone1 -> NodeA (zone-1), targetZone2 -> NodeC (zone-2). No gaps.
}

func TestFilterTargetsZoneAwareMultipleGroups(t *testing.T) {
	selfPeer := peer.Peer{Name: "self", Addr: "127.0.0.1:12345", Self: true, State: peer.StateParticipant}

	targetZoneA := model.LabelSet{
		"__address__":   "10.0.0.1:9090",
		metaPodNodeName: "node-a1",
	}
	targetZoneB := model.LabelSet{
		"__address__":   "10.0.0.2:9090",
		metaPodNodeName: "node-b1",
	}

	nodeToZone := map[string]string{
		"node-a1": "us-east-1a",
		"node-b1": "us-east-1b",
	}

	// Local ring with only self.
	localRing := shard.Ring(512)
	localRing.SetPeers([]peer.Peer{selfPeer})

	c := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer},
	}

	input := map[string][]*targetgroup.Group{
		"job1": {
			{Source: "source1", Targets: []model.LabelSet{targetZoneA}},
			{Source: "source2", Targets: []model.LabelSet{targetZoneB}},
		},
	}

	result, stats := filterTargets(input, c, "us-east-1a", localRing, nodeToZone)

	require.Len(t, result["job1"], 2)
	// First group: targetZoneA is same zone and selfPeer -> kept
	require.Len(t, result["job1"][0].Targets, 1)
	require.Contains(t, result["job1"][0].Targets, targetZoneA)
	// Second group: targetZoneB is different zone -> empty (but group preserved)
	require.Len(t, result["job1"][1].Targets, 0)
	require.Equal(t, 2, stats.total)
	require.Equal(t, 1, stats.sameZone)
	require.Equal(t, 1, stats.diffZone)
}

func TestFilterTargetsClusterNotReady(t *testing.T) {
	c := &testCluster{
		ready: false,
	}

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source: "source1",
				Targets: []model.LabelSet{
					{"__address__": "10.0.0.1:9090"},
				},
			},
		},
	}

	// Should return empty map when cluster is not ready, regardless of zone setting.
	localRing := shard.Ring(512)
	result, _ := filterTargets(input, c, "us-east-1a", localRing, nil)
	require.Len(t, result, 0)

	result, _ = filterTargets(input, c, "", nil, nil)
	require.Len(t, result, 0)
}

func TestFilterTargetsZoneAwarePreservesGroupStructure(t *testing.T) {
	selfPeer := peer.Peer{Name: "self", Addr: "127.0.0.1:12345", Self: true, State: peer.StateParticipant}

	targetDiffZone := model.LabelSet{
		"__address__":   "10.0.0.1:9090",
		metaPodNodeName: "node-b1",
	}

	nodeToZone := map[string]string{
		"node-b1": "us-east-1b",
	}

	localRing := shard.Ring(512)
	localRing.SetPeers([]peer.Peer{selfPeer})

	c := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer},
	}

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source:  "source1",
				Labels:  model.LabelSet{"team": "ops"},
				Targets: []model.LabelSet{targetDiffZone},
			},
		},
	}

	result, stats := filterTargets(input, c, "us-east-1a", localRing, nodeToZone)

	// The group structure should be preserved even when all targets are filtered out.
	require.Len(t, result["job1"], 1)
	require.Len(t, result["job1"][0].Targets, 0)
	require.Equal(t, "source1", result["job1"][0].Source)
	require.Equal(t, model.LabelSet{"team": "ops"}, result["job1"][0].Labels)
	require.Equal(t, 1, stats.total)
	require.Equal(t, 0, stats.kept)
	require.Equal(t, 1, stats.dropped)
	require.Equal(t, 1, stats.diffZone)
}

func TestFilterTargetsZoneAwarePodNodeName(t *testing.T) {
	// This test verifies the __meta_kubernetes_pod_node_name -> nodeToZone
	// lookup for zone detection. This is the primary strategy for determining
	// target zone in endpoint and endpointslice roles.
	selfPeer := peer.Peer{Name: "self", Addr: "127.0.0.1:12345", Self: true, State: peer.StateParticipant}

	// Target has pod node name — zone resolved via nodeToZone map.
	targetSameNode := model.LabelSet{
		"__address__":   "10.0.0.1:9090",
		metaPodNodeName: "node-1",
	}
	targetDiffNode := model.LabelSet{
		"__address__":   "10.0.0.2:9090",
		metaPodNodeName: "node-2",
	}
	// Target with a node name that is NOT in the nodeToZone map (unknown zone).
	targetUnknownNode := model.LabelSet{
		"__address__":   "10.0.0.3:9090",
		metaPodNodeName: "node-unknown",
	}

	localRing := shard.Ring(512)
	localRing.SetPeers([]peer.Peer{selfPeer})

	keyUnknown := shard.StringKey(nonMetaLabelString(targetUnknownNode))
	c := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer},
		lookupMap: map[shard.Key][]peer.Peer{
			keyUnknown: {selfPeer},
		},
	}

	nodeToZone := map[string]string{
		"node-1": "us-east-1a",
		"node-2": "us-east-1b",
	}

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source:  "source1",
				Targets: []model.LabelSet{targetSameNode, targetDiffNode, targetUnknownNode},
			},
		},
	}

	result, stats := filterTargets(input, c, "us-east-1a", localRing, nodeToZone)

	require.Len(t, result["job1"], 1)
	resultTargets := result["job1"][0].Targets
	// targetSameNode: pod node name -> nodeToZone -> "us-east-1a" (same zone) -> kept
	// targetDiffNode: pod node name -> nodeToZone -> "us-east-1b" (diff zone) -> dropped
	// targetUnknownNode: pod node name not in nodeToZone -> no zone -> global ring -> selfPeer -> kept
	require.Len(t, resultTargets, 2)
	require.Contains(t, resultTargets, targetSameNode)
	require.Contains(t, resultTargets, targetUnknownNode)

	require.Equal(t, 3, stats.total)
	require.Equal(t, 1, stats.sameZone)
	require.Equal(t, 1, stats.diffZone)
	require.Equal(t, 1, stats.noZone)
	require.Equal(t, 2, stats.kept)
	require.Equal(t, 1, stats.dropped)
}

func TestFilterTargetsZoneAwareGroupLabelPodNodeName(t *testing.T) {
	// This test verifies zone detection via pod-node-name from group labels.
	// In the pod role, __meta_kubernetes_pod_node_name is on group.Labels.
	selfPeer := peer.Peer{Name: "self", Addr: "127.0.0.1:12345", Self: true, State: peer.StateParticipant}

	// Targets have NO pod node name themselves — zone comes from group labels.
	targetA := model.LabelSet{"__address__": "10.0.0.1:9090"}
	targetB := model.LabelSet{"__address__": "10.0.0.2:9090"}

	localRing := shard.Ring(512)
	localRing.SetPeers([]peer.Peer{selfPeer})

	c := &testCluster{
		ready: true,
		peers: []peer.Peer{selfPeer},
	}

	nodeToZone := map[string]string{
		"node-1": "us-east-1a",
		"node-2": "us-east-1b",
	}

	input := map[string][]*targetgroup.Group{
		"job1": {
			{
				Source: "source-same-node",
				// Group-level pod node name -> nodeToZone -> same zone
				Labels:  model.LabelSet{metaPodNodeName: "node-1"},
				Targets: []model.LabelSet{targetA},
			},
			{
				Source: "source-diff-node",
				// Group-level pod node name -> nodeToZone -> different zone
				Labels:  model.LabelSet{metaPodNodeName: "node-2"},
				Targets: []model.LabelSet{targetB},
			},
		},
	}

	result, stats := filterTargets(input, c, "us-east-1a", localRing, nodeToZone)

	require.Len(t, result["job1"], 2)
	// First group: group pod node name -> "node-1" -> "us-east-1a" (same zone) -> kept
	require.Len(t, result["job1"][0].Targets, 1)
	require.Contains(t, result["job1"][0].Targets, targetA)
	// Second group: group pod node name -> "node-2" -> "us-east-1b" (diff zone) -> dropped
	require.Len(t, result["job1"][1].Targets, 0)

	require.Equal(t, 2, stats.total)
	require.Equal(t, 1, stats.sameZone)
	require.Equal(t, 1, stats.diffZone)
	require.Equal(t, 1, stats.kept)
	require.Equal(t, 1, stats.dropped)
}

func TestResolveTargetZone(t *testing.T) {
	nodeToZone := map[string]string{
		"node-1": "us-east-1a",
		"node-2": "us-east-1b",
	}

	tests := []struct {
		name        string
		target      model.LabelSet
		groupLabels model.LabelSet
		nodeToZone  map[string]string
		expected    string
	}{
		{
			name:        "no labels at all",
			target:      model.LabelSet{"__address__": "10.0.0.1:9090"},
			groupLabels: model.LabelSet{},
			nodeToZone:  nil,
			expected:    "",
		},
		{
			name: "pod node name on target with nodeToZone",
			target: model.LabelSet{
				"__address__":   "10.0.0.1:9090",
				metaPodNodeName: "node-1",
			},
			groupLabels: model.LabelSet{},
			nodeToZone:  nodeToZone,
			expected:    "us-east-1a",
		},
		{
			name:   "pod node name on group with nodeToZone",
			target: model.LabelSet{"__address__": "10.0.0.1:9090"},
			groupLabels: model.LabelSet{
				metaPodNodeName: "node-2",
			},
			nodeToZone: nodeToZone,
			expected:   "us-east-1b",
		},
		{
			name: "target pod node name takes precedence over group",
			target: model.LabelSet{
				"__address__":   "10.0.0.1:9090",
				metaPodNodeName: "node-1",
			},
			groupLabels: model.LabelSet{
				metaPodNodeName: "node-2",
			},
			nodeToZone: nodeToZone,
			expected:   "us-east-1a", // node-1 -> us-east-1a
		},
		{
			name: "pod node name not in nodeToZone",
			target: model.LabelSet{
				"__address__":   "10.0.0.1:9090",
				metaPodNodeName: "node-unknown",
			},
			groupLabels: model.LabelSet{},
			nodeToZone:  nodeToZone,
			expected:    "",
		},
		{
			name: "skipped when nodeToZone is nil",
			target: model.LabelSet{
				"__address__":   "10.0.0.1:9090",
				metaPodNodeName: "node-1",
			},
			groupLabels: model.LabelSet{},
			nodeToZone:  nil,
			expected:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveTargetZone(tc.target, tc.groupLabels, tc.nodeToZone)
			require.Equal(t, tc.expected, result)
		})
	}
}
