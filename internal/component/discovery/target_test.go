package discovery

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/Masterminds/goutils"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

func TestDecodeMap(t *testing.T) {
	scope := vm.NewScope(map[string]interface{}{
		"foobar": 42,
	})

	input := `{ a = "5", b = "10" }`
	expected := NewTargetFromMap(map[string]string{"a": "5", "b": "10"})

	expr, err := parser.ParseExpression(input)
	require.NoError(t, err)

	eval := vm.New(expr)
	actual := Target{}
	require.NoError(t, eval.Evaluate(scope, &actual))
	require.Equal(t, expected, actual)

	// Test can iterate over it
	var seen []string
	actual.ForEachLabel(func(k string, v string) bool {
		seen = append(seen, fmt.Sprintf("%s=%s", k, v))
		return true
	})
	slices.Sort(seen)
	require.Equal(t, []string{"a=5", "b=10"}, seen)

	// Some loggers print targets out, check it's all good.
	require.Equal(t, `{"a"="5", "b"="10"}`, fmt.Sprintf("%s", actual))
}

func TestConvertFromNative(t *testing.T) {
	var nativeTargets = []model.LabelSet{
		{model.LabelName("hip"): model.LabelValue("hop")},
		{model.LabelName("nae"): model.LabelValue("nae")},
	}

	nativeGroup := &targetgroup.Group{
		Targets: nativeTargets,
		Labels: model.LabelSet{
			model.LabelName("boom"): model.LabelValue("bap"),
		},
		Source: "test",
	}

	expected := []Target{
		NewTargetFromMap(map[string]string{"hip": "hop", "boom": "bap"}),
		NewTargetFromMap(map[string]string{"nae": "nae", "boom": "bap"}),
	}

	require.True(t, equality.DeepEqual(expected, toAlloyTargets(map[string]*targetgroup.Group{"test": nativeGroup})))
}

func TestEquals_Basic(t *testing.T) {
	// NOTE: if we start caching anything as a field, the equality may break. We should test it.
	t1 := NewTargetFromMap(map[string]string{"hip": "hop", "boom": "bap"})
	require.Equal(t, 2, t1.Len())
	tb := NewTargetBuilderFrom(t1)
	tb.Set("boom", "bap")
	t2 := tb.Target()
	// This is a way commonly used in tests.
	require.Equal(t, t1, t2)
	// This is the way exports are compared in BuiltinComponentNode.setExports, and it's important for performance that
	// Targets equality is working correctly.
	require.True(t, reflect.DeepEqual(t1, t2))
}

// TODO(thampiotr): will need a lot more tests like this and with a builder
func TestEquals_Custom(t *testing.T) {
	t1 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"foo": "bar"},
		model.LabelSet{"hip": "hop"},
	)
	t2 := NewTargetFromSpecificAndBaseLabelSet(
		nil,
		model.LabelSet{"hip": "hop", "foo": "bar"},
	)
	require.NotEqual(t, t1, t2)
	require.True(t, t1.Equals(&t2))
	require.True(t, t1.EqualsTarget(&t2))
}

func Benchmark_Targets_TypicalPipeline(b *testing.B) {
	sharedLabels := 5
	labelsPerTarget := 5
	labelsLength := 10
	targetsCount := 20_000
	numPeers := 10

	genLabelSet := func(size int) model.LabelSet {
		ls := model.LabelSet{}
		for i := 0; i < size; i++ {
			name, _ := goutils.RandomAlphaNumeric(labelsLength)
			value, _ := goutils.RandomAlphaNumeric(labelsLength)
			ls[model.LabelName(name)] = model.LabelValue(value)
		}
		return ls
	}

	var labelSets []model.LabelSet
	for i := 0; i < targetsCount; i++ {
		labelSets = append(labelSets, genLabelSet(labelsPerTarget))
	}

	cache := map[string]*targetgroup.Group{}
	cache["test"] = &targetgroup.Group{
		Targets: labelSets,
		Labels:  genLabelSet(sharedLabels),
		Source:  "test",
	}

	peers := make([]peer.Peer, 0, numPeers)
	for i := 0; i < numPeers; i++ {
		peerName := fmt.Sprintf("peer_%d", i)
		peers = append(peers, peer.Peer{Name: peerName, Addr: peerName, Self: i == 0, State: peer.StateParticipant})
	}

	cluster := &randomCluster{
		peers:        peers,
		peersByIndex: make(map[int][]peer.Peer, len(peers)),
	}

	b.ResetTimer()

	var prevDistTargets *DistributedTargets
	for i := 0; i < b.N; i++ {
		// Creating the targets in discovery
		targets := toAlloyTargets(cache)

		// Relabel of targets in discovery.relabel
		for ind := range targets {
			builder := NewTargetBuilderFrom(targets[ind])
			// would do alloy_relabel.ProcessBuilder here to relabel
			targets[ind] = builder.Target()
		}

		// discovery.scrape: distributing targets for clustering
		dt := NewDistributedTargets(true, cluster, targets)
		_ = dt.LocalTargets()
		_ = dt.MovedToRemoteInstance(prevDistTargets)
		// Sending LabelSet to Prometheus library for scraping
		_ = ComponentTargetsToPromTargetGroups("test", targets)

		// Remote write happens on a sample level and largely outside Alloy's codebase, so skipping here.

		prevDistTargets = dt
	}
}

type randomCluster struct {
	peers []peer.Peer
	// stores results in a map to reduce the allocation noise in the benchmark
	peersByIndex map[int][]peer.Peer
}

func (f *randomCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	ind := int(key)
	if ind < 0 {
		ind = -ind
	}
	peerIndex := ind % len(f.peers)
	if _, ok := f.peersByIndex[peerIndex]; !ok {
		f.peersByIndex[peerIndex] = []peer.Peer{f.peers[peerIndex]}
	}
	return f.peersByIndex[peerIndex], nil
}

func (f *randomCluster) Peers() []peer.Peer {
	return f.peers
}
