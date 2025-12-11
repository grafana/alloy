package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/loki/pkg/push"
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
			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(test.pipelineStagesYaml), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable, labelstore.New(nil, prometheus.DefaultRegisterer))
			require.NoError(t, err)

			result := processEntries(pl, newEntry(nil, nil, "", time.Now()))[0]
			require.Equal(t, test.expectedStructuredMetadata, result.StructuredMetadata)
		})
	}
}

func Test_StructuredMetadataDropStage_Validation(t *testing.T) {
	stage, err := newStructuredMetadataDropStage(log.NewNopLogger(), StructuredMetadataDropConfig{Values: []string{}})
	assert.EqualError(t, err, ErrEmptyStructuredMetadataDropStageConfig.Error())
	assert.Nil(t, stage)
}
