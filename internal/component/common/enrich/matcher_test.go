package enrich_test

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/enrich"
	"github.com/grafana/alloy/internal/component/discovery"
)

func TestMatcher(t *testing.T) {
	tests := []struct {
		name     string
		strategy map[string]string
		targets  []model.LabelSet
		input    model.LabelSet
		want     model.LabelSet
		wantLen  int
	}{
		{
			name:     "single label",
			strategy: map[string]string{"service": "service_name"},
			targets:  []model.LabelSet{{"service": "api", "env": "prod"}},
			input:    model.LabelSet{"service_name": "api"},
			want:     model.LabelSet{"service": "api", "env": "prod"},
			wantLen:  1,
		},
		{
			name:     "all pairs match the same target regardless of label name order",
			strategy: map[string]string{"namespace": "z_namespace", "pod": "a_pod"},
			targets: []model.LabelSet{
				{"namespace": "prod", "pod": "api", "env": "production"},
				{"namespace": "stage", "pod": "api", "env": "staging"},
			},
			input:   model.LabelSet{"z_namespace": "stage", "a_pod": "api", "extra": "ignored"},
			want:    model.LabelSet{"namespace": "stage", "pod": "api", "env": "staging"},
			wantLen: 2,
		},
		{
			name:     "pairs cannot match different targets",
			strategy: map[string]string{"namespace": "namespace", "pod": "pod"},
			targets: []model.LabelSet{
				{"namespace": "prod", "pod": "api"},
				{"namespace": "stage", "pod": "worker"},
			},
			input:   model.LabelSet{"namespace": "prod", "pod": "worker"},
			wantLen: 2,
		},
		{
			name:     "missing incoming label",
			strategy: map[string]string{"namespace": "namespace", "pod": "pod"},
			targets:  []model.LabelSet{{"namespace": "prod", "pod": "api"}},
			input:    model.LabelSet{"namespace": "prod"},
			wantLen:  1,
		},
		{
			name:     "empty incoming label",
			strategy: map[string]string{"namespace": "namespace", "pod": "pod"},
			targets:  []model.LabelSet{{"namespace": "prod", "pod": "api"}},
			input:    model.LabelSet{"namespace": "prod", "pod": ""},
			wantLen:  1,
		},
		{
			name:     "missing and empty target labels are skipped",
			strategy: map[string]string{"namespace": "namespace", "pod": "pod"},
			targets: []model.LabelSet{
				{"namespace": "prod", "pod": "api"},
				{"namespace": "prod"},
				{"namespace": "prod", "pod": ""},
			},
			input:   model.LabelSet{"namespace": "prod", "pod": "api"},
			want:    model.LabelSet{"namespace": "prod", "pod": "api"},
			wantLen: 1,
		},
		{
			name:    "empty strategy does not match",
			targets: []model.LabelSet{{"service": "api"}},
			input:   model.LabelSet{"service": "api"},
		},
		{
			name:     "no targets",
			strategy: map[string]string{"service": "service"},
			input:    model.LabelSet{"service": "api"},
		},
		{
			name:     "last duplicate target wins",
			strategy: map[string]string{"service": "service"},
			targets: []model.LabelSet{
				{"service": "api", "env": "old"},
				{"service": "api", "env": "new"},
			},
			input:   model.LabelSet{"service": "api"},
			want:    model.LabelSet{"service": "api", "env": "new"},
			wantLen: 1,
		},
		{
			name:     "value boundaries are preserved",
			strategy: map[string]string{"a": "a", "b": "b"},
			targets: []model.LabelSet{
				{"a": "ab", "b": "c"},
				{"a": "a", "b": "bc"},
			},
			input:   model.LabelSet{"a": "ab", "b": "c"},
			want:    model.LabelSet{"a": "ab", "b": "c"},
			wantLen: 2,
		},
		{
			name:     "multiple target labels map to the same incoming label",
			strategy: map[string]string{"a": "service", "b": "service"},
			targets:  []model.LabelSet{{"a": "api", "b": "api"}},
			input:    model.LabelSet{"service": "api"},
			want:     model.LabelSet{"a": "api", "b": "api"},
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := make([]discovery.Target, 0, len(tt.targets))
			for _, target := range tt.targets {
				targets = append(targets, discovery.NewTargetFromLabelSet(target))
			}
			matcher := enrich.NewMatcher(targets, tt.strategy)
			require.Equal(t, tt.wantLen, matcher.Len())

			// Exercise both label representations used by the components.
			require.Equal(t, tt.want, matcher.Match(func(name string) string {
				return string(tt.input[model.LabelName(name)])
			}))
			builder := labels.NewScratchBuilder(len(tt.input))
			for name, value := range tt.input {
				builder.Add(string(name), string(value))
			}
			builder.Sort()
			require.Equal(t, tt.want, matcher.Match(builder.Labels().Get))
		})
	}
}

func TestMatcherSnapshotsTargetsAndStrategy(t *testing.T) {
	strategy := map[string]string{"service": "service_name"}
	own := model.LabelSet{"service": "api", "env": "prod"}
	group := model.LabelSet{"service": "default", "region": "eu"}
	target := discovery.NewTargetFromSpecificAndBaseLabelSet(own, group)
	matcher := enrich.NewMatcher([]discovery.Target{target}, strategy)

	strategy["service"] = "other"
	own["service"] = "worker"
	own["env"] = "stage"
	group["region"] = "us"

	require.Equal(t, model.LabelSet{"service": "api", "env": "prod", "region": "eu"},
		matcher.Match(labels.FromStrings("service_name", "api").Get))
	require.Nil(t, matcher.Match(labels.FromStrings("service_name", "worker").Get))
}
