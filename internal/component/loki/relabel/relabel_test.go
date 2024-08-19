package relabel

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	lsf "github.com/grafana/alloy/internal/component/loki/source/file"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// Rename the kubernetes_(.*) labels without the suffix and remove them,
// then set the `environment` label to the value of the namespace.
var rc = `rule {
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
       }`

func TestRelabeling(t *testing.T) {
	// Unmarshal the Alloy relabel rules into a custom struct, as we don't have
	// an easy way to refer to a loki.LogsReceiver value for the forward_to
	// argument.
	type cfg struct {
		Rcs []*alloy_relabel.Config `alloy:"rule,block,optional"`
	}
	var relabelConfigs cfg
	err := syntax.Unmarshal([]byte(rc), &relabelConfigs)
	require.NoError(t, err)

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()

	// Create and run the component, so that it relabels and forwards logs.
	opts := component.Options{
		Logger:         util.TestAlloyLogger(t),
		Registerer:     prometheus.NewRegistry(),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	args := Arguments{
		ForwardTo:      []loki.LogsReceiver{ch1, ch2},
		RelabelConfigs: relabelConfigs.Rcs,
		MaxCacheSize:   10,
	}

	c, err := New(opts, args)
	require.NoError(t, err)
	go c.Run(context.Background())

	// Send a log entry to the component's receiver.
	logEntry := loki.Entry{
		Labels: model.LabelSet{"filename": "/var/log/pods/agent/agent/1.log", "kubernetes_namespace": "dev", "kubernetes_pod_name": "agent", "foo": "bar"},
		Entry: logproto.Entry{
			Timestamp: time.Now(),
			Line:      "very important log",
		},
	}

	c.receiver.Chan() <- logEntry

	wantLabelSet := model.LabelSet{
		"filename":    "/var/log/pods/agent/agent/1.log",
		"namespace":   "dev",
		"pod_name":    "agent",
		"environment": "dev",
		"foo":         "bar",
	}

	// The log entry should be received in both channels, with the relabeling
	// rules correctly applied.
	for i := 0; i < 2; i++ {
		select {
		case logEntry := <-ch1.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "very important log", logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case logEntry := <-ch2.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "very important log", logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}
	}
}

func BenchmarkRelabelComponent(b *testing.B) {
	type cfg struct {
		Rcs []*alloy_relabel.Config `alloy:"rule,block,optional"`
	}
	var relabelConfigs cfg
	_ = syntax.Unmarshal([]byte(rc), &relabelConfigs)
	ch1 := loki.NewLogsReceiver()

	// Create and run the component, so that it relabels and forwards logs.
	opts := component.Options{
		Logger:         util.TestAlloyLogger(b),
		Registerer:     prometheus.NewRegistry(),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	args := Arguments{
		ForwardTo:      []loki.LogsReceiver{ch1},
		RelabelConfigs: relabelConfigs.Rcs,
		MaxCacheSize:   500_000,
	}

	c, _ := New(opts, args)
	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)

	var entry loki.Entry
	go func() {
		for e := range ch1.Chan() {
			entry = e
		}
	}()

	now := time.Now()
	for i := 0; i < b.N; i++ {
		c.receiver.Chan() <- loki.Entry{
			Labels: model.LabelSet{"filename": "/var/log/pods/agent/agent/%d.log", "kubernetes_namespace": "dev", "kubernetes_pod_name": model.LabelValue(fmt.Sprintf("agent-%d", i)), "foo": "bar"},
			Entry: logproto.Entry{
				Timestamp: now,
				Line:      "very important log",
			},
		}
	}

	_ = entry
	cancel()
}

func TestCache(t *testing.T) {
	type cfg struct {
		Rcs []*alloy_relabel.Config `alloy:"rule,block,optional"`
	}
	var relabelConfigs cfg
	err := syntax.Unmarshal([]byte(rc), &relabelConfigs)
	require.NoError(t, err)

	ch1 := loki.NewLogsReceiver()

	// Create and run the component, so that it relabels and forwards logs.
	opts := component.Options{
		Logger:         util.TestAlloyLogger(t),
		Registerer:     prometheus.NewRegistry(),
		OnStateChange:  func(e component.Exports) {},
		GetServiceData: getServiceData,
	}
	args := Arguments{
		ForwardTo: []loki.LogsReceiver{ch1},
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

	c, err := New(opts, args)
	require.NoError(t, err)
	go c.Run(context.Background())

	go func() {
		for e := range ch1.Chan() {
			require.Equal(t, "very important log", e.Line)
		}
	}()

	e := getEntry()

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
	c.receiver.Chan() <- e
	e.Labels = lsets[1]
	c.receiver.Chan() <- e
	e.Labels = lsets[2]
	c.receiver.Chan() <- e

	time.Sleep(100 * time.Millisecond)
	// Let's look into the cache's structure now!
	// The cache should have stored each label set by its fingerprint.
	for i := 0; i < 3; i++ {
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
	c.receiver.Chan() <- e
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
	c.receiver.Chan() <- e

	e.Labels = ls2
	c.receiver.Chan() <- e

	time.Sleep(100 * time.Millisecond)
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
	c.receiver.Chan() <- e
	e.Labels = lsets[4]
	c.receiver.Chan() <- e

	require.Equal(t, c.cache.Len(), 4)
	wantKeys := []model.Fingerprint{lsets[0].Fingerprint(), ls1.Fingerprint(), lsets[3].Fingerprint(), lsets[4].Fingerprint()}
	for i, k := range c.cache.Keys() { // Returns the cache keys in LRU order.
		f, ok := k.(model.Fingerprint)
		require.True(t, ok)
		require.Equal(t, f, wantKeys[i])
	}
}

func TestEntrySentToTwoRelabelComponents(t *testing.T) {
	// Set up two different loki.relabel components.
	stg1 := `
forward_to = []
rule {
	action       = "replace"
	target_label = "lbl"
	replacement  = "foo"
}
`
	stg2 := `
forward_to = []
rule {
	action       = "replace"
	target_label = "lbl"
	replacement  = "bar"
}
`

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	var args1, args2 Arguments
	require.NoError(t, syntax.Unmarshal([]byte(stg1), &args1))
	require.NoError(t, syntax.Unmarshal([]byte(stg2), &args2))
	args1.ForwardTo = []loki.LogsReceiver{ch1}
	args2.ForwardTo = []loki.LogsReceiver{ch2}

	// Start the loki.process components.
	tc1, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.relabel")
	require.NoError(t, err)
	tc2, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.relabel")

	require.NoError(t, err)
	go func() {
		err1 := tc1.Run(componenttest.TestContext(t), args1)
		require.NoError(t, err1)
	}()
	go func() {
		err2 := tc2.Run(componenttest.TestContext(t), args2)
		require.NoError(t, err2)
	}()
	require.NoError(t, tc1.WaitExports(time.Second))
	require.NoError(t, tc2.WaitExports(time.Second))

	// Create a file to log to.
	f, err := os.CreateTemp(t.TempDir(), "example")
	require.NoError(t, err)
	defer f.Close()

	// Create and start a component that will read from that file and fan out to both components.
	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.file")
	require.NoError(t, err)

	go func() {
		err := ctrl.Run(context.Background(), lsf.Arguments{
			Targets: []discovery.Target{{"__path__": f.Name(), "somelbl": "somevalue"}},
			ForwardTo: []loki.LogsReceiver{
				tc1.Exports().(Exports).Receiver,
				tc2.Exports().(Exports).Receiver,
			},
		})
		require.NoError(t, err)
	}()
	ctrl.WaitRunning(time.Minute)

	// Write a line to the file.
	_, err = f.Write([]byte("writing some text\n"))
	require.NoError(t, err)

	wantLabelSet := model.LabelSet{
		"filename": model.LabelValue(f.Name()),
		"somelbl":  "somevalue",
	}

	// The two entries have been modified without a race condition.
	for i := 0; i < 2; i++ {
		select {
		case logEntry := <-ch1.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "writing some text", logEntry.Line)
			require.Equal(t, wantLabelSet.Clone().Merge(model.LabelSet{"lbl": "foo"}), logEntry.Labels)
		case logEntry := <-ch2.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "writing some text", logEntry.Line)
			require.Equal(t, wantLabelSet.Clone().Merge(model.LabelSet{"lbl": "bar"}), logEntry.Labels)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}
	}
}

func TestRuleGetter(t *testing.T) {
	// Set up the component Arguments.
	originalCfg := `rule {
         action       = "keep"
		 source_labels = ["__name__"]
         regex        = "up"
       }
		forward_to = []`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(originalCfg), &args))

	// Set up and start the component.
	tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.relabel")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitExports(time.Second))

	// Use the getter to retrieve the original relabeling rules.
	exports := tc.Exports().(Exports)
	gotOriginal := exports.Rules

	// Update the component with new relabeling rules and retrieve them.
	updatedCfg := `rule {
         action       = "drop"
		 source_labels = ["__name__"]
         regex        = "up"
       }
		forward_to = []`
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
}

func getEntry() loki.Entry {
	return loki.Entry{
		Labels: model.LabelSet{},
		Entry: logproto.Entry{
			Timestamp: time.Now(),
			Line:      "very important log",
		},
	}
}

func getServiceData(name string) (interface{}, error) {
	switch name {
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}
