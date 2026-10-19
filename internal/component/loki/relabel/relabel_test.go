package relabel

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/lokitest"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
)

func TestComponent(t *testing.T) {
	type testCase struct {
		name       string
		rules      []*alloy_relabel.Config
		input      loki.Entry
		wantLabels model.LabelSet
		dropped    bool
	}

	newRule := func(configure func(*alloy_relabel.Config)) *alloy_relabel.Config {
		rule := &alloy_relabel.Config{}
		rule.SetToDefault()
		configure(rule)
		return rule
	}

	now := time.Now()

	tests := []testCase{
		{
			name: "relabels and forwards entries",
			rules: []*alloy_relabel.Config{
				newRule(func(rule *alloy_relabel.Config) {
					rule.Regex = alloy_relabel.Regexp(relabel.MustNewRegexp("kubernetes_(.*)"))
					rule.Replacement = "$1"
					rule.Action = alloy_relabel.LabelMap
				}),
				newRule(func(rule *alloy_relabel.Config) {
					rule.Regex = alloy_relabel.Regexp(relabel.MustNewRegexp("kubernetes_(.*)"))
					rule.Action = alloy_relabel.LabelDrop
				}),
				newRule(func(rule *alloy_relabel.Config) {
					rule.SourceLabels = []string{"namespace"}
					rule.TargetLabel = "environment"
					rule.Action = alloy_relabel.Replace
				}),
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"filename":             "/var/log/pods/agent/agent/1.log",
					"kubernetes_namespace": "dev",
					"kubernetes_pod_name":  "agent",
					"foo":                  "bar",
				},
				Entry: push.Entry{
					Timestamp: now,
					Line:      "very important log",
				},
			},
			wantLabels: model.LabelSet{
				"filename":    "/var/log/pods/agent/agent/1.log",
				"namespace":   "dev",
				"pod_name":    "agent",
				"environment": "dev",
				"foo":         "bar",
			},
		},
		{
			name: "drops entries with drop rule",
			rules: []*alloy_relabel.Config{
				newRule(func(rule *alloy_relabel.Config) {
					rule.SourceLabels = []string{"foo"}
					rule.Regex = alloy_relabel.Regexp(relabel.MustNewRegexp("bar"))
					rule.Action = alloy_relabel.Drop
				}),
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"filename": "/var/log/pods/agent/agent/1.log",
					"foo":      "bar",
				},
				Entry: push.Entry{
					Timestamp: now,
					Line:      "very important log",
				},
			},
			dropped: true,
		},
		{
			name: "drops entries when all labels are removed",
			rules: []*alloy_relabel.Config{
				newRule(func(rule *alloy_relabel.Config) {
					rule.Regex = alloy_relabel.Regexp(relabel.MustNewRegexp(".*"))
					rule.Action = alloy_relabel.LabelDrop
				}),
			},
			input: loki.Entry{
				Labels: model.LabelSet{
					"filename": "/var/log/pods/agent/agent/1.log",
					"foo":      "bar",
				},
				Entry: push.Entry{
					Timestamp: now,
					Line:      "very important log",
				},
			},
			dropped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lokitest.RunWithPipelines(t, func(t *testing.T, p lokitest.Pipeline) {
				recv1, recv2 := p.NewReceiver(t), p.NewReceiver(t)
				args := Arguments{
					ForwardTo:      []loki.LogsReceiver{recv1, recv2},
					RelabelConfigs: tt.rules,
					MaxCacheSize:   10,
				}

				ctrl, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.relabel")
				require.NoError(t, err)
				go func() {
					assert.NoError(t, ctrl.Run(t.Context(), args, p.Opts))
				}()
				require.NoError(t, ctrl.WaitExports(time.Second))

				p.Send(t, ctrl.Exports().(Exports).Receiver, tt.input)
				if tt.dropped {
					recv1.NoNext(t, 1*time.Second)
					recv2.NoNext(t, 1*time.Second)
					return
				}

				received1 := recv1.Next(t, 1)
				require.Equal(t, now, received1[0].Timestamp)
				require.Equal(t, tt.input.Line, received1[0].Line)
				require.Equal(t, tt.wantLabels, received1[0].Labels)

				received2 := recv2.Next(t, 1)
				require.Equal(t, now, received2[0].Timestamp)
				require.Equal(t, tt.input.Line, received2[0].Line)
				require.Equal(t, tt.wantLabels, received2[0].Labels)
			})
		})
	}
}

func TestCache(t *testing.T) {
	lokitest.RunWithPipelines(t, func(t *testing.T, p lokitest.Pipeline) {
		recv1 := p.NewReceiver(t)

		// Create and run the component, so that it relabels and forwards logs.
		args := Arguments{
			ForwardTo: []loki.LogsReceiver{recv1},
			RelabelConfigs: []*alloy_relabel.Config{
				{
					SourceLabels: []string{"name", "A"},
					Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),

					Action:      "replace",
					TargetLabel: "env",
					Replacement: "staging",
				}},
			MaxCacheSize: 4,
		}

		ctrl, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.relabel")
		require.NoError(t, err)
		go func() {
			assert.NoError(t, ctrl.Run(t.Context(), args, p.Opts))
		}()
		require.NoError(t, ctrl.WaitExports(time.Second))

		inner, err := ctrl.GetComponent()
		require.NoError(t, err)
		c := inner.(*Component)
		recv := ctrl.Exports().(Exports).Receiver

		e := loki.NewEntry(model.LabelSet{}, push.Entry{
			Timestamp: time.Now(),
			Line:      "very important log",
		})

		lsets := []model.LabelSet{
			{"name": "foo"},
			{"name": "bar"},
			{"name": "baz"},
			{"name": "qux"},
			{"name": "xyz"},
		}
		rlsets := []model.LabelSet{
			{"env": "staging", "name": "foo"},
			{"env": "staging", "name": "bar"},
			{"env": "staging", "name": "baz"},
			{"env": "staging", "name": "qux"},
			{"env": "staging", "name": "xyz"},
		}
		// Send three entries with different label sets along the receiver.
		e.Labels = lsets[0]
		p.Send(t, recv, e)
		e.Labels = lsets[1]
		p.Send(t, recv, e)
		e.Labels = lsets[2]
		p.Send(t, recv, e)

		recv1.Next(t, 3)

		// Let's look into the cache's structure now!
		// The cache should have stored each label set by its fingerprint.
		for i := range 3 {
			val, ok := c.cache.Get(lsets[i].Fingerprint())
			require.True(t, ok)
			cached, ok := val.([]cacheItem)
			require.True(t, ok)

			// Each cache value should be a 1-item slice, with the correct initial
			// and relabeled values applied to it.
			require.Len(t, cached, 1)
			require.Equal(t, cached[0].original, lsets[i])
			require.Equal(t, cached[0].relabeled, rlsets[i])
		}

		// Let's send over an entry we've seen before.
		// We should've hit the cached path, with no changes to the cache's length
		// or the underlying stored value.
		e.Labels = lsets[0]
		p.Send(t, recv, e)
		recv1.Next(t, 1)

		require.Equal(t, c.cache.Len(), 3)
		val, _ := c.cache.Get(lsets[0].Fingerprint())
		cachedVal := val.([]cacheItem)
		require.Len(t, cachedVal, 1)
		require.Equal(t, cachedVal[0].original, lsets[0])
		require.Equal(t, cachedVal[0].relabeled, rlsets[0])

		// Now, let's try to hit a collision.
		// These LabelSets are known to collide (string: 8746e5b6c5f0fb60)
		// https://github.com/pstibrany/fnv-1a-64bit-collisions
		ls1 := model.LabelSet{"A": "K6sjsNNczPl"}
		ls2 := model.LabelSet{"A": "cswpLMIZpwt"}
		envls := model.LabelSet{"env": "staging"}
		require.Equal(t, ls1.Fingerprint(), ls2.Fingerprint(), "expected labelset fingerprints to collide; have we changed the hashing algorithm?")

		e.Labels = ls1
		p.Send(t, recv, e)
		e.Labels = ls2
		p.Send(t, recv, e)
		recv1.Next(t, 2)

		// Both of these should be under a single, new cache key which will contain
		// both entries.
		require.Equal(t, c.cache.Len(), 4)
		val, ok := c.cache.Get(ls1.Fingerprint())
		require.True(t, ok)
		cachedVal = val.([]cacheItem)
		require.Len(t, cachedVal, 2)

		require.Equal(t, cachedVal[0].original, ls1)
		require.Equal(t, cachedVal[1].original, ls2)
		require.Equal(t, cachedVal[0].relabeled, ls1.Merge(envls))
		require.Equal(t, cachedVal[1].relabeled, ls2.Merge(envls))

		// Finally, send two more entries, which should fill up the cache and evict
		// the Least Recently Used items (lsets[1], and lsets[2]).
		e.Labels = lsets[3]
		p.Send(t, recv, e)
		e.Labels = lsets[4]
		p.Send(t, recv, e)
		recv1.Next(t, 2)

		require.Equal(t, c.cache.Len(), 4)
		wantKeys := []model.Fingerprint{lsets[0].Fingerprint(), ls1.Fingerprint(), lsets[3].Fingerprint(), lsets[4].Fingerprint()}
		actualKeys := make([]model.Fingerprint, 0, len(wantKeys))
		for _, k := range c.cache.Keys() { // Returns the cache keys in LRU order.
			f, ok := k.(model.Fingerprint)
			require.True(t, ok)
			actualKeys = append(actualKeys, f)
		}
		require.Equal(t, wantKeys, actualKeys)
	})
}

func TestEntrySentToTwoRelabelComponents(t *testing.T) {
	// Set up two different loki.relabel components.
	cfg1 := `
forward_to = []
rule {
	action       = "replace"
	target_label = "lbl"
	replacement  = "foo"
}
`
	cfg2 := `
forward_to = []
rule {
	action       = "replace"
	target_label = "lbl"
	replacement  = "bar"
}
`

	lokitest.RunWithPipelines(t, func(t *testing.T, p lokitest.Pipeline) {
		recv1, recv2 := p.NewReceiver(t), p.NewReceiver(t)
		var args1, args2 Arguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg1), &args1))
		require.NoError(t, syntax.Unmarshal([]byte(cfg2), &args2))
		args1.ForwardTo = []loki.LogsReceiver{recv1}
		args2.ForwardTo = []loki.LogsReceiver{recv2}

		// Start the loki.relabel components.
		tc1, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.relabel")
		require.NoError(t, err)
		tc2, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.relabel")
		require.NoError(t, err)

		go func() { assert.NoError(t, tc1.Run(t.Context(), args1, p.Opts)) }()
		go func() { assert.NoError(t, tc2.Run(t.Context(), args2, p.Opts)) }()
		require.NoError(t, tc1.WaitExports(time.Second))
		require.NoError(t, tc2.WaitExports(time.Second))

		now := time.Now()
		e := loki.NewEntry(model.LabelSet{"filename": "/tmp/app.log", "somelbl": "somevalue"}, push.Entry{
			Timestamp: now,
			Line:      "writing some text",
		})
		p.Send(t, tc1.Exports().(Exports).Receiver, e)
		p.Send(t, tc2.Exports().(Exports).Receiver, e)

		received1 := recv1.Next(t, 1)
		require.Equal(t, now, received1[0].Timestamp)
		require.Equal(t, "writing some text", received1[0].Line)
		require.Equal(t, model.LabelSet{
			"filename": "/tmp/app.log",
			"somelbl":  "somevalue",
			"lbl":      "foo",
		}, received1[0].Labels)

		received2 := recv2.Next(t, 1)
		require.Equal(t, now, received2[0].Timestamp)
		require.Equal(t, "writing some text", received2[0].Line)
		require.Equal(t, model.LabelSet{
			"filename": "/tmp/app.log",
			"somelbl":  "somevalue",
			"lbl":      "bar",
		}, received2[0].Labels)
	})
}

func TestRuleGetter(t *testing.T) {
	// Set up the component Arguments.
	originalCfg := `rule {
         action       = "keep"
		 source_labels = ["__name__"]
         regex        = "up"
       }
		forward_to = []`

	updatedCfg := `rule {
         action       = "drop"
		 source_labels = ["__name__"]
         regex        = "up"
       }
		forward_to = []`

	lokitest.RunWithPipelines(t, func(t *testing.T, p lokitest.Pipeline) {
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(originalCfg), &args))

		// Set up and start the component.
		tc, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.relabel")
		require.NoError(t, err)
		go func() {
			assert.NoError(t, tc.Run(t.Context(), args, p.Opts))
		}()
		require.NoError(t, tc.WaitExports(time.Second))

		// Use the getter to retrieve the original relabeling rules.
		exports := tc.Exports().(Exports)
		gotOriginal := exports.Rules

		// Update the component with new relabeling rules and retrieve them.
		require.NoError(t, syntax.Unmarshal([]byte(updatedCfg), &args))

		require.NoError(t, tc.Update(args))
		exports = tc.Exports().(Exports)
		gotUpdated := exports.Rules

		require.NotEqual(t, gotOriginal, gotUpdated)
		require.Len(t, gotOriginal, 1)
		require.Len(t, gotUpdated, 1)

		require.Equal(t, gotOriginal[0].Action, alloy_relabel.Keep)
		require.Equal(t, gotUpdated[0].Action, alloy_relabel.Drop)
		require.Equal(t, gotUpdated[0].SourceLabels, gotOriginal[0].SourceLabels)
		require.Equal(t, gotUpdated[0].Regex, gotOriginal[0].Regex)
	})
}

func BenchmarkComponent(b *testing.B) {
	// Rename the kubernetes_(.*) labels without the suffix and remove them,
	// then set the `environment` label to the value of the namespace.
	const rc = `rule {
         regex        = "kubernetes_(.*)"
         replacement  = "$1"
         action       = "labelmap"
       }
       rule {
         regex  = "kubernetes_(.*)"
         action = "labeldrop"
       }
       rule {
         source_labels = ["namespace"]
         target_label  = "environment"
         action        = "replace"
       }
`
	type cfg struct {
		Rcs []*alloy_relabel.Config `alloy:"rule,block,optional"`
	}
	var relabelConfigs cfg
	require.NoError(b, syntax.Unmarshal([]byte(rc), &relabelConfigs))

	setup := func(b *testing.B, p lokitest.Pipeline, forwardTo loki.LogsReceiver) loki.LogsReceiver {
		args := Arguments{
			ForwardTo:      []loki.LogsReceiver{forwardTo},
			RelabelConfigs: relabelConfigs.Rcs,
			MaxCacheSize:   500_000,
		}

		ctrl, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.relabel")
		require.NoError(b, err)
		go func() {
			assert.NoError(b, ctrl.Run(b.Context(), args, p.Opts))
		}()
		require.NoError(b, ctrl.WaitExports(time.Second))
		return ctrl.Exports().(Exports).Receiver
	}

	newBatches := func(workers int) []loki.Batch {
		const (
			streamsPerBatch  = 10
			entriesPerStream = 50
		)

		now := time.Now()
		batches := make([]loki.Batch, workers)
		for w := range batches {
			for i := range streamsPerBatch {
				entries := make([]push.Entry, entriesPerStream)
				for j := range entries {
					entries[j] = push.Entry{Timestamp: now, Line: "very important log"}
				}
				batches[w].Add(loki.NewStream(
					model.LabelSet{
						"filename":             "/var/log/pods/agent/agent/1.log",
						"kubernetes_namespace": "dev",
						"kubernetes_pod_name":  model.LabelValue(fmt.Sprintf("agent-%d-%d", w, i)),
						"foo":                  "bar",
					},
					entries...,
				))
			}
		}
		return batches
	}

	b.Run("single sender", func(b *testing.B) {
		lokitest.BenchmarkWithPipelines(b, newBatches(1), setup)
	})

	b.Run("multiple senders", func(b *testing.B) {
		lokitest.BenchmarkWithPipelines(b, newBatches(8), setup)
	})
}
