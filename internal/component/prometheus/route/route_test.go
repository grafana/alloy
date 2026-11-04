package route

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestArguments(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		valid        bool
	}{
		{"no routes", `default_forward_to = []`, true},
		{"unnamed route", `default_forward_to = []
route {
  selector = "up"
  forward_to = [null]
}`, true},
		{"missing default", ``, false},
		{"missing selector", `default_forward_to = []
route {
  forward_to = [null]
}`, false},
		{"missing destinations", `default_forward_to = []
route {
  selector = "up"
}`, false},
		{"empty destinations", `default_forward_to = []
route {
  selector = "up"
  forward_to = []
}`, false},
		{"named route", `default_forward_to = []
route "name" {
  selector = "up"
  forward_to = [null]
}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.config), &args)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestSelectors(t *testing.T) {
	l := labels.FromStrings("__name__", "requests_total", "env", "prod", "team", "payments")
	for _, tc := range []struct {
		selector string
		match    bool
	}{
		{`requests_total`, true},
		{`requests_total{env="prod",team=~"payments|billing"}`, true},
		{`{__name__=~"requests_.*"}`, true},
		{`{env="dev"}`, false},
		{`{env!="dev"}`, true},
		{`{team!~"payments|billing"}`, false},
		{`{team=~"pay"}`, false}, // Regular expressions are fully anchored.
		{`{missing=""}`, true},
		{`{missing!=""}`, false},
		{`{missing!="prod"}`, true},
		{`{missing=~".*"}`, true},
		{`{missing!~".*"}`, false},
		{`{env="prod",team="billing"}`, false},
		{`{}`, true},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			cfg, err := compile(Arguments{Routes: []Route{{Selector: tc.selector, ForwardTo: destinations(appenders.Noop{})}}})
			require.NoError(t, err)
			require.Equal(t, tc.match, cfg.routes[0].selector.Matches(l))
		})
	}
	for _, selector := range []string{"", " ", `{env=}`, `{env=~"["}`, `rate(requests_total[5m])`, `requests_total > 0`, `up or down`, `up[5m]`, `up offset 5m`, `{env="prod"} |= "error"`} {
		t.Run("invalid/"+selector, func(t *testing.T) {
			err := (Arguments{Routes: []Route{{Selector: selector, ForwardTo: destinations(appenders.Noop{})}}}).Validate()
			require.ErrorContains(t, err, "route 1: invalid selector")
		})
	}
}

func TestRouting(t *testing.T) {
	for _, useLabelStore := range []bool{false, true} {
		t.Run(fmt.Sprintf("labelstore=%t", useLabelStore), func(t *testing.T) {
			a, b, fallback := &recordingReceiver{}, &recordingReceiver{}, &recordingReceiver{}
			c, reg, _ := newComponent(t, useLabelStore, Arguments{
				DefaultForwardTo: destinations(fallback),
				Routes: []Route{
					{Selector: `{tenant="a"}`, ForwardTo: destinations(a, a)},
					{Selector: `{priority="critical"}`, ForwardTo: destinations(a, b)},
				},
			})
			app := c.receiver.Appender(t.Context())
			both := labels.FromStrings("__name__", "requests_total", "tenant", "a", "priority", "critical")
			other := labels.FromStrings("__name__", "requests_total", "tenant", "b")
			_, err := app.Append(0, both, 100, 7)
			require.NoError(t, err)
			_, err = app.Append(0, other, 100, 9)
			require.NoError(t, err)
			require.NoError(t, app.Commit())
			require.Len(t, a.apps, 3)
			require.Len(t, b.apps, 1)
			require.Len(t, fallback.apps, 1)
			for _, dest := range []*recordingReceiver{a, b} {
				for _, child := range dest.apps {
					require.Equal(t, 1, child.commits)
					require.Len(t, child.events, 1)
					require.True(t, labels.Equal(both, child.events[0].labels))
					require.Equal(t, float64(7), child.events[0].v)
				}
			}
			require.True(t, labels.Equal(other, fallback.apps[0].events[0].labels))
			require.Equal(t, float64(2), testutil.ToFloat64(c.processed))
			require.Equal(t, float64(5), testutil.ToFloat64(c.forwarded))
			require.Zero(t, testutil.ToFloat64(c.dropped))
			count, err := testutil.GatherAndCount(reg,
				"alloy_prometheus_route_samples_processed_total",
				"alloy_prometheus_route_samples_forwarded_total",
				"alloy_prometheus_route_samples_dropped_total",
			)
			require.NoError(t, err)
			require.Equal(t, 3, count)
		})
	}
}

func TestNoRoutesAndUnavailableReceivers(t *testing.T) {
	for _, tc := range []struct {
		name     string
		routes   []Route
		fallback bool
		want     int
	}{
		{name: "default only", fallback: true, want: 1},
		{name: "drop everything"},
		{name: "unavailable matching receiver suppresses fallback", routes: []Route{{Selector: "up", ForwardTo: destinations(nil)}}, fallback: true},
		{name: "unmatched route uses fallback", routes: []Route{{Selector: "down", ForwardTo: destinations(nil)}}, fallback: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := &recordingReceiver{}
			args := Arguments{Routes: tc.routes}
			if tc.fallback {
				args.DefaultForwardTo = destinations(nil, dest)
			}
			c, _, _ := newComponent(t, false, args)
			app := c.receiver.Appender(t.Context())
			_, err := app.Append(0, labels.FromStrings("__name__", "up"), 100, 1)
			require.NoError(t, err)
			require.NoError(t, app.Commit())
			require.Len(t, dest.apps, tc.want)
			require.Equal(t, float64(1-tc.want), testutil.ToFloat64(c.dropped))
		})
	}
}

func TestAllAppenderMethods(t *testing.T) {
	l := labels.FromStrings("__name__", "requests_total", "tenant", "a")
	h := &histogram.Histogram{Count: 1, Sum: 2}
	fh := &histogram.FloatHistogram{Count: 1, Sum: 2}
	e := exemplar.Exemplar{Labels: labels.FromStrings("tenant", "other", "trace_id", "abc"), Value: 2, Ts: 10, HasTs: true}
	m := metadata.Metadata{Type: "counter", Help: "Requests", Unit: "requests"}
	for _, useLabelStore := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			send func(storage.Appender, labels.Labels) error
			want event
		}{
			{"sample", func(a storage.Appender, l labels.Labels) error { _, err := a.Append(0, l, 10, 2); return err }, event{kind: "sample", t: 10, v: 2}},
			{"histogram", func(a storage.Appender, l labels.Labels) error {
				_, err := a.AppendHistogram(0, l, 10, h, nil)
				return err
			}, event{kind: "histogram", t: 10, h: h}},
			{"float histogram", func(a storage.Appender, l labels.Labels) error {
				_, err := a.AppendHistogram(0, l, 10, nil, fh)
				return err
			}, event{kind: "histogram", t: 10, fh: fh}},
			{"exemplar", func(a storage.Appender, l labels.Labels) error { _, err := a.AppendExemplar(0, l, e); return err }, event{kind: "exemplar", exemplar: e}},
			{"metadata", func(a storage.Appender, l labels.Labels) error { _, err := a.UpdateMetadata(0, l, m); return err }, event{kind: "metadata", metadata: m}},
			{"start timestamp", func(a storage.Appender, l labels.Labels) error {
				_, err := a.AppendSTZeroSample(0, l, 10, 5)
				return err
			}, event{kind: "start", t: 10, st: 5}},
			{"histogram start timestamp", func(a storage.Appender, l labels.Labels) error {
				_, err := a.AppendHistogramSTZeroSample(0, l, 10, 5, h, nil)
				return err
			}, event{kind: "histogram_start", t: 10, st: 5, h: h}},
			{"float histogram start timestamp", func(a storage.Appender, l labels.Labels) error {
				_, err := a.AppendHistogramSTZeroSample(0, l, 10, 5, nil, fh)
				return err
			}, event{kind: "histogram_start", t: 10, st: 5, fh: fh}},
		} {
			t.Run(fmt.Sprintf("%s/labelstore=%t", tc.name, useLabelStore), func(t *testing.T) {
				dest, fallback := &recordingReceiver{}, &recordingReceiver{}
				c, _, ls := newComponent(t, useLabelStore, Arguments{
					Routes:           []Route{{Selector: `{tenant="a"}`, ForwardTo: destinations(dest)}},
					DefaultForwardTo: destinations(fallback),
				})
				app := c.receiver.Appender(t.Context())
				require.NoError(t, tc.send(app, l))
				require.NoError(t, app.Commit())
				require.Empty(t, fallback.apps)
				require.Len(t, dest.apps, 1)
				want := tc.want
				want.labels = l
				if useLabelStore {
					want.ref = storage.SeriesRef(ls.GetOrAddGlobalRefID(l))
				}
				require.Equal(t, []event{want}, dest.apps[0].events)

				// The same method must use the default for unmatched series.
				other := labels.FromStrings("__name__", "requests_total", "tenant", "b")
				app = c.receiver.Appender(t.Context())
				require.NoError(t, tc.send(app, other))
				require.NoError(t, app.Commit())
				require.Len(t, fallback.apps, 1)
				require.True(t, labels.Equal(other, fallback.apps[0].events[0].labels))
			})
		}
	}
}

func TestStaleMarkers(t *testing.T) {
	dest := &recordingReceiver{}
	c, _, _ := newComponent(t, true, Arguments{Routes: []Route{{Selector: "up", ForwardTo: destinations(dest)}}})
	app := c.receiver.Appender(t.Context())
	_, err := app.Append(0, labels.FromStrings("__name__", "up"), 100, math.Float64frombits(value.StaleNaN))
	require.NoError(t, err)
	require.NoError(t, app.Commit())
	require.True(t, value.IsStaleNaN(dest.apps[0].events[0].v))
	require.NoError(t, c.Update(Arguments{}))
	require.Len(t, dest.apps[0].events, 1, "updates must not synthesize stale markers")
}

func TestUpdateSnapshotAndReferences(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("labelstore=%t", enabled), func(t *testing.T) {
			a, b := &recordingReceiver{}, &recordingReceiver{}
			args := Arguments{Routes: []Route{{Selector: "up", ForwardTo: destinations(a)}}}
			c, _, ls := newComponent(t, enabled, args)
			receiver := c.receiver
			l := labels.FromStrings("__name__", "up")
			old := receiver.Appender(t.Context())
			ref, err := old.Append(0, l, 100, 1)
			require.NoError(t, err)
			if enabled {
				require.Equal(t, storage.SeriesRef(ls.GetOrAddGlobalRefID(l)), ref)
			} else {
				require.NotZero(t, ref, "scrape needs a reference to track staleness")
				require.NotEqual(t, storage.SeriesRef(42), ref, "a local downstream ref must not escape the router")
			}
			// Also retain a transaction whose first append happens after the update.
			pending := receiver.Appender(t.Context())
			args.Routes[0].ForwardTo[0] = b
			require.NoError(t, c.Update(args))
			require.Same(t, receiver, c.receiver)
			_, err = old.Append(ref, l, 200, 2)
			require.NoError(t, err)
			require.NoError(t, old.Commit())
			_, err = pending.Append(ref, l, 300, 3)
			require.NoError(t, err)
			require.NoError(t, pending.Commit())

			fresh := receiver.Appender(t.Context())
			_, err = fresh.Append(ref, l, 400, 4)
			require.NoError(t, err)
			require.NoError(t, fresh.Commit())
			require.Len(t, a.apps, 2)
			require.Len(t, a.apps[0].events, 2)
			require.Len(t, b.apps, 1)
			if enabled {
				require.Equal(t, ref, b.apps[0].events[0].ref)
			} else {
				require.Zero(t, b.apps[0].events[0].ref, "router-local refs must never reach downstream receivers")
			}

			args.Routes[0].Selector = "rate(up[5m])"
			require.Error(t, c.Update(args))
			fresh = receiver.Appender(t.Context())
			_, err = fresh.Append(ref, l, 500, 5)
			require.NoError(t, err)
			require.NoError(t, fresh.Commit())
			require.Len(t, b.apps, 2, "invalid updates must retain the last valid configuration")
		})
	}
}

func TestRouterLocalReferences(t *testing.T) {
	a, b := &recordingReceiver{}, &recordingReceiver{}
	c, _, _ := newComponent(t, false, Arguments{
		Routes:           []Route{{Selector: `{tenant="a"}`, ForwardTo: destinations(a)}},
		DefaultForwardTo: destinations(b),
	})
	labelsA := labels.FromStrings("__name__", "requests_total", "tenant", "a")
	labelsB := labels.FromStrings("__name__", "requests_total", "tenant", "b")
	app := c.receiver.Appender(t.Context())
	refA, err := app.Append(0, labelsA, 100, 1)
	require.NoError(t, err)
	refB, err := app.Append(0, labelsB, 100, 2)
	require.NoError(t, err)
	require.NotZero(t, refA)
	require.NotZero(t, refB)
	require.NotEqual(t, refA, refB, "colliding child refs must not collide in the upstream scrape cache")
	require.NoError(t, app.Commit())

	app = c.receiver.Appender(t.Context())
	ref, err := app.Append(refA, labelsA, 200, 3)
	require.NoError(t, err)
	require.Equal(t, refA, ref)
	ref, err = app.Append(refB, labelsB, 200, 4)
	require.NoError(t, err)
	require.Equal(t, refB, ref)
	require.NoError(t, app.Commit())
	for _, receiver := range []*recordingReceiver{a, b} {
		for _, child := range receiver.apps {
			require.Zero(t, child.events[0].ref)
		}
	}
}

func TestErrorsAndTransactions(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback=%t", rollback), func(t *testing.T) {
			errA, errB := errors.New("destination a"), errors.New("destination b")
			a := &recordingReceiver{sendErr: errA, finishErr: errA}
			b := &recordingReceiver{sendErr: errB, finishErr: errB}
			good, unused := &recordingReceiver{}, &recordingReceiver{finishErr: errors.New("unused destination")}
			c, _, _ := newComponent(t, false, Arguments{
				Routes: []Route{
					{Selector: "up", ForwardTo: destinations(a, good)},
					{Selector: "up", ForwardTo: destinations(b)},
					{Selector: "down", ForwardTo: destinations(unused)},
				},
				DefaultForwardTo: destinations(unused),
			})
			app := c.receiver.Appender(t.Context())
			_, err := app.Append(0, labels.FromStrings("__name__", "up"), 100, 1)
			require.ErrorIs(t, err, errA)
			require.ErrorIs(t, err, errB)
			require.Len(t, good.apps, 1, "a failed destination must not prevent other matching routes")
			require.Empty(t, unused.apps, "a failure must not select the default destination")
			if rollback {
				err = app.Rollback()
			} else {
				err = app.Commit()
			}
			require.ErrorIs(t, err, errA)
			require.ErrorIs(t, err, errB)
			for _, receiver := range []*recordingReceiver{a, b, good} {
				child := receiver.apps[0]
				if rollback {
					require.Equal(t, 1, child.rollbacks)
					require.Zero(t, child.commits)
				} else {
					require.Equal(t, 1, child.commits)
					require.Zero(t, child.rollbacks)
				}
			}
		})
	}
}

func TestOptionsAndContext(t *testing.T) {
	a, b := &recordingReceiver{}, &recordingReceiver{}
	c, _, _ := newComponent(t, false, Arguments{
		Routes:           []Route{{Selector: "up", ForwardTo: destinations(a)}},
		DefaultForwardTo: destinations(b),
	})
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "preserved")
	app := c.receiver.Appender(ctx)
	opts := &storage.AppendOptions{DiscardOutOfOrder: true}
	app.SetOptions(opts)
	opts.DiscardOutOfOrder = false // SetOptions must not retain the caller's pointer.
	_, err := app.Append(0, labels.FromStrings("__name__", "up"), 100, 1)
	require.NoError(t, err)
	require.True(t, a.apps[0].options[0].DiscardOutOfOrder)
	app.SetOptions(opts)
	_, err = app.Append(0, labels.FromStrings("__name__", "other"), 100, 1)
	require.NoError(t, err)
	require.Len(t, a.apps[0].options, 2)
	require.False(t, a.apps[0].options[1].DiscardOutOfOrder)
	require.False(t, b.apps[0].options[0].DiscardOutOfOrder)
	for _, receiver := range []*recordingReceiver{a, b} {
		require.Equal(t, "preserved", receiver.contexts[0].Value(contextKey{}))
		target, ok := scrape.TargetFromContext(receiver.contexts[0])
		require.True(t, ok)
		require.NotNil(t, target)
		store, ok := scrape.MetricMetadataStoreFromContext(receiver.contexts[0])
		require.True(t, ok)
		require.NotNil(t, store)
	}
	require.NoError(t, app.Commit())
}

func TestConcurrentUpdates(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("labelstore=%t", enabled), func(t *testing.T) {
			c, _, _ := newComponent(t, enabled, Arguments{})
			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {
					for range 100 {
						app := c.receiver.Appender(t.Context())
						_, err := app.Append(0, labels.FromStrings("__name__", "up"), 100, 1)
						if err != nil {
							t.Error(err)
						}
						if err := app.Commit(); err != nil {
							t.Error(err)
						}
					}
				})
			}
			for range 100 {
				require.NoError(t, c.Update(Arguments{Routes: []Route{{Selector: "up", ForwardTo: destinations(appenders.Noop{})}}}))
				require.NoError(t, c.Update(Arguments{DefaultForwardTo: destinations(appenders.Noop{})}))
			}
			wg.Wait()
		})
	}
}

func TestExited(t *testing.T) {
	c, _, _ := newComponent(t, false, Arguments{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, c.Run(ctx))
	_, err := c.receiver.Appender(t.Context()).Append(0, labels.FromStrings("__name__", "up"), 100, 1)
	require.ErrorContains(t, err, "has exited")
}

func newComponent(t testing.TB, enabled bool, args Arguments) (*Component, *prom.Registry, labelstore.LabelStore) {
	t.Helper()
	reg := prom.NewRegistry()
	ls := labelstore.New(util.TestLogger(t), prom.NewRegistry(), enabled).(labelstore.LabelStore)
	var exported storage.AppendableV2
	c, err := New(component.Options{
		ID:            "prometheus.route.test",
		Logger:        util.TestLogger(t),
		Registerer:    reg,
		OnStateChange: func(exports component.Exports) { exported = exports.(Exports).Receiver },
		GetServiceData: func(name string) (any, error) {
			if name != labelstore.ServiceName {
				return nil, fmt.Errorf("unexpected service %s", name)
			}
			return ls, nil
		},
	}, args)
	require.NoError(t, err)
	require.Same(t, c.receiver, exported)
	t.Cleanup(func() {
		if c.exited.Load() {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.NoError(t, c.Run(ctx))
	})
	return c, reg, ls
}

func destinations(receivers ...storage.AppendableV2) []storage.AppendableV2 { return receivers }

type event struct {
	kind     string
	ref      storage.SeriesRef
	labels   labels.Labels
	t, st    int64
	v        float64
	h        *histogram.Histogram
	fh       *histogram.FloatHistogram
	exemplar exemplar.Exemplar
	metadata metadata.Metadata
}

type recordingReceiver struct {
	appenders.Noop
	apps               []*recordingAppender
	contexts           []context.Context
	sendErr, finishErr error
}

func (r *recordingReceiver) Appender(ctx context.Context) storage.Appender {
	a := &recordingAppender{sendErr: r.sendErr, finishErr: r.finishErr}
	r.apps = append(r.apps, a)
	r.contexts = append(r.contexts, ctx)
	return a
}

type recordingAppender struct {
	events             []event
	options            []storage.AppendOptions
	commits, rollbacks int
	sendErr, finishErr error
}

func (a *recordingAppender) record(e event) (storage.SeriesRef, error) {
	a.events = append(a.events, e)
	return 42, a.sendErr // Deliberately collide across all downstream receivers.
}
func (a *recordingAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	return a.record(event{kind: "sample", ref: ref, labels: l, t: t, v: v})
}
func (a *recordingAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.record(event{kind: "histogram", ref: ref, labels: l, t: t, h: h, fh: fh})
}
func (a *recordingAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return a.record(event{kind: "exemplar", ref: ref, labels: l, exemplar: e})
}
func (a *recordingAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return a.record(event{kind: "metadata", ref: ref, labels: l, metadata: m})
}
func (a *recordingAppender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	return a.record(event{kind: "start", ref: ref, labels: l, t: t, st: st})
}
func (a *recordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.record(event{kind: "histogram_start", ref: ref, labels: l, t: t, st: st, h: h, fh: fh})
}
func (a *recordingAppender) SetOptions(opts *storage.AppendOptions) {
	a.options = append(a.options, *opts)
}
func (a *recordingAppender) Commit() error   { a.commits++; return a.finishErr }
func (a *recordingAppender) Rollback() error { a.rollbacks++; return a.finishErr }
