package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/loki/pkg/push"
)

var pipelineStagesStructuredMetadataFromLogfmt = `
stage.logfmt {
	mapping = { "app" = ""}
}

stage.structured_metadata { 
	values = {"app" = ""}
}
`

var pipelineStagesStructuredMetadataFromJSON = `
stage.json {
	expressions = {app = ""}
}

stage.structured_metadata { 
	values = {"app" = ""}
}
`

var pipelineStagesStructuredMetadataWithRegexParser = `
stage.regex {
	expression = "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<content>.*)$"
}

stage.structured_metadata { 
	values = {"stream" = ""}
}
`

var pipelineStagesStructuredMetadataFromJSONWithTemplate = `
stage.json {
	expressions = {app = ""}
}

stage.template {
    source   = "app"
    template = "{{ ToUpper .Value }}"
}

stage.structured_metadata { 
	values = {"app" = ""}
}
`

var pipelineStagesStructuredMetadataAndRegularLabelsFromJSON = `
stage.json {
	expressions = {app = "", component = "" }
}

stage.structured_metadata { 
	values = {"app" = ""}
}

stage.labels { 
	values = {"component" = ""}
}
`

var pipelineStagesStructuredMetadataFromStaticLabels = `
stage.static_labels {
	values = {"component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg"}
}

stage.structured_metadata {
	values = {"pod" = ""}
}
`

var pipelineStagesStructuredMetadataFromStaticLabelsDifferentKey = `
stage.static_labels {
	values = {"component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg"}
}

stage.structured_metadata {
	values = {"pod_name" = "pod"}
}
`

var pipelineStagesStructuredMetadataFromRegexLabels = `
stage.static_labels {
  values = {"component" = "querier", "label_app_kubernetes_io_name" = "loki", "label_app_kubernetes_io_component" = "querier"}
}

stage.structured_metadata {
  regex = "label_.*"
}
`
var pipelineStagesStructuredMetadataFromRegexAndNonRegexLabels = `
stage.static_labels {
  values = {"component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg", "label_app_kubernetes_io_name" = "loki", "label_app_kubernetes_io_component" = "querier"}
}

stage.structured_metadata {
  values = {"pod_name" = "pod"}
  regex = "label_.*"
}
`

var pipelineStagesStructuredMetadataFromExtractedValues = `
stage.logfmt {
  mapping = { "pod" = "", "metadata_name" = "", "metadata_component" = "" }
}

stage.structured_metadata {
  values = {"pod_name" = "pod"}
  regex = "metadata_.*"
}
`

var pipelineStagesStructuredMetadataFromNestedValues = `
stage.json {
	expressions = {app = "", component_nested = "", component_non_nested = "" }
}

stage.structured_metadata {
  regex = "component_.*"
}
`

func Test_StructuredMetadataStage(t *testing.T) {
	tests := map[string]struct {
		pipelineStagesYaml         string
		logLine                    string
		expectedStructuredMetadata push.LabelsAdapter
		expectedLabels             model.LabelSet
	}{
		"expected structured metadata to be extracted with logfmt parser and to be added to entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromLogfmt,
			logLine:                    "app=loki component=ingester",
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "app", Value: "loki"}},
		},
		"expected structured metadata to be extracted with json parser and to be added to entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromJSON,
			logLine:                    `{"app":"loki" ,"component":"ingester"}`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "app", Value: "loki"}},
		},
		"expected structured metadata to be extracted with regexp parser and to be added to entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataWithRegexParser,
			logLine:                    `2019-01-01T01:00:00.000000001Z stderr P i'm a log message!`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "stream", Value: "stderr"}},
		},
		"expected structured metadata to be extracted with json parser and to be added to entry after rendering the template": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromJSONWithTemplate,
			logLine:                    `{"app":"loki" ,"component":"ingester"}`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "app", Value: "LOKI"}},
		},
		"expected structured metadata and regular labels to be extracted with json parser and to be added to entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataAndRegularLabelsFromJSON,
			logLine:                    `{"app":"loki" ,"component":"ingester"}`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "app", Value: "loki"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("ingester")},
		},
		"expected structured metadata and regular labels to be extracted with static labels stage and to be added to entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromStaticLabels,
			logLine:                    `sample log line`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "pod", Value: "loki-querier-664f97db8d-qhnwg"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		"expected structured metadata and regular labels to be extracted with static labels stage using different structured key": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromStaticLabelsDifferentKey,
			logLine:                    `sample log line`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		"expected structured metadata and regular labels to be extracted using regex with static labels stage": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromRegexLabels,
			logLine:                    `sample log line`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "label_app_kubernetes_io_name", Value: "loki"}, push.LabelAdapter{Name: "label_app_kubernetes_io_component", Value: "querier"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		"expected structured metadata and regular labels to be extracted using regex and non-regex with static labels stage": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromRegexAndNonRegexLabels,
			logLine:                    `sample log line`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"}, push.LabelAdapter{Name: "label_app_kubernetes_io_name", Value: "loki"}, push.LabelAdapter{Name: "label_app_kubernetes_io_component", Value: "querier"}},
			expectedLabels:             model.LabelSet{model.LabelName("component"): model.LabelValue("querier")},
		},
		"expected structured metadata to be set from extracted values": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromExtractedValues,
			logLine:                    `pod=loki-querier-664f97db8d-qhnwg metadata_name=loki metadata_component=querier msg="sample log line"`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "pod_name", Value: "loki-querier-664f97db8d-qhnwg"}, push.LabelAdapter{Name: "metadata_name", Value: "loki"}, push.LabelAdapter{Name: "metadata_component", Value: "querier"}},
		},
		"expected structured metadata from nested values": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataFromNestedValues,
			logLine:                    `{"app":"loki", "component_nested": {"name":"ingester", "props":{"n1": "v1", "n2": "v2"}}, "component_non_nested": "non_nested_val"}`,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "component_nested", Value: `{"name":"ingester","props":{"n1":"v1","n2":"v2"}}`}, push.LabelAdapter{Name: "component_non_nested", Value: "non_nested_val"}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(test.pipelineStagesYaml), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

			result := processEntries(pl, newEntry(nil, nil, test.logLine, time.Now()))[0]
			expectedStructuredMetadata := make(map[string]string)
			for _, l := range test.expectedStructuredMetadata {
				expectedStructuredMetadata[l.Name] = l.Value
			}
			resultStructuredMetadata := make(map[string]string)
			for _, l := range result.StructuredMetadata {
				resultStructuredMetadata[l.Name] = l.Value
			}
			require.Equal(t, expectedStructuredMetadata, resultStructuredMetadata)
			if test.expectedLabels != nil {
				require.Equal(t, test.expectedLabels, result.Labels)
			} else {
				require.Empty(t, result.Labels)
			}
		})
	}
}
