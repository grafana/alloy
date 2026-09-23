package scrape

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
)

func TestDistributeTargets_ExcludedLabels(t *testing.T) {
	const jobName = "prometheus.scrape.test"
	identity := discovery.NewTargetFromMap(map[string]string{
		"__address__":      "localhost:9090",
		"__metrics_path__": "/metrics/database",
	})
	ownershipKey := shard.Key(identity.NonMetaLabelsHash())

	// Peers discover different signatures for the same endpoints. They must
	// agree on ownership, including after the credentials refresh.
	for _, localOwner := range []bool{true, false} {
		t.Run(fmt.Sprintf("local_owner_%t", localOwner), func(t *testing.T) {
			owner := peer1Self
			owner.Self = localOwner
			c := &Component{
				opts: component.Options{ID: jobName, Logger: util.TestAlloyLogger(t).Slog()},
				cluster: &fakeCluster{
					peers:     []peer.Peer{peer1Self, peer2, peer3},
					lookupMap: map[shard.Key][]peer.Peer{ownershipKey: {owner}},
				},
				targetsGauge:        client.NewGauge(client.GaugeOpts{Name: "test_targets"}),
				movedTargetsCounter: client.NewCounter(client.CounterOpts{Name: "test_moved_targets"}),
			}
			args := testArgs()
			args.Clustering.ExcludedLabels = []string{"__param_sig", "__param_exp"}
			for refresh := range 2 {
				var targets []discovery.Target
				for i := range 2 {
					targets = append(targets, discovery.NewTargetFromMap(map[string]string{
						"__address__":      "localhost:9090",
						"__metrics_path__": "/metrics/database",
						"__param_sig":      fmt.Sprintf("signature-%t-%d-%d", localOwner, refresh, i),
						"__param_exp":      fmt.Sprintf("%d", refresh+1),
						"__meta_source":    fmt.Sprintf("peer-%t", localOwner),
					}))
				}
				groups, moved := c.distributeTargets(targets, jobName, args)
				require.Empty(t, moved)
				require.Equal(t, 2, c.distributedTargets.TargetCount())
				if !localOwner {
					require.Empty(t, groups[jobName])
					require.Zero(t, testutil.ToFloat64(c.targetsGauge))
					continue
				}
				require.Equal(t, discovery.ComponentTargetsToPromTargetGroups(jobName, targets), groups)
				require.Equal(t, float64(2), testutil.ToFloat64(c.targetsGauge))

				// Populate the scrape URLs using the same Prometheus code as the
				// scraper. Excluded credentials must still reach both requests.
				promTargets := c.populatePromLabels(c.distributedTargets.LocalTargets(), jobName, args)
				require.Len(t, promTargets, 2)
				for i, target := range promTargets {
					sig, _ := targets[i].Get("__param_sig")
					exp, _ := targets[i].Get("__param_exp")
					require.Equal(t, "/metrics/database", target.URL().Path)
					require.Equal(t, sig, target.URL().Query().Get("sig"))
					require.Equal(t, exp, target.URL().Query().Get("exp"))
				}
			}

			// A reload removing exclusions restores the full-label ownership
			// keys and still identifies each outgoing target individually.
			if localOwner {
				targets := c.distributedTargets.LocalTargets()
				for _, target := range targets {
					c.cluster.(*fakeCluster).lookupMap[shard.Key(target.NonMetaLabelsHash())] = []peer.Peer{peer2}
				}
				args.Clustering.ExcludedLabels = nil
				groups, moved := c.distributeTargets(targets, jobName, args)
				require.Empty(t, groups[jobName])
				require.Len(t, moved, 2)
				expected := c.populatePromLabels(targets, jobName, args)
				for i, target := range moved {
					require.Equal(t, expected[i].URL(), target.URL())
				}
			}
		})
	}
}

func TestClusteringExclusionsRuntimeUpdate(t *testing.T) {
	endpoint := testTargetWithId(9)
	t.Cleanup(endpoint.Close)
	builder := discovery.NewTargetBuilderFrom(endpoint.Target())
	builder.Set("__param_sig", "signature")
	builder.Set("__param_exp", "expiry")
	target := builder.Target()
	fullKey := shard.Key(target.NonMetaLabelsHash())
	withoutSignature := shard.Key(target.SpecificLabelsHash([]string{"__address__", "__param_exp"}))
	withoutCredentials := shard.Key(endpoint.Target().NonMetaLabelsHash())
	lookups := make(chan shard.Key, 16)
	c := &fakeCluster{
		peers: []peer.Peer{peer1Self, peer2},
		lookupMap: map[shard.Key][]peer.Peer{
			fullKey: {peer1Self}, withoutSignature: {peer2}, withoutCredentials: {peer1Self},
		},
		onLookup: func(key shard.Key) { lookups <- key },
	}
	reg := client.NewRegistry()
	args := testArgs()
	args.Targets = []discovery.Target{target}
	appender := testappender.NewCollectingAppender()
	args.ForwardTo = []storage.Appendable{testappender.ConstantAppendable{Inner: appender}}
	opts := testOptions(t, reg, c)
	promManagerMutex.Lock()
	s, err := New(opts, args)
	promManagerMutex.Unlock()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(testTimeout):
			t.Error("scrape component did not stop")
		}
	})

	var lastScraped int64
	for _, step := range []struct {
		name       string
		excluded   []string
		key        shard.Key
		localCount int
	}{
		{"default", nil, fullKey, 1},
		{"enable exclusions", []string{"__param_sig"}, withoutSignature, 0},
		{"change exclusions", []string{"__param_sig", "__param_exp"}, withoutCredentials, 1},
		{"clear exclusions", nil, fullKey, 1},
	} {
		t.Run(step.name, func(t *testing.T) {
			if step.name != "default" {
				args.Clustering.ExcludedLabels = step.excluded
				require.NoError(t, s.Update(args))
			}
			select {
			case key := <-lookups:
				require.Equal(t, step.key, key)
			case <-time.After(testTimeout):
				t.Fatal("updated targets were not redistributed")
			}
			waitForMetricValue(t, reg, "prometheus_scrape_targets_gauge", float64(step.localCount))
			require.Eventually(t, func() bool {
				return len(s.scraper.TargetsActive()[opts.ID]) == step.localCount
			}, testTimeout, 10*time.Millisecond)
			if step.localCount != 0 {
				require.Eventually(t, func() bool {
					sample := appender.LatestSampleFor(`{__name__="test_counter", instance="9", job="prometheus.scrape.test"}`)
					if sample == nil || sample.Timestamp <= lastScraped || sample.Value != 109 {
						return false
					}
					lastScraped = sample.Timestamp
					return true
				}, testTimeout, 10*time.Millisecond)
			} else {
				require.Never(t, func() bool {
					sample := appender.LatestSampleFor(`{__name__="test_counter", instance="9", job="prometheus.scrape.test"}`)
					return sample == nil || sample.Value != 109
				}, 5*args.ScrapeInterval, 10*time.Millisecond)
			}
		})
	}
}
