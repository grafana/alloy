package discovery

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Masterminds/goutils"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// discovererUpdateTestCase is a test case for testing discovery updates. A discovery component is created and the
// initialTargets are published. We check that the component correctly publishes exports matching exepectedInitialExports.
// Then, the discoverer is updated and new updatedTargets are published. We check that the exports published so far
// match the expectedUpdatedExports. Finally, the component is shut down, and we check that the list of exports published
// matches the expectedFinalExports.
type discovererUpdateTestCase struct {
	name                   string
	initialTargets         []*targetgroup.Group
	expectedInitialExports []component.Exports
	updatedTargets         []*targetgroup.Group
	expectedUpdatedExports []component.Exports
	expectedFinalExports   []component.Exports
}

var updateTestCases = []discovererUpdateTestCase{
	{
		name: "from one target to another",
		initialTargets: []*targetgroup.Group{
			{Source: "test", Labels: model.LabelSet{"test_key": "value"}, Targets: []model.LabelSet{{"foo": "bar"}}},
		},
		expectedInitialExports: []component.Exports{
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}}, // Initial export
		},
		updatedTargets: []*targetgroup.Group{
			{Source: "test", Labels: model.LabelSet{"test_key_2": "value"}, Targets: []model.LabelSet{{"baz": "bux"}}},
		},
		expectedUpdatedExports: []component.Exports{
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}},   // Initial export
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}},   // Initial re-published on shutdown
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"test_key_2": "value", "baz": "bux"})}}, // Updated export
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}},   // Initial export
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}},   // Initial re-published on shutdown
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"test_key_2": "value", "baz": "bux"})}}, // Updated export
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"test_key_2": "value", "baz": "bux"})}}, // Updated re-published on shutdown
		},
	},
	{
		name:           "from no targets to no targets",
		initialTargets: nil,
		expectedInitialExports: []component.Exports{
			Exports{Targets: []Target{}}, // Initial
		},
		updatedTargets: nil,
		expectedUpdatedExports: []component.Exports{
			Exports{Targets: []Target{}}, // Initial
			Exports{Targets: []Target{}}, // Initial on shutdown
			Exports{Targets: []Target{}}, // Updated
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{}}, // Initial
			Exports{Targets: []Target{}}, // Initial on shutdown
			Exports{Targets: []Target{}}, // Updated
			Exports{Targets: []Target{}}, // Updated on shutdown
		},
	},
	{
		name:           "from no targets to one target",
		initialTargets: nil,
		expectedInitialExports: []component.Exports{
			Exports{Targets: []Target{}}, // Initial publish
		},
		updatedTargets: []*targetgroup.Group{
			{Source: "test", Labels: model.LabelSet{"test_key_2": "value"}, Targets: []model.LabelSet{{"baz": "bux"}}},
		},
		expectedUpdatedExports: []component.Exports{
			Exports{Targets: []Target{}}, // Initial publish
			Exports{Targets: []Target{}}, // Initial re-published on shutdown
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"test_key_2": "value", "baz": "bux"})}}, // Updated export.
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{}}, // Initial publish
			Exports{Targets: []Target{}}, // Initial re-published on shutdown
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"test_key_2": "value", "baz": "bux"})}}, // Updated export.
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"test_key_2": "value", "baz": "bux"})}}, // Updated export re-published on shutdown.
		},
	},
	{
		name: "from one target to no targets",
		initialTargets: []*targetgroup.Group{
			{Source: "test", Labels: model.LabelSet{"test_key": "value"}, Targets: []model.LabelSet{{"foo": "bar"}}},
		},
		expectedInitialExports: []component.Exports{
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}}, // Initial export
		},
		updatedTargets: nil,
		expectedUpdatedExports: []component.Exports{
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}}, // Initial export
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}}, // Initial re-published on shutdown
			Exports{Targets: []Target{}}, // Updated export should publish empty!
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}}, // Initial export
			Exports{Targets: []Target{NewTargetFromMap(map[string]string{"foo": "bar", "test_key": "value"})}}, // Initial re-published on shutdown
			Exports{Targets: []Target{}}, // Updated export should publish empty!
			Exports{Targets: []Target{}}, // Updated re-published on shutdown
		},
	},
}

func TestDiscoveryUpdates(t *testing.T) {
	prevMaxUpdateFrequency := MaxUpdateFrequency
	MaxUpdateFrequency = 100 * time.Millisecond
	defer func() {
		MaxUpdateFrequency = prevMaxUpdateFrequency
	}()

	for _, tc := range updateTestCases {
		t.Run(tc.name, func(t *testing.T) {
			var publishedExports []component.Exports
			publishedExportsMut := sync.Mutex{}
			opts := component.Options{
				ID: "discovery.test",
				OnStateChange: func(e component.Exports) {
					publishedExportsMut.Lock()
					defer publishedExportsMut.Unlock()
					publishedExports = append(publishedExports, e)
				},
				Logger: log.NewLogfmtLogger(os.Stdout),
				GetServiceData: func(name string) (any, error) {
					switch name {
					case livedebugging.ServiceName:
						return livedebugging.NewLiveDebugging(), nil
					default:
						return nil, fmt.Errorf("service %q does not exist", name)
					}
				},
			}
			debugDataPublisher, _ := opts.GetServiceData(livedebugging.ServiceName)
			comp := &Component{
				opts:               opts,
				newDiscoverer:      make(chan struct{}, 1),
				debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
			}

			discoverer := newFakeDiscoverer()
			updateDiscoverer(comp, discoverer)

			ctx, ctxCancel := context.WithCancel(t.Context())
			defer ctxCancel()

			runDone := make(chan struct{})
			go func() {
				err := comp.Run(ctx)
				require.NoError(t, err)
				runDone <- struct{}{}
			}()

			if tc.initialTargets != nil {
				discoverer.Publish(tc.initialTargets)
			}

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				publishedExportsMut.Lock()
				defer publishedExportsMut.Unlock()
				assertExportsEqual(t, tc.expectedInitialExports, publishedExports)
			}, 3*time.Second, time.Millisecond)

			discoverer = newFakeDiscoverer()
			updateDiscoverer(comp, discoverer)

			if tc.updatedTargets != nil {
				discoverer.Publish(tc.updatedTargets)
			}

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				publishedExportsMut.Lock()
				defer publishedExportsMut.Unlock()
				assertExportsEqual(t, tc.expectedUpdatedExports, publishedExports)
			}, 3*time.Second, time.Millisecond)

			ctxCancel()
			<-runDone

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				publishedExportsMut.Lock()
				defer publishedExportsMut.Unlock()
				assertExportsEqual(t, tc.expectedFinalExports, publishedExports)
			}, 3*time.Second, time.Millisecond)
		})
	}
}

func assertExportsEqual(t *assert.CollectT, expected []component.Exports, actual []component.Exports) {
	if actual == nil {
		assert.NotNil(t, actual)
		return
	}
	equal := equality.DeepEqual(expected, actual)
	assert.True(t, equal, "expected and actual exports are different")
	if !equal { // also do assert.Equal to get a nice diff view if there is an issue.
		assert.Equal(t, expected, actual)
	}
}

/*
on darwin/arm64/Apple M2:
Benchmark_ToAlloyTargets-8   	     150	   7549967 ns/op	12768249 B/op	   40433 allocs/op
Benchmark_ToAlloyTargets-8   	     169	   7257841 ns/op	12767441 B/op	   40430 allocs/op
Benchmark_ToAlloyTargets-8   	     171	   7026276 ns/op	12767394 B/op	   40430 allocs/op
Benchmark_ToAlloyTargets-8   	     170	   7060700 ns/op	12767377 B/op	   40430 allocs/op
Benchmark_ToAlloyTargets-8   	     170	   7034392 ns/op	12767427 B/op	   40430 allocs/op
*/
func Benchmark_ToAlloyTargets(b *testing.B) {
	sharedLabels := 5
	labelsPerTarget := 5
	labelsLength := 10
	targetsCount := 20_000

	genLabelSet := func(size int) model.LabelSet {
		ls := model.LabelSet{}
		for i := 0; i < size; i++ {
			name, _ := goutils.RandomAlphaNumeric(labelsLength)
			value, _ := goutils.RandomAlphaNumeric(labelsLength)
			ls[model.LabelName(name)] = model.LabelValue(value)
		}
		return ls
	}

	var targets = []model.LabelSet{}
	for i := 0; i < targetsCount; i++ {
		targets = append(targets, genLabelSet(labelsPerTarget))
	}

	cache := map[string]*targetgroup.Group{}
	cache["test"] = &targetgroup.Group{
		Targets: targets,
		Labels:  genLabelSet(sharedLabels),
		Source:  "test",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		toAlloyTargets(cache)
	}
}

func updateDiscoverer(comp *Component, discoverer *fakeDiscoverer) {
	comp.discMut.Lock()
	defer comp.discMut.Unlock()
	comp.latestDisc = discoverer
	comp.newDiscoverer <- struct{}{}
}

type fakeDiscoverer struct {
	publishChan chan<- []*targetgroup.Group
	ready       *sync.WaitGroup
}

func newFakeDiscoverer() *fakeDiscoverer {
	ready := &sync.WaitGroup{}
	ready.Add(1)
	return &fakeDiscoverer{
		ready: ready,
	}
}

func (f *fakeDiscoverer) Publish(tg []*targetgroup.Group) {
	f.ready.Wait()
	f.publishChan <- tg
}

func (f *fakeDiscoverer) Run(ctx context.Context, publishChan chan<- []*targetgroup.Group) {
	f.publishChan = publishChan
	f.ready.Done()
	<-ctx.Done()
}

func (f *fakeDiscoverer) Register() error { return nil }

func (f *fakeDiscoverer) Unregister() {}
