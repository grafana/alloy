package stages

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/loki/pkg/push"
	util_log "github.com/grafana/loki/v3/pkg/util/log"
)

var pipelineStagesStructuredMetadataDropOne = `
stage.static_labels {
	values = {"foo" = "bar"}
}

stage.structured_metadata {
	values = {"foo" = ""}
}

stage.structured_metadata_drop {
	values = ["foo"]
}
`
var pipelineStagesStructuredMetadataDropTwo = `
stage.static_labels {
	values = {
	  "foo" = "bar",
	  "bar" = "baz",
	}
}

stage.structured_metadata {
	values = {
	  "foo" = "",
	  "bar" = "",
	}
}

stage.structured_metadata_drop {
	values = ["foo", "bar"]
}
`

var pipelineStagesStructuredMetadataDropNonExisting = `
stage.static_labels {
	values = {
	  "foo" = "bar",
	}
}

stage.structured_metadata {
	values = {
	  "foo" = "",
	}
}

stage.structured_metadata_drop {
	values = ["baz"]
}
`

var pipelineStagesStructuredMetadataDropKeepOthers = `
stage.static_labels {
	values = {
	  "foo" = "bar",
	  "bar" = "baz",
	}
}
stage.structured_metadata {
	values = {
	  "foo" = "",
	  "bar" = "",
	}
}

stage.structured_metadata_drop {
	values = ["foo"]
}
`

func Test_StructuredMetadataDropStage(t *testing.T) {
	tests := map[string]struct {
		pipelineStagesYaml         string
		logLine                    string
		expectedStructuredMetadata push.LabelsAdapter
	}{
		"expected structured_metadata_drop to remove one entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataDropOne,
			expectedStructuredMetadata: push.LabelsAdapter{},
		},
		"expected structured_metadata_drop to remove two entries": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataDropTwo,
			expectedStructuredMetadata: push.LabelsAdapter{},
		},
		"expected structured_metadata_drop to remove non existing entry": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataDropNonExisting,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "foo", Value: "bar"}},
		},
		"expected structured_metadata_drop to keep other entries": {
			pipelineStagesYaml:         pipelineStagesStructuredMetadataDropKeepOthers,
			expectedStructuredMetadata: push.LabelsAdapter{push.LabelAdapter{Name: "bar", Value: "baz"}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pl, err := NewPipeline(util_log.Logger, loadConfig(test.pipelineStagesYaml), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)

			result := processEntries(pl, newEntry(nil, nil, "", time.Now()))[0]
			require.Equal(t, test.expectedStructuredMetadata, result.StructuredMetadata)
		})
	}
}

func Test_StructuredMetadataDropStage_Validation(t *testing.T) {
	stage, err := newStructuredMetadataDropStage(util_log.Logger, StructuredMetadataDropConfig{Values: []string{}})
	assert.EqualError(t, err, ErrEmptyStructuredMetadataDropStageConfig.Error())
	assert.Nil(t, stage)
}
