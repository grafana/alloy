package otelcol_test

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/grafana/alloy/internal/component/otelcol"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestConvertMatchProperties(t *testing.T) {
	inputMatchProps := otelcol.MatchProperties{
		MatchType: "strict",
		RegexpConfig: &otelcol.RegexpConfig{
			CacheEnabled:       false,
			CacheMaxNumEntries: 2,
		},
		Services:         []string{"svcA", "svcB"},
		SpanNames:        []string{"login.*"},
		LogBodies:        []string{"AUTH.*"},
		LogSeverityTexts: []string{"debug.*"},
		LogSeverity: &otelcol.LogSeverityNumberMatchProperties{
			Min:            "TRACE2",
			MatchUndefined: true,
		},
		MetricNames: []string{"metric1"},
		Attributes: []otelcol.Attribute{
			{
				Key:   "attr1",
				Value: 5,
			},
			{
				Key:   "attr2",
				Value: "asdf",
			},
			{
				Key:   "attr2",
				Value: false,
			},
		},
		Resources: []otelcol.Attribute{
			{
				Key:   "attr1",
				Value: 5,
			},
		},
		Libraries: []otelcol.InstrumentationLibrary{
			{
				Name:    "mongo-java-driver",
				Version: ptr.To("3.8.0"),
			},
		},
		SpanKinds: []string{"span1"},
	}

	expectedMatchProps := map[string]any{
		"attributes": []any{
			map[string]any{
				"key":   "attr1",
				"value": 5,
			},
			map[string]any{
				"key":   "attr2",
				"value": "asdf",
			},
			map[string]any{
				"key":   "attr2",
				"value": false,
			},
		},
		"libraries": []any{
			map[string]any{
				"name":    "mongo-java-driver",
				"version": "3.8.0",
			},
		},
		"log_bodies": []string{"AUTH.*"},
		"log_severity_number": map[string]any{
			"min":             plog.SeverityNumber(2),
			"match_undefined": true,
		},
		"log_severity_texts": []string{
			"debug.*",
		},
		"match_type":   "strict",
		"metric_names": []string{"metric1"},
		"regexp": map[string]any{
			"cacheenabled":       false,
			"cachemaxnumentries": 2,
		},
		"resources": []any{
			map[string]any{
				"key":   "attr1",
				"value": 5,
			},
		},
		"services":   []string{"svcA", "svcB"},
		"span_kinds": []string{"span1"},
		"span_names": []string{"login.*"},
	}

	tests := []struct {
		testName            string
		inputMatchConfig    otelcol.MatchConfig
		expectedMatchConfig map[string]any
	}{
		{
			testName:            "TestConvertEmpty",
			inputMatchConfig:    otelcol.MatchConfig{},
			expectedMatchConfig: make(map[string]any),
		},
		{
			testName: "TestConvertMandatory",
			inputMatchConfig: otelcol.MatchConfig{
				Include: &otelcol.MatchProperties{
					MatchType: "strict",
				},
			},
			expectedMatchConfig: map[string]any{
				"include": map[string]any{
					"match_type": "strict",
				},
			},
		},
		{
			testName: "TestAllOptsInclExcl",
			inputMatchConfig: otelcol.MatchConfig{
				Include: &inputMatchProps,
				Exclude: &inputMatchProps,
			},
			expectedMatchConfig: map[string]any{
				"include": expectedMatchProps,
				"exclude": expectedMatchProps,
			},
		},
		{
			testName: "TestAllOptsIncl",
			inputMatchConfig: otelcol.MatchConfig{
				Include: &inputMatchProps,
			},
			expectedMatchConfig: map[string]any{
				"include": expectedMatchProps,
			},
		},
		{
			testName: "TestAllOptsExcl",
			inputMatchConfig: otelcol.MatchConfig{
				Exclude: &inputMatchProps,
			},
			expectedMatchConfig: map[string]any{
				"exclude": expectedMatchProps,
			},
		},
	}

	for _, tt := range tests {
		if matchConf := tt.inputMatchConfig.Exclude; matchConf != nil {
			result, err := matchConf.Convert()
			require.NoError(t, err)
			require.Equal(t, tt.expectedMatchConfig["exclude"], result)
		} else {
			require.Empty(t, tt.expectedMatchConfig["exclude"])
		}

		if matchConf := tt.inputMatchConfig.Include; matchConf != nil {
			result, err := matchConf.Convert()
			require.NoError(t, err)
			require.Equal(t, tt.expectedMatchConfig["include"], result)
		} else {
			require.Empty(t, tt.expectedMatchConfig["include"])
		}
	}
}

func TestUnmarshalSeverityLevel(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid TRACE config",
			cfg: `
				min = "TRACE"
				match_undefined = true
			`,
		},
		{
			name: "valid DEBUG config",
			cfg: `
				min = "DEBUG"
				match_undefined = true
			`,
		},
		{
			name: "valid INFO config",
			cfg: `
				min = "INFO"
				match_undefined = true
			`,
		},
		{
			name: "valid INFO config without matching undefined",
			cfg: `
				min = "INFO"
				match_undefined = false
			`,
		},
		{
			name: "valid WARN config",
			cfg: `
				min = "WARN"
				match_undefined = true
			`,
		},
		{
			name: "valid ERROR config",
			cfg: `
			min = "ERROR"
			match_undefined = true
		`,
		},
		{
			name: "valid FATAL config",
			cfg: `
			min = "FATAL"
			match_undefined = true
		`,
		},
		{
			name: "valid FATAL4 config",
			cfg: `
			min = "FATAL4"
			match_undefined = true
		`,
		},
		{
			name: "invalid lowercase sev level",
			cfg: `
				min = "trace"
				match_undefined = true
			`,
			expectErr: true,
		},
		{
			name: "non-existent sev level",
			cfg: `
				min = "foo"
				match_undefined = true
			`,
			expectErr: true,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl otelcol.LogSeverityNumberMatchProperties
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalSeverityLevelString(t *testing.T) {
	for _, sevLevelStr := range []string{
		"TRACE",
		"TRACE2",
		"TRACE3",
		"TRACE4",
		"DEBUG",
		"DEBUG2",
		"DEBUG3",
		"DEBUG4",
		"INFO",
		"INFO2",
		"INFO3",
		"INFO4",
		"WARN",
		"WARN2",
		"WARN3",
		"WARN4",
		"ERROR",
		"ERROR2",
		"ERROR3",
		"ERROR4",
		"FATAL",
		"FATAL2",
		"FATAL3",
		"FATAL4",
	} {
		sevLevelStr := sevLevelStr

		t.Run(sevLevelStr, func(t *testing.T) {
			t.Parallel()

			var sl otelcol.SeverityLevel
			require.NoError(t, sl.UnmarshalText([]byte(sevLevelStr)))
			require.Equal(t, sevLevelStr, string(sl))
		})
	}
}
