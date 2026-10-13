package podlogs

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	monitoringv1alpha2 "github.com/grafana/alloy/internal/component/loki/source/podlogs/internal/apis/monitoring/v1alpha2"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func ptr[T any](v T) *T { return &v }

func TestConvertPipeline(t *testing.T) {
	tests := []struct {
		name      string
		spec      *monitoringv1alpha2.PipelineSpec
		expectErr string
		check     func(t *testing.T, cfgs []stages.StageConfig)
	}{
		{
			name: "all supported stages",
			spec: &monitoringv1alpha2.PipelineSpec{Stages: []monitoringv1alpha2.PipelineStage{
				{Drop: &monitoringv1alpha2.DropStage{Expression: ".*DEBUG.*", DropCounterReason: "debug"}},
				{Regex: &monitoringv1alpha2.RegexStage{Expression: `user=(?P<user>\w+)`, Source: ptr("line")}},
				{Labels: &monitoringv1alpha2.LabelsStage{Values: map[string]string{"user": "", "uid": "user_id"}}},
				{StaticLabels: &monitoringv1alpha2.StaticLabelsStage{Values: map[string]string{"team": "a"}}},
			}},
			check: func(t *testing.T, cfgs []stages.StageConfig) {
				require.Len(t, cfgs, 4)
				require.Equal(t, ".*DEBUG.*", cfgs[0].DropConfig.Expression)
				require.Equal(t, "debug", cfgs[0].DropConfig.DropReason)
				require.Equal(t, "line", *cfgs[1].RegexConfig.Source)
				require.Nil(t, cfgs[2].LabelsConfig.Values["user"])
				require.Equal(t, "user_id", *cfgs[2].LabelsConfig.Values["uid"])
				require.Equal(t, "a", *cfgs[3].StaticLabelsConfig.Values["team"])
			},
		},
		{
			name:      "empty stage",
			spec:      &monitoringv1alpha2.PipelineSpec{Stages: []monitoringv1alpha2.PipelineStage{{}}},
			expectErr: "stage 0: exactly one of",
		},
		{
			name: "two keys in one stage",
			spec: &monitoringv1alpha2.PipelineSpec{Stages: []monitoringv1alpha2.PipelineStage{{
				Drop:  &monitoringv1alpha2.DropStage{Expression: "a"},
				Regex: &monitoringv1alpha2.RegexStage{Expression: "b"},
			}}},
			expectErr: "stage 0: exactly one of",
		},
		{
			name: "invalid label name",
			spec: &monitoringv1alpha2.PipelineSpec{Stages: []monitoringv1alpha2.PipelineStage{
				{StaticLabels: &monitoringv1alpha2.StaticLabelsStage{Values: map[string]string{"": "a"}}},
			}},
			expectErr: "stage 0: staticLabels: invalid label name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgs, err := convertPipeline(tc.spec)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			tc.check(t, cfgs)
		})
	}
}

func TestPipelineCache(t *testing.T) {
	out := make(chan loki.Entry, 10)
	cache := newPipelineCache(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, out)

	podLogs := &monitoringv1alpha2.PodLogs{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "app"},
		Spec: monitoringv1alpha2.PodLogsSpec{
			Pipeline: &monitoringv1alpha2.PipelineSpec{Stages: []monitoringv1alpha2.PipelineStage{
				{Drop: &monitoringv1alpha2.DropStage{Expression: ".*DEBUG.*"}},
				{Regex: &monitoringv1alpha2.RegexStage{Expression: `user=(?P<user>\w+)`, LabelsFromGroups: true}},
			}},
		},
	}
	key := client.ObjectKeyFromObject(podLogs)

	first, err := cache.get(podLogs)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Contains(t, first.id, key.String())

	// Same spec: same pipeline, no rebuild.
	again, err := cache.get(podLogs)
	require.NoError(t, err)
	require.Same(t, first, again)

	// Entries flow through the stages and end up on the channel.
	consume := func(line string) {
		batch := loki.NewBatch()
		batch.AddEntry(model.LabelSet{"job": "a"}, time.Now().UnixMicro(), push.Entry{Timestamp: time.Now(), Line: line})
		require.NoError(t, first.consumer.Consume(t.Context(), batch))
	}
	consume("level=DEBUG user=bob")
	consume("level=INFO user=alice")

	entry := <-out
	require.Equal(t, "level=INFO user=alice", entry.Line)
	require.Equal(t, model.LabelSet{"job": "a", "user": "alice"}, entry.Labels)
	require.Empty(t, out)

	// Changed spec: new pipeline with a new id.
	podLogs.Spec.Pipeline.Stages = podLogs.Spec.Pipeline.Stages[:1]
	changed, err := cache.get(podLogs)
	require.NoError(t, err)
	require.NotSame(t, first, changed)
	require.NotEqual(t, first.id, changed.id)
	require.Equal(t, []*podLogsPipeline{first}, cache.retired)

	// Invalid spec: error, the previous pipeline stays.
	podLogs.Spec.Pipeline.Stages = []monitoringv1alpha2.PipelineStage{{}}
	_, err = cache.get(podLogs)
	require.Error(t, err)
	require.Same(t, changed, cache.pipelines[key])

	// No pipeline: nil, entry retired.
	podLogs.Spec.Pipeline = nil
	none, err := cache.get(podLogs)
	require.NoError(t, err)
	require.Nil(t, none)
	require.NotContains(t, cache.pipelines, key)
	require.Equal(t, []*podLogsPipeline{first, changed}, cache.retired)

	// retain stops retired pipelines and drops keys not seen in a reconcile.
	podLogs.Spec.Pipeline = &monitoringv1alpha2.PipelineSpec{Stages: []monitoringv1alpha2.PipelineStage{
		{StaticLabels: &monitoringv1alpha2.StaticLabelsStage{Values: map[string]string{"a": "b"}}},
	}}
	_, err = cache.get(podLogs)
	require.NoError(t, err)
	cache.retain(map[client.ObjectKey]struct{}{})
	require.Empty(t, cache.pipelines)
	require.Empty(t, cache.retired)
}

func TestChannelConsumer_ContextCanceled(t *testing.T) {
	out := make(chan loki.Entry) // unbuffered, nobody reads
	c := channelConsumer{out: out}

	batch := loki.NewBatch()
	batch.AddEntry(model.LabelSet{"job": "a"}, time.Now().UnixMicro(), push.Entry{Line: "x"})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, c.Consume(ctx, batch), context.Canceled)
}
