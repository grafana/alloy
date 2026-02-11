package stages

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/featuregate"
)

var testLabelsYaml = ` stage.json {
                           expressions = { level = "", app_rename = "app" }
                       }
                       stage.labels { 
                           values = {"level" = "", "app" = "app_rename" }
                       }`

var testLabelsLogLine = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN"
}
`
var testLabelsLogLineWithMissingKey = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"]
}
`

func TestLabelsPipeline_Labels(t *testing.T) {
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testLabelsYaml), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	expectedLbls := model.LabelSet{
		"level": "WARN",
		"app":   "loki",
	}

	out := processEntries(pl, newEntry(nil, nil, testLabelsLogLine, time.Now()))[0]
	assert.Equal(t, expectedLbls, out.Labels)
}

func TestLabelsPipelineWithMissingKey_Labels(t *testing.T) {
	var buf bytes.Buffer
	w := log.NewSyncWriter(&buf)
	logger := log.NewLogfmtLogger(w)
	pl, err := NewPipeline(logger, loadConfig(testLabelsYaml), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	Debug = true

	_ = processEntries(pl, newEntry(nil, nil, testLabelsLogLineWithMissingKey, time.Now()))

	expectedLog := "level=debug msg=\"failed to convert extracted label value to string\" err=\"can't convert <nil> to string\" type=null"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

var (
	lv1 = "lv1"
	lv3 = ""
)

var emptyLabelsConfig = LabelsConfig{nil}

func TestLabels(t *testing.T) {
	tests := map[string]struct {
		config       LabelsConfig
		err          error
		expectedCfgs map[string]string
	}{
		"missing config": {
			config:       emptyLabelsConfig,
			err:          errors.New(ErrEmptyLabelStageConfig),
			expectedCfgs: nil,
		},
		"invalid label name": {
			config: LabelsConfig{
				Values: map[string]*string{"\xfd": nil},
			},
			err:          fmt.Errorf(ErrInvalidLabelName, "\xfd"),
			expectedCfgs: nil,
		},
		"label value is set from name": {
			config: LabelsConfig{Values: map[string]*string{
				"l1": &lv1,
				"l2": nil,
				"l3": &lv3,
			}},
			err: nil,
			expectedCfgs: map[string]string{
				"l1": lv1,
				"l2": "l2",
				"l3": "l3",
			},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			actual, err := validateLabelsConfig(test.config.Values)
			if (err != nil) != (test.err != nil) {
				t.Errorf("validateLabelsConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if (err != nil) && (err.Error() != test.err.Error()) {
				t.Errorf("validateLabelsConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if test.expectedCfgs != nil {
				assert.Equal(t, test.expectedCfgs, actual)
			}
		})
	}
}

func TestLabelStage_Process(t *testing.T) {
	sourceName := "diff_source"
	tests := map[string]struct {
		config         LabelsConfig
		extractedData  map[string]any
		inputLabels    model.LabelSet
		expectedLabels model.LabelSet
	}{
		"extract_success": {
			LabelsConfig{Values: map[string]*string{
				"testLabel": nil,
			}},
			map[string]any{
				"testLabel": "testValue",
			},
			model.LabelSet{},
			model.LabelSet{
				"testLabel": "testValue",
			},
		},
		"different_source_name": {
			LabelsConfig{Values: map[string]*string{
				"testLabel": &sourceName,
			}},
			map[string]any{
				sourceName: "testValue",
			},
			model.LabelSet{},
			model.LabelSet{
				"testLabel": "testValue",
			},
		},
		"empty_extracted_data": {
			LabelsConfig{Values: map[string]*string{
				"testLabel": &sourceName,
			}},
			map[string]any{},
			model.LabelSet{},
			model.LabelSet{},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			st, err := newLabelStage(log.NewNopLogger(), test.config)
			if err != nil {
				t.Fatal(err)
			}

			out := processEntries(st, newEntry(test.extractedData, test.inputLabels, "", time.Time{}))[0]
			assert.Equal(t, test.expectedLabels, out.Labels)
		})
	}
}
