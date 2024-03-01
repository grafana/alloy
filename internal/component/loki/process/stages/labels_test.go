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

	util_log "github.com/grafana/loki/pkg/util/log"
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
	pl, err := NewPipeline(util_log.Logger, loadConfig(testLabelsYaml), nil, prometheus.DefaultRegisterer)
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
	pl, err := NewPipeline(logger, loadConfig(testLabelsYaml), nil, prometheus.DefaultRegisterer)
	if err != nil {
		t.Fatal(err)
	}
	Debug = true

	_ = processEntries(pl, newEntry(nil, nil, testLabelsLogLineWithMissingKey, time.Now()))

	expectedLog := "level=debug msg=\"failed to convert extracted label value to string\" err=\"Can't convert <nil> to string\" type=null"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

var (
	lv1  = "lv1"
	lv2c = "l2"
	lv3  = ""
	lv3c = "l3"
)

var emptyLabelsConfig = LabelsConfig{nil}

func TestLabels(t *testing.T) {
	tests := map[string]struct {
		config       LabelsConfig
		err          error
		expectedCfgs LabelsConfig
	}{
		"missing config": {
			config:       emptyLabelsConfig,
			err:          errors.New(ErrEmptyLabelStageConfig),
			expectedCfgs: emptyLabelsConfig,
		},
		"invalid label name": {
			config: LabelsConfig{
				Values: map[string]*string{"#*FDDS*": nil},
			},
			err:          fmt.Errorf(ErrInvalidLabelName, "#*FDDS*"),
			expectedCfgs: emptyLabelsConfig,
		},
		"label value is set from name": {
			config: LabelsConfig{Values: map[string]*string{
				"l1": &lv1,
				"l2": nil,
				"l3": &lv3,
			}},
			err: nil,
			expectedCfgs: LabelsConfig{Values: map[string]*string{
				"l1": &lv1,
				"l2": &lv2c,
				"l3": &lv3c,
			}},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateLabelsConfig(test.config)
			if (err != nil) != (test.err != nil) {
				t.Errorf("validateLabelsConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if (err != nil) && (err.Error() != test.err.Error()) {
				t.Errorf("validateLabelsConfig() expected error = %v, actual error = %v", test.err, err)
				return
			}
			if test.expectedCfgs.Values != nil {
				assert.Equal(t, test.expectedCfgs, test.config)
			}
		})
	}
}

func TestLabelStage_Process(t *testing.T) {
	sourceName := "diff_source"
	tests := map[string]struct {
		config         LabelsConfig
		extractedData  map[string]interface{}
		inputLabels    model.LabelSet
		expectedLabels model.LabelSet
	}{
		"extract_success": {
			LabelsConfig{Values: map[string]*string{
				"testLabel": nil,
			}},
			map[string]interface{}{
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
			map[string]interface{}{
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
			map[string]interface{}{},
			model.LabelSet{},
			model.LabelSet{},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			st, err := newLabelStage(util_log.Logger, test.config)
			if err != nil {
				t.Fatal(err)
			}

			out := processEntries(st, newEntry(test.extractedData, test.inputLabels, "", time.Time{}))[0]
			assert.Equal(t, test.expectedLabels, out.Labels)
		})
	}
}
