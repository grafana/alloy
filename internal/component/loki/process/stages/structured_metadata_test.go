package stages

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
)

func TestStructuredMetadataStage(t *testing.T) {
	type testCase struct {
		name   string
		config string
		entry  Entry

		expectedLabels             model.LabelSet
		expectedStructuredMetadata push.LabelsAdapter
	}

	tests := []testCase{
		{
			name: "expected structured metadata to be extracted with logfmt parser and to be added to entry",
			config: `
				stage.logfmt {
					mapping = { "app" = "" }
				}

				stage.structured_metadata {
					values = { "app" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: "app=loki component=ingester"}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "loki"}},
		},
		{
			name: "expected structured metadata to be extracted with json parser and to be added to entry",
			config: `
				stage.json {
					expressions = { app = "" }
				}

				stage.structured_metadata {
					values = { "app" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `{"app":"loki" ,"component":"ingester"}`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "loki"}},
		},
		{
			name: "expected structured metadata to be extracted with regexp parser and to be added to entry",
			config: `
				stage.regex {
					expression = "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<content>.*)$"
				}

				stage.structured_metadata {
					values = { "stream" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "stream", Value: "stderr"}},
		},
		{
			name: "expected structured metadata to be extracted once when values and regex both match extracted values",
			config: `
				stage.regex {
					expression = "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<content>.*)$"
				}

				stage.structured_metadata {
					values = { "stream" = "" }
					regex  = "stream"
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "stream", Value: "stderr"}},
		},
		{
			name: "expected structured metadata to be extracted once when present in both extracted values and labels",
			config: `
				stage.cri {}

				stage.structured_metadata {
					values = { "stream" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `2019-01-01T01:00:00.000000001Z stderr F i'm a log message!`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "stream", Value: "stderr"}},
		},
		{
			name: "expected structured metadata to be extracted with json parser and to be added to entry after rendering the template",
			config: `
				stage.json {
					expressions = { app = "" }
				}

				stage.template {
					source   = "app"
					template = "{{ ToUpper .Value }}"
				}

				stage.structured_metadata {
					values = { "app" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `{"app":"loki" ,"component":"ingester"}`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "LOKI"}},
		},
		{
			name: "expected structured metadata and regular labels to be extracted with json parser and to be added to entry",
			config: `
				stage.json {
					expressions = { app = "", component = "" }
				}

				stage.structured_metadata {
					values = { "app" = "" }
				}

				stage.labels {
					values = { "component" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `{"app":"loki" ,"component":"ingester"}`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "loki"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("ingester")},
		},
		{
			name: "expected structured metadata and regular labels to be extracted with static labels stage and to be added to entry",
			config: `
				stage.static_labels {
					values = { "component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg" }
				}

				stage.structured_metadata {
					values = { "pod" = "" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `sample log line`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "pod", Value: "loki-querier-664f97db8d-qhnwg"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		{
			name: "expected structured metadata and regular labels to be extracted with static labels stage using different structured key",
			config: `
				stage.static_labels {
					values = { "component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg" }
				}

				stage.structured_metadata {
					values = { "pod_name" = "pod" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `sample log line`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		{
			name: "expected structured metadata and regular labels to be extracted using regex with static labels stage",
			config: `
				stage.static_labels {
					values = { "component" = "querier", "label_app_kubernetes_io_name" = "loki", "label_app_kubernetes_io_component" = "querier" }
				}

				stage.structured_metadata {
					regex = "label_.*"
				}
			`,
			entry: newTestEntry(nil, nil, push.Entry{Line: `sample log line`}),
			expectedStructuredMetadata: push.LabelsAdapter{
				{Name: "label_app_kubernetes_io_component", Value: "querier"},
				{Name: "label_app_kubernetes_io_name", Value: "loki"},
			},
			expectedLabels: model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		{
			name: "expected structured metadata and regular labels to be extracted using regex and non-regex with static labels stage",
			config: `
				stage.static_labels {
					values = { "component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg", "label_app_kubernetes_io_name" = "loki", "label_app_kubernetes_io_component" = "querier" }
				}

				stage.structured_metadata {
					values = { "pod_name" = "pod" }
					regex  = "label_.*"
				}
			`,
			entry: newTestEntry(nil, nil, push.Entry{Line: `sample log line`}),
			expectedStructuredMetadata: push.LabelsAdapter{
				{Name: "label_app_kubernetes_io_component", Value: "querier"},
				{Name: "label_app_kubernetes_io_name", Value: "loki"},
				{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"},
			},
			expectedLabels: model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		{
			name: "expected structured metadata to be set from extracted values",
			config: `
				stage.logfmt {
					mapping = { "pod" = "", "metadata_name" = "", "metadata_component" = "" }
				}

				stage.structured_metadata {
					values = { "pod_name" = "pod" }
					regex  = "metadata_.*"
				}
			`,
			entry: newTestEntry(nil, nil, push.Entry{Line: `pod=loki-querier-664f97db8d-qhnwg metadata_name=loki metadata_component=querier msg="sample log line"`}),
			expectedStructuredMetadata: push.LabelsAdapter{
				{Name: "metadata_component", Value: "querier"},
				{Name: "metadata_name", Value: "loki"},
				{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"},
			},
		},
		{
			name: "expected structured metadata from nested values",
			config: `
				stage.json {
					expressions = { app = "", component_nested = "", component_non_nested = "" }
				}

				stage.structured_metadata {
					regex = "component_.*"
				}
			`,
			entry: newTestEntry(nil, nil, push.Entry{Line: `{"app":"loki", "component_nested": {"name":"ingester", "props":{"n1": "v1", "n2": "v2"}}, "component_non_nested": "non_nested_val"}`}),
			expectedStructuredMetadata: push.LabelsAdapter{
				{Name: "component_nested", Value: `{"name":"ingester","props":{"n1":"v1","n2":"v2"}}`},
				{Name: "component_non_nested", Value: "non_nested_val"},
			},
		},
		{
			name: "expected later structured metadata stage to replace earlier stage output",
			config: `
				stage.logfmt {
					mapping = { "app" = "", "next_app" = "" }
				}

				stage.structured_metadata {
					values = { "app" = "app" }
				}

				stage.structured_metadata {
					values = { "app" = "next_app" }
				}
			`,
			entry:                      newTestEntry(nil, nil, push.Entry{Line: `app=first next_app=second`}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "second"}},
		},
		{
			name: "expected later source within a stage to replace existing structured metadata",
			config: `
				stage.structured_metadata {
					values = { "app" = "" }
				}
			`,
			entry: newTestEntry(map[string]any{"app": "from-extracted"}, model.LabelSet{
					"app": "from-labels",
				}, push.Entry{
					Line:               `sample log line`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "original"}},
				}),
			expectedStructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "from-labels"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(tt.config), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

			entry := tt.entry
			entry.Timestamp = time.Now()

			result := processEntries(pl, entry)[0]

			require.ElementsMatch(t, normalizeStructuredMetadata(tt.expectedStructuredMetadata), normalizeStructuredMetadata(result.StructuredMetadata))

			if tt.expectedLabels != nil {
				require.Equal(t, tt.expectedLabels, result.Labels)
			} else {
				require.Empty(t, result.Labels)
			}
		})
	}
}

func normalizeStructuredMetadata(labels push.LabelsAdapter) push.LabelsAdapter {
	normalized := make(push.LabelsAdapter, 0, len(labels))

	for _, label := range labels {
		normalized = append(normalized, push.LabelAdapter{
			Name:  label.Name,
			Value: canonicalizeJSON(label.Value),
		})
	}

	return normalized
}

func canonicalizeJSON(value string) string {
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return value
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		return value
	}

	return string(bytes.TrimSpace(encoded))
}
