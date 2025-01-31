package stages

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/loki/pkg/push"
	util_log "github.com/grafana/loki/v3/pkg/util/log"
)

var pipelineStagesStructuredMetadataRegexFromStaticLabels = `
stage.static_labels {
	values = {"component" = "querier", "pod" = "loki-querier-664f97db8d-qhnwg"}
}
stage.structured_metadata_regex {
        regex = "comp.*"
}
`

func Test_structuredMetadataRegexStage(t *testing.T) {
	tests := map[string]struct {
		pipelineStagesYaml         string
		logLine                    string
		expectedStructuredMetadata push.LabelsAdapter
		expectedLabels             model.LabelSet
	}{
		"expected ": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataRegexFromStaticLabels,
			logLine:                    "",
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "component", Value: "querier"}},
			expectedLabels:             model.LabelSet{model.LabelName("pod"): model.LabelValue("loki-querier-664f97db8d-qhnwg")},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pl, err := NewPipeline(util_log.Logger, loadConfig(test.pipelineStagesYaml), nil, prometheus.DefaultRegisterer)
			require.NoError(t, err)

			result := processEntries(pl, newEntry(nil, nil, test.logLine, time.Now()))[0]
			require.Equal(t, test.expectedStructuredMetadata, result.StructuredMetadata)
			if test.expectedLabels != nil {
				require.Equal(t, test.expectedLabels, result.Labels)
			} else {
				require.Empty(t, result.Labels)
			}
		})
	}
}
