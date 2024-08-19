package discovery

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
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
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}}, // Initial export
		},
		updatedTargets: []*targetgroup.Group{
			{Source: "test", Labels: model.LabelSet{"test_key_2": "value"}, Targets: []model.LabelSet{{"baz": "bux"}}},
		},
		expectedUpdatedExports: []component.Exports{
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}},   // Initial export
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}},   // Initial re-published on shutdown
			Exports{Targets: []Target{{"test_key_2": "value", "baz": "bux"}}}, // Updated export
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}},   // Initial export
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}},   // Initial re-published on shutdown
			Exports{Targets: []Target{{"test_key_2": "value", "baz": "bux"}}}, // Updated export
			Exports{Targets: []Target{{"test_key_2": "value", "baz": "bux"}}}, // Updated re-published on shutdown
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
			Exports{Targets: []Target{}},                                      // Initial publish
			Exports{Targets: []Target{}},                                      // Initial re-published on shutdown
			Exports{Targets: []Target{{"test_key_2": "value", "baz": "bux"}}}, // Updated export.
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{}},                                      // Initial publish
			Exports{Targets: []Target{}},                                      // Initial re-published on shutdown
			Exports{Targets: []Target{{"test_key_2": "value", "baz": "bux"}}}, // Updated export.
			Exports{Targets: []Target{{"test_key_2": "value", "baz": "bux"}}}, // Updated export re-published on shutdown.
		},
	},
	{
		name: "from one target to no targets",
		initialTargets: []*targetgroup.Group{
			{Source: "test", Labels: model.LabelSet{"test_key": "value"}, Targets: []model.LabelSet{{"foo": "bar"}}},
		},
		expectedInitialExports: []component.Exports{
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}}, // Initial export
		},
		updatedTargets: nil,
		expectedUpdatedExports: []component.Exports{
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}}, // Initial export
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}}, // Initial re-published on shutdown
			Exports{Targets: []Target{}},                                    // Updated export should publish empty!
		},
		expectedFinalExports: []component.Exports{
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}}, // Initial export
			Exports{Targets: []Target{{"foo": "bar", "test_key": "value"}}}, // Initial re-published on shutdown
			Exports{Targets: []Target{}},                                    // Updated export should publish empty!
			Exports{Targets: []Target{}},                                    // Updated re-published on shutdown
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
			comp := &Component{
				opts: component.Options{
					ID: "discovery.test",
					OnStateChange: func(e component.Exports) {
						publishedExportsMut.Lock()
						defer publishedExportsMut.Unlock()
						publishedExports = append(publishedExports, e)
					},
					Logger: log.NewLogfmtLogger(os.Stdout),
				},
				newDiscoverer: make(chan struct{}, 1),
			}

			discoverer := newFakeDiscoverer()
			updateDiscoverer(comp, discoverer)

			ctx, ctxCancel := context.WithCancel(context.Background())
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
				assert.Equal(t, tc.expectedInitialExports, publishedExports)
			}, 3*time.Second, time.Millisecond)

			discoverer = newFakeDiscoverer()
			updateDiscoverer(comp, discoverer)

			if tc.updatedTargets != nil {
				discoverer.Publish(tc.updatedTargets)
			}

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				publishedExportsMut.Lock()
				defer publishedExportsMut.Unlock()
				assert.Equal(t, tc.expectedUpdatedExports, publishedExports)
			}, 3*time.Second, time.Millisecond)

			ctxCancel()
			<-runDone

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				publishedExportsMut.Lock()
				defer publishedExportsMut.Unlock()
				assert.Equal(t, tc.expectedFinalExports, publishedExports)
			}, 3*time.Second, time.Millisecond)
		})
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
	ready       sync.WaitGroup
}

func newFakeDiscoverer() *fakeDiscoverer {
	ready := sync.WaitGroup{}
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
	select {
	case <-ctx.Done():
	}
}

func (f *fakeDiscoverer) Register() error { return nil }

func (f *fakeDiscoverer) Unregister() {}
