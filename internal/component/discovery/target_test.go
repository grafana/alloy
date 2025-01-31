package discovery

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Masterminds/goutils"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
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

func TestEquals_Custom(t *testing.T) {
	eq1 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"foo": "bar"},
		model.LabelSet{"hip": "hop"},
	)
	eq2 := NewTargetFromSpecificAndBaseLabelSet(
		nil,
		model.LabelSet{"hip": "hop", "foo": "bar"},
	)
	eq3 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"hip": "hop", "foo": "bar"},
		nil,
	)
	ne1 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"foo": "bar"},
		nil,
	)
	ne2 := NewTargetFromSpecificAndBaseLabelSet(
		nil,
		model.LabelSet{"foo": "bar"},
	)

	equalTargets := []Target{eq1, eq2, eq3}
	for _, t1 := range equalTargets {
		for _, t2 := range equalTargets {
			require.True(t, t1.Equals(&t2))
			require.True(t, t1.EqualsTarget(&t2))
			require.True(t, t2.Equals(&t1))
			require.True(t, t2.EqualsTarget(&t1))
		}
	}

	notEqualTargets := []Target{ne1, ne2}
	for _, t1 := range notEqualTargets {
		for _, t2 := range equalTargets {
			require.False(t, t1.Equals(&t2))
			require.False(t, t1.EqualsTarget(&t2))
			require.False(t, t2.Equals(&t1))
			require.False(t, t2.EqualsTarget(&t1))
		}
	}
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

func TestComponentTargetsToPromTargetGroups(t *testing.T) {
	type testTarget struct {
		own   map[string]string
		group map[string]string
	}
	type args struct {
		jobName string
		tgs     []testTarget
	}
	tests := []struct {
		name     string
		args     args
		expected map[string][]*targetgroup.Group
	}{
		{
			name:     "empty targets",
			args:     args{jobName: "job"},
			expected: map[string][]*targetgroup.Group{"job": {}},
		},
		{
			name: "targets all in same group",
			args: args{
				jobName: "job",
				tgs: []testTarget{
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"boom": "bap"}},
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"tiki": "ta"}},
				},
			},
			expected: map[string][]*targetgroup.Group{"job": {
				{
					Source: "job_part_0",
					Labels: mapToLabelSet(map[string]string{"hip": "hop"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"boom": "bap"}),
						mapToLabelSet(map[string]string{"tiki": "ta"}),
					},
				},
			}},
		},
		{
			name: "two groups",
			args: args{
				jobName: "job",
				tgs: []testTarget{
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"boom": "bap"}},
					{group: map[string]string{"kung": "foo"}, own: map[string]string{"tiki": "ta"}},
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"hoo": "rey"}},
					{group: map[string]string{"kung": "foo"}, own: map[string]string{"bibim": "bap"}},
				},
			},
			expected: map[string][]*targetgroup.Group{"job": {
				{
					Source: "job_part_0",
					Labels: mapToLabelSet(map[string]string{"hip": "hop"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"boom": "bap"}),
						mapToLabelSet(map[string]string{"hoo": "rey"}),
					},
				},
				{
					Source: "job_part_1",
					Labels: mapToLabelSet(map[string]string{"kung": "foo"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"tiki": "ta"}),
						mapToLabelSet(map[string]string{"bibim": "bap"}),
					},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := make([]Target, 0, len(tt.args.tgs))
			for _, tg := range tt.args.tgs {
				targets = append(targets, NewTargetFromSpecificAndBaseLabelSet(mapToLabelSet(tg.own), mapToLabelSet(tg.group)))
			}
			actual := ComponentTargetsToPromTargetGroups(tt.args.jobName, targets)
			assert.Contains(t, actual, tt.args.jobName)
			slices.SortFunc(actual[tt.args.jobName], func(a *targetgroup.Group, b *targetgroup.Group) int {
				return strings.Compare(a.Source, b.Source)
			})
			assert.Equal(t, tt.expected, actual, "ComponentTargetsToPromTargetGroups(%v, %v)", tt.args.jobName, tt.args.tgs)
		})
	}
}

func mapToLabelSet(m map[string]string) model.LabelSet {
	r := make(model.LabelSet, len(m))
	for k, v := range m {
		r[model.LabelName(k)] = model.LabelValue(v)
	}
	return r
}
