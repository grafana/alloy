package stages

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

func TestStructuredMetadataStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
		checks   entryCheckFNs
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "app=loki component=ingester", now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"app": "loki"}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               "app=loki component=ingester",
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "loki"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"app":"loki" ,"component":"ingester"}`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"app": "loki"}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               `{"app":"loki" ,"component":"ingester"}`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "loki"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"time":    "2019-01-01T01:00:00.000000001Z",
					"stream":  "stderr",
					"flags":   "P",
					"content": "i'm a log message!",
				}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`,
					StructuredMetadata: push.LabelsAdapter{{Name: "stream", Value: "stderr"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"time":    "2019-01-01T01:00:00.000000001Z",
					"stream":  "stderr",
					"flags":   "P",
					"content": "i'm a log message!",
				}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`,
					StructuredMetadata: push.LabelsAdapter{{Name: "stream", Value: "stderr"}},
				}),
			},
		},
		{
			name: "expected structured metadata to be extracted once when present in both extracted values and labels",
			config: `
			stage.cri {}

			stage.structured_metadata {
				values = { "stream" = "" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `2019-01-01T01:00:00.000000001Z stderr F i'm a log message!`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"time":    "2019-01-01T01:00:00.000000001Z",
					"stream":  "stderr",
					"flags":   "F",
					"content": "i'm a log message!",
				}, model.LabelSet{}, push.Entry{
					// cri overwrites the timestamp with the one parsed from the line.
					Timestamp:          time.Date(2019, 1, 1, 1, 0, 0, 1, time.UTC),
					Line:               `i'm a log message!`,
					StructuredMetadata: push.LabelsAdapter{{Name: "stream", Value: "stderr"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"app":"loki" ,"component":"ingester"}`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"app": "LOKI"}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               `{"app":"loki" ,"component":"ingester"}`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "LOKI"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"app":"loki" ,"component":"ingester"}`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"app": "loki", "component": "ingester"}, model.LabelSet{"component": "ingester"}, push.Entry{
					Timestamp:          now,
					Line:               `{"app":"loki" ,"component":"ingester"}`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "loki"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `sample log line`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"component": "querier"}, push.Entry{
					Timestamp:          now,
					Line:               `sample log line`,
					StructuredMetadata: push.LabelsAdapter{{Name: "pod", Value: "loki-querier-664f97db8d-qhnwg"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `sample log line`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"component": "querier"}, push.Entry{
					Timestamp:          now,
					Line:               `sample log line`,
					StructuredMetadata: push.LabelsAdapter{{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"}},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `sample log line`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"component": "querier"}, push.Entry{
					Timestamp: now,
					Line:      `sample log line`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "label_app_kubernetes_io_component", Value: "querier"},
						{Name: "label_app_kubernetes_io_name", Value: "loki"},
					},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `sample log line`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{}, model.LabelSet{"component": "querier"}, push.Entry{
					Timestamp: now,
					Line:      `sample log line`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "label_app_kubernetes_io_component", Value: "querier"},
						{Name: "label_app_kubernetes_io_name", Value: "loki"},
						{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"},
					},
				}),
			},
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `pod=loki-querier-664f97db8d-qhnwg metadata_name=loki metadata_component=querier msg="sample log line"`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"pod":                "loki-querier-664f97db8d-qhnwg",
					"metadata_name":      "loki",
					"metadata_component": "querier",
				}, model.LabelSet{}, push.Entry{
					Timestamp: now,
					Line:      `pod=loki-querier-664f97db8d-qhnwg metadata_name=loki metadata_component=querier msg="sample log line"`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "metadata_component", Value: "querier"},
						{Name: "metadata_name", Value: "loki"},
						{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"},
					},
				}),
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"app":"loki", "component_nested": {"name":"ingester", "props":{"n1": "v1", "n2": "v2"}}, "component_non_nested": "non_nested_val"}`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{
					"app":                  "loki",
					"component_nested":     `{"name":"ingester","props":{"n1":"v1","n2":"v2"}}`,
					"component_non_nested": "non_nested_val",
				}, model.LabelSet{}, push.Entry{
					Timestamp: now,
					Line:      `{"app":"loki", "component_nested": {"name":"ingester", "props":{"n1": "v1", "n2": "v2"}}, "component_non_nested": "non_nested_val"}`,
					StructuredMetadata: push.LabelsAdapter{
						{Name: "component_nested", Value: `{"name":"ingester","props":{"n1":"v1","n2":"v2"}}`},
						{Name: "component_non_nested", Value: "non_nested_val"},
					},
				}),
			},
			checks: entryCheckFNs{
				extracted:          canonicalExtractedCheck,
				structuredMetadata: canonicalStructuredMetadataCheck,
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
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `app=first next_app=second`, now),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"app": "first", "next_app": "second"}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               `app=first next_app=second`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "second"}},
				}),
			},
		},
		{
			name: "expected later source within a stage to replace existing structured metadata",
			config: `
			stage.structured_metadata {
				values = { "app" = "" }
			}
			`,
			entries: []Entry{
				newTestEntry(map[string]any{"app": "from-extracted"}, model.LabelSet{"app": "from-labels"}, push.Entry{
					Timestamp:          now,
					Line:               `sample log line`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "original"}},
				}),
			},
			expected: []Entry{
				newTestEntry(map[string]any{"app": "from-labels"}, model.LabelSet{}, push.Entry{
					Timestamp:          now,
					Line:               `sample log line`,
					StructuredMetadata: push.LabelsAdapter{{Name: "app", Value: "from-labels"}},
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected, tt.checks)
		})
	}
}

func canonicalStructuredMetadataCheck(expected, actual push.LabelsAdapter) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].Name != actual[i].Name {
			return false
		}
		if canonicalizeJSON(expected[i].Value) != canonicalizeJSON(actual[i].Value) {
			return false
		}
	}
	return true
}

func canonicalExtractedCheck(expected, actual map[string]any) bool {
	if len(expected) != len(actual) {
		return false
	}
	for k, v := range expected {
		av, ok := actual[k]
		if !ok {
			return false
		}

		evs, evOk := v.(string)
		avs, avOK := av.(string)
		if evOk && avOK {
			if canonicalizeJSON(evs) != canonicalizeJSON(avs) {
				return false
			}
			continue
		}
		if v != av {
			return false
		}
	}
	return true
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
