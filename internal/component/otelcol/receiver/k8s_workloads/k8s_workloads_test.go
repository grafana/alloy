package k8s_workloads

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"k8s.io/client-go/rest"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/syntax"
)

func TestArgumentsUnmarshal(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		cluster_name = "production-eu"
		cluster_uid  = "cluster-uid"

		clustering {
			enabled = true
		}

		output {}
	`), &args))

	require.Equal(t, "production-eu", args.ClusterName)
	require.Equal(t, "cluster-uid", args.ClusterUID)
	require.True(t, args.Clustering.Enabled)
	require.NotNil(t, args.Output)
}

func TestArgumentsRequireOutput(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal(nil, &args)
	require.ErrorContains(t, err, `missing required block "output"`)
}

func TestShouldWatch(t *testing.T) {
	tests := []struct {
		name       string
		clustering bool
		cluster    *fakeCluster
		want       bool
		wantErr    string
	}{
		{
			name:    "clustering disabled",
			cluster: &fakeCluster{},
			want:    true,
		},
		{
			name:       "cluster not ready",
			clustering: true,
			cluster:    &fakeCluster{},
		},
		{
			name:       "local owner",
			clustering: true,
			cluster:    &fakeCluster{ready: true, owners: []peer.Peer{{Self: true}}},
			want:       true,
		},
		{
			name:       "remote owner",
			clustering: true,
			cluster:    &fakeCluster{ready: true, owners: []peer.Peer{{Self: false}}},
		},
		{
			name:       "lookup failure",
			clustering: true,
			cluster:    &fakeCluster{ready: true, lookupErr: errors.New("lookup failed")},
			wantErr:    "determining workload watcher ownership: lookup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Component{
				opts:    component.Options{ID: "otelcol.receiver.k8s_workloads.test"},
				cluster: tt.cluster,
			}
			got, err := c.shouldWatch(Arguments{Clustering: cluster.ComponentBlock{Enabled: tt.clustering}})
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClusterChangeKeepsGenerationWhenOwnershipIsUnchanged(t *testing.T) {
	fake := &fakeCluster{ready: true, owners: []peer.Peer{{Self: false}}}
	c := &Component{
		opts:           component.Options{ID: "otelcol.receiver.k8s_workloads.test"},
		cluster:        fake,
		args:           Arguments{Clustering: cluster.ComponentBlock{Enabled: true}},
		restConfig:     &rest.Config{},
		restart:        make(chan struct{}, 1),
		clusterChanged: make(chan struct{}, 1),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	require.Eventually(t, func() bool { return fake.lookupCalls.Load() == 1 }, time.Second, time.Millisecond)
	c.NotifyClusterChange()
	require.Eventually(t, func() bool { return fake.lookupCalls.Load() == 2 }, time.Second, time.Millisecond)
	require.Never(t, func() bool { return fake.lookupCalls.Load() > 2 }, 100*time.Millisecond, time.Millisecond)

	cancel()
	require.NoError(t, <-done)
}

type fakeCluster struct {
	ready       bool
	owners      []peer.Peer
	lookupErr   error
	lookupCalls atomic.Int32
}

func (f *fakeCluster) Lookup(shard.Key, int, shard.Op) ([]peer.Peer, error) {
	f.lookupCalls.Add(1)
	return f.owners, f.lookupErr
}

func (f *fakeCluster) Peers() []peer.Peer { return f.owners }
func (f *fakeCluster) Ready() bool        { return f.ready }
func (f *fakeCluster) Enabled() bool      { return true }

func TestSnapshotArguments(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`output {}`), &args))
	require.Equal(t, time.Minute, args.Snapshots.Interval)
	require.Equal(t, 512*1024, args.Snapshots.MaxSizeBytes)
	require.NoError(t, syntax.Unmarshal([]byte(`snapshots {
 interval = "5s"
 max_size_bytes = 8192
}
output {}`), &args))
	require.Equal(t, 5*time.Second, args.Snapshots.Interval)
	for _, config := range []string{`snapshots { interval = "0s" }
output {}`, `snapshots { max_size_bytes = 0 }
output {}`} {
		require.Error(t, syntax.Unmarshal([]byte(config), &args))
	}
}
