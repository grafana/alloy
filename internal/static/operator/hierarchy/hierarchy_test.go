//go:build !nonetwork && !nodocker && !race

package hierarchy

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/util/k8s"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// TestNotifier tests that notifier properly handles events for changed
// objects.
func TestNotifier(t *testing.T) {
	// TODO: this is broken with go 1.20.6
	// waiting on https://github.com/testcontainers/testcontainers-go/issues/1359
	t.Skip()
	l := log.NewNopLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cluster, err := k8s.NewCluster(ctx, k8s.Options{})
	require.NoError(t, err)
	defer cluster.Stop()

	cli := cluster.Client()

	// Tests will rely on a namespace existing, so let's create a namespace with
	// some labels.
	testNs := v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "enqueue-test",
			Labels: map[string]string{"foo": "bar"},
		},
	}
	err = cli.Create(ctx, &testNs)
	require.NoError(t, err)

	testPod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "test-pod",
		Namespace: "enqueue-test",
		Labels:    map[string]string{"fizz": "buzz"},
	}}

	tt := []struct {
		name          string
		sel           Selector
		expectEnqueue bool
	}{
		{
			name:          "no watchers",
			sel:           nil,
			expectEnqueue: false,
		},
		{
			name: "matches watcher",
			sel: &LabelsSelector{
				NamespaceName:   "enqueue-test",
				NamespaceLabels: parseSelector(t, "foo in (bar)"),
				Labels:          parseSelector(t, "fizz in (buzz)"),
			},
			expectEnqueue: true,
		},
		{
			name: "matches watcher with explicit namespace",
			sel: &LabelsSelector{
				NamespaceName: "enqueue-test",
				Labels:        parseSelector(t, "fizz in (buzz)"),
			},
			expectEnqueue: true,
		},
		{
			name: "bad namespace name selector",
			sel: &LabelsSelector{
				NamespaceName: "default",
				Labels:        labels.Everything(),
			},
			expectEnqueue: false,
		},
		{
			name: "bad namespace label selector",
			sel: &LabelsSelector{
				NamespaceName:   "enqueue-test",
				NamespaceLabels: parseSelector(t, "foo notin (bar)"),
				Labels:          labels.Everything(),
			},
			expectEnqueue: false,
		},
		{
			name: "bad label selector",
			sel: &LabelsSelector{
				NamespaceName:   "default",
				NamespaceLabels: labels.Everything(),
				Labels:          parseSelector(t, "fizz notin (buzz)"),
			},
			expectEnqueue: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			limiter := workqueue.DefaultControllerRateLimiter()
			q := workqueue.NewRateLimitingQueue(limiter)

			notifier := NewNotifier(l, cli)

			if tc.sel != nil {
				err := notifier.Notify(Watcher{
					Object:   &v1.Pod{},
					Owner:    types.NamespacedName{Name: "watcher", Namespace: "enqueue-test"},
					Selector: tc.sel,
				})
				require.NoError(t, err)
			}

			e := notifier.EventHandler()
			e.Create(ctx, event.CreateEvent{Object: testPod}, q)
			if tc.expectEnqueue {
				require.Equal(t, 1, q.Len(), "expected change enqueue")
			} else {
				require.Equal(t, 0, q.Len(), "no changes should have been enqueued")
			}
		})
	}
}

func parseSelector(t *testing.T, selector string) labels.Selector {
	t.Helper()
	s, err := labels.Parse(selector)
	require.NoError(t, err)
	return s
}
