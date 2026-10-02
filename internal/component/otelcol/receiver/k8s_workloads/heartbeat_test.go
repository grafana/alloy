package k8s_workloads

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestHeartbeatSnapshot(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{true: "empty", false: "deployments"}[empty], func(t *testing.T) {
			c := testController(t)
			client := fake.NewSimpleClientset()
			if !empty {
				_, err := client.AppsV1().Deployments("demo").Create(t.Context(), deploymentFixture(), metav1.CreateOptions{})
				require.NoError(t, err)
			}
			c.opts.client = client
			c.opts.clusterName = "test"
			var logs plog.Logs
			c.opts.emit = func(_ context.Context, batch func() eventBatch) error { logs = batch().logs; return nil }
			require.NoError(t, c.heartbeat(t.Context()))
			record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			require.Equal(t, "grafana.sdlc.k8s.cluster.heartbeat", record.EventName())
			var payload clusterHeartbeat
			require.NoError(t, json.Unmarshal([]byte(record.Body().Str()), &payload))
			require.True(t, payload.Complete)
			require.Equal(t, stableID("cluster-1", "heartbeat", payload.ObservedAt), payload.SnapshotID)
			if empty {
				require.NotNil(t, payload.Deployments)
				require.Empty(t, payload.Deployments)
			} else {
				require.Len(t, payload.Deployments, 1)
				d := payload.Deployments[0]
				require.Equal(t, "demo", d.Namespace)
				require.Equal(t, rolloutSucceeded, d.Status)
				require.Equal(t, stableID("cluster-1", "d-1", "1"), d.RolloutID)
				require.Equal(t, "nginx:1.27", d.Containers[0].Image)
			}
			require.Zero(t, c.queue.Len(), "snapshots must not create historical rollout events")
		})
	}
}

func TestHeartbeatPaginationAndFailures(t *testing.T) {
	for _, failSecond := range []bool{false, true} {
		t.Run(map[bool]string{true: "incomplete", false: "complete"}[failSecond], func(t *testing.T) {
			c := testController(t)
			client := fake.NewSimpleClientset()
			c.opts.client = client
			calls, emitted := 0, 0
			client.PrependReactor("list", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
				calls++
				options := action.(ktesting.ListActionImpl).ListOptions
				require.Equal(t, int64(500), options.Limit)
				if calls == 1 {
					require.Empty(t, options.Continue)
					return true, &appsv1.DeploymentList{ListMeta: metav1.ListMeta{Continue: "next"}, Items: []appsv1.Deployment{*deploymentFixture()}}, nil
				}
				require.Equal(t, "next", options.Continue)
				if failSecond {
					return true, nil, errors.New("expired continuation")
				}
				return true, &appsv1.DeploymentList{}, nil
			})
			c.opts.emit = func(_ context.Context, batch func() eventBatch) error { emitted++; return nil }
			err := c.heartbeat(t.Context())
			require.Equal(t, 2, calls)
			if failSecond {
				require.Error(t, err)
				require.Zero(t, emitted)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, emitted)
			}
		})
	}
}

func TestHeartbeatDeliveryFailureDoesNotQueueRetry(t *testing.T) {
	c := testController(t)
	c.opts.client = fake.NewSimpleClientset()
	c.opts.emit = func(_ context.Context, _ func() eventBatch) error { return errors.New("unavailable") }
	require.Error(t, c.heartbeat(t.Context()))
	require.Zero(t, c.queue.Len())
}
