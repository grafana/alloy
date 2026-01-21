package stages

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/featuregate"
)

var testTenantAlloyExtractedData = `
stage.json {
		expressions = { "customer_id" = "" }
}
stage.tenant { 
		source = "customer_id"
} `

var testTenantLogLineWithMissingKey = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN"
}
`

func TestPipelineWithMissingKey_Tenant(t *testing.T) {
	var buf bytes.Buffer
	w := log.NewSyncWriter(&buf)
	logger := log.NewLogfmtLogger(w)
	pl, err := NewPipeline(logger, loadConfig(testTenantAlloyExtractedData), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	Debug = true

	_ = processEntries(pl, newEntry(nil, nil, testTenantLogLineWithMissingKey, time.Now()))
	expectedLog := "level=debug msg=\"failed to convert value to string\" err=\"can't convert <nil> to string\" type=null"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

func TestTenantStage_Validation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config      TenantConfig
		expectedErr error
	}{
		"should pass on source config option set": {
			config: TenantConfig{
				Source: "tenant",
			},
			expectedErr: nil,
		},
		"should pass on value config option set": {
			config: TenantConfig{
				Value: "team-a",
			},
			expectedErr: nil,
		},
		"should fail on missing source and value": {
			config:      TenantConfig{},
			expectedErr: ErrTenantStageEmptyLabelSourceOrValue,
		},
		"should fail on empty source": {
			config: TenantConfig{
				Source: "",
			},
			expectedErr: ErrTenantStageEmptyLabelSourceOrValue,
		},
		"should fail on empty value": {
			config: TenantConfig{
				Value: "",
			},
			expectedErr: ErrTenantStageEmptyLabelSourceOrValue,
		},
		"should fail on empty label": {
			config: TenantConfig{
				Label: "",
			},
			expectedErr: ErrTenantStageEmptyLabelSourceOrValue,
		},
		"should fail on both source and value set": {
			config: TenantConfig{
				Source: "tenant",
				Value:  "team-a",
			},
			expectedErr: ErrTenantStageConflictingLabelSourceAndValue,
		},
		"should fail on both source and label set": {
			config: TenantConfig{
				Source: "tenant",
				Label:  "team-a",
			},
			expectedErr: ErrTenantStageConflictingLabelSourceAndValue,
		},
		"should fail on both label and value set": {
			config: TenantConfig{
				Label: "tenant",
				Value: "team-a",
			},
			expectedErr: ErrTenantStageConflictingLabelSourceAndValue,
		},
		"should fail on all set": {
			config: TenantConfig{
				Label:  "tenant",
				Source: "tenant",
				Value:  "team-a",
			},
			expectedErr: ErrTenantStageConflictingLabelSourceAndValue,
		},
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			stage, err := newTenantStage(log.NewNopLogger(), testData.config)

			if testData.expectedErr != nil {
				assert.EqualError(t, err, testData.expectedErr.Error())
				assert.Nil(t, stage)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, stage)
			}
		})
	}
}

func TestTenantStage_Process(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config         TenantConfig
		inputLabels    model.LabelSet
		inputExtracted map[string]any
		expectedTenant *string
	}{
		"should not set the tenant if the source field is not defined in the extracted map": {
			config:         TenantConfig{Source: "tenant_id"},
			inputLabels:    model.LabelSet{},
			inputExtracted: map[string]any{},
			expectedTenant: nil,
		},
		"should not override the tenant if the source field is not defined in the extracted map": {
			config:         TenantConfig{Source: "tenant_id"},
			inputLabels:    model.LabelSet{client.ReservedLabelTenantID: "foo"},
			inputExtracted: map[string]any{},
			expectedTenant: ptr("foo"),
		},
		"should set the tenant if the source field is defined in the extracted map": {
			config:         TenantConfig{Source: "tenant_id"},
			inputLabels:    model.LabelSet{},
			inputExtracted: map[string]any{"tenant_id": "bar"},
			expectedTenant: ptr("bar"),
		},
		"should set the tenant if the label is defined in the label map": {
			config:         TenantConfig{Label: "tenant_id"},
			inputLabels:    model.LabelSet{"tenant_id": "bar"},
			inputExtracted: map[string]any{},
			expectedTenant: ptr("bar"),
		},
		"should override the tenant if the source field is defined in the extracted map": {
			config:         TenantConfig{Source: "tenant_id"},
			inputLabels:    model.LabelSet{client.ReservedLabelTenantID: "foo"},
			inputExtracted: map[string]any{"tenant_id": "bar"},
			expectedTenant: ptr("bar"),
		},
		"should not set the tenant if the source field data type can't be converted to string": {
			config:         TenantConfig{Source: "tenant_id"},
			inputLabels:    model.LabelSet{},
			inputExtracted: map[string]any{"tenant_id": []string{"bar"}},
			expectedTenant: nil,
		},
		"should set the tenant with the configured static value": {
			config:         TenantConfig{Value: "bar"},
			inputLabels:    model.LabelSet{},
			inputExtracted: map[string]any{},
			expectedTenant: ptr("bar"),
		},
		"should override the tenant with the configured static value": {
			config:         TenantConfig{Value: "bar"},
			inputLabels:    model.LabelSet{client.ReservedLabelTenantID: "foo"},
			inputExtracted: map[string]any{},
			expectedTenant: ptr("bar"),
		},
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			stage, err := newTenantStage(log.NewNopLogger(), testData.config)
			require.NoError(t, err)

			// Process and dummy line and ensure nothing has changed except
			// the tenant reserved label

			out := processEntries(stage, newEntry(testData.inputExtracted, testData.inputLabels.Clone(), "hello world", time.Unix(1, 1)))[0]

			assert.Equal(t, time.Unix(1, 1), out.Timestamp)
			assert.Equal(t, "hello world", out.Line)

			actualTenant, ok := out.Labels[ReservedLabelTenantID]
			if testData.expectedTenant == nil {
				assert.False(t, ok)
			} else {
				assert.Equal(t, *testData.expectedTenant, string(actualTenant))
			}
		})
	}
}
