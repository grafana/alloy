package discovery

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/Masterminds/goutils"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"

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

	// Test can use it like a map
	var seen []string
	actual.ForEachLabel(func(k string, v string) bool {
		seen = append(seen, fmt.Sprintf("%s=%s", k, v))
		return true
	})
	slices.Sort(seen)
	require.Equal(t, []string{"a=5", "b=10"}, seen)

	actual.Set("foo", "bar")
	get, ok := actual.Get("foo")
	require.True(t, ok)
	require.Equal(t, "bar", get)

	actual.Delete("foo")
	get, ok = actual.Get("foo")
	require.False(t, ok)
	require.Equal(t, "", get)

	// Some loggers print targets out, check it's all good. But without caring about order.
	str := fmt.Sprintf("%s", actual)
	valid := str == `{a="5", b="10"}` || str == `{b="10", a="5"}`
	require.True(t, valid)
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

	require.Equal(t, expected, toAlloyTargets(map[string]*targetgroup.Group{"test": nativeGroup}))
}

func TestEquals(t *testing.T) {
	t1 := NewTargetFromMap(map[string]string{"hip": "hop", "boom": "bap"})
	// TODO(thampiotr): if we start caching this as a field, the equality may break.
	require.Equal(t, 2, t1.Labels().Len())
	t2 := NewTargetFromMap(map[string]string{"hip": "hop"})
	t2.Set("boom", "bap")
	// This is the way exports are compared in BuiltinComponentNode.setExports, and it's important for performance that
	// Targets equality is working correctly.
	require.True(t, reflect.DeepEqual(t1, t2))
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
		peers: peers,
	}

	b.ResetTimer()

	var prev *DistributedTargets
	for i := 0; i < b.N; i++ {
		// Creating the targets in discovery
		targets := toAlloyTargets(cache)
		// Relabel of targets in discovery.relabel
		for _, target := range targets {
			l := target.Labels()
			_ = NewTargetFromModelLabels(l)
		}
		// Distributed targets for clustering
		dt := NewDistributedTargets(true, cluster, targets)
		_ = dt.LocalTargets()
		_ = dt.MovedToRemoteInstance(prev)
		// Sending LabelSet to Prometheus library for scraping
		for _, target := range targets {
			_ = target.LabelSet()
		}
		prev = dt
	}
}

type randomCluster struct {
	peers []peer.Peer
}

func (f *randomCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	return []peer.Peer{f.peers[rand.Int()%len(f.peers)]}, nil
}

func (f *randomCluster) Peers() []peer.Peer {
	return f.peers
}
